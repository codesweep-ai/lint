package prose

import (
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/codesweep-ai/lint/internal/lint"
)

// A repository that keeps a ledger writes prose into it every day. The records
// are JSON, and the page beside them is HTML, so a linter that reads only
// Markdown never opens either. The writing in them is this project's own, read
// by the people the documents are written for, and it goes out published: the
// page is what a human is pointed at.
//
// ledger/AGENTS.md and ledger/GUIDE.md are deliberately not here. cs-ledger
// renders both, and OSS-602 reports a repository whose copy is not the current
// render, so checking their style would ask for the one edit the readiness
// linter forbids. CODE_OF_CONDUCT.md sits in skipDefault for the same reason.

// ledgerDir is the tree cs-ledger owns, by the convention REF-303 and the
// OSS-6xx family already read.
const ledgerDir = "ledger"

// prosePaths are the fields that hold prose, by the path a walk of the
// document reaches them at. The union of three shapes is one map because a
// path names its file: only a record has details, only ledger.json has a
// description, only the queue has a why.
//
// Everything a record carries besides these is an identifier, a date, a
// status, a commit sha, or an attribution such as foundBy. A writing rule has
// nothing to say about any of them, and reading them as sentences would report
// "rev-2 orchestrator" as an epigram with no verb.
var prosePaths = map[string]bool{
	"title":             true, // the record in one line
	"details":           true, // the narrative writeup, in Markdown
	"resolution":        true, // why a record ended without a fix
	"evidence.verified": true, // how a closure was verified
	"notes[].text":      true, // the dated updates, appended over a record's life
	"description":       true, // ledger.json: what the project is
	"items[].why":       true, // queue.json: why this record is the next one
}

// ledgerFiles returns the ledger's own files that carry prose, in reporting
// order.
//
// Every JSON file under the tree, rather than the three names cs-ledger writes
// today: drafts arrive under a member's own directory, and a repository that
// grows a fourth kind of record should not have to wait for this list. A file
// that holds none of the prose fields yields nothing to check and costs one
// read.
//
// A repository with no ledger gets nothing, and naming the tree in skipExtra
// turns the whole family off.
func (l *Linter) ledgerFiles(root string, skip map[string]bool) []string {
	if skip[ledgerDir] {
		return nil
	}
	dir := filepath.Join(root, ledgerDir)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil
	}
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // an unreadable corner of the tree is not this rule's finding
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".json", ".html":
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// pieceKind is what the text on the other end of a piece is, which decides
// which rules have anything to say about it.
type pieceKind int

const (
	// document is a Markdown page: prose, read start to finish.
	document pieceKind = iota
	// record is one prose field of a ledger record.
	record
	// title is a record's own line, which the page renders as its heading.
	title
	// page is the text a reader sees on a rendered page.
	page
)

// piece is one run of prose, and where a finding in it is reported.
//
// A document is one piece and carries no anchor, so a finding takes its line
// from the offset, as it always has. A record field is anchored: JSON holds a
// string's newlines escaped, so however many paragraphs a field carries, all
// of it sits on one line of the file, and that line is what a writer opens.
type piece struct {
	rel    string
	anchor string
	text   string
	kind   pieceKind
}

// pieces returns the prose one file holds.
func (l *Linter) pieces(root, rel string) ([]piece, error) {
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return nil, err
	}
	raw := string(b)
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".json":
		return recordPieces(rel, raw), nil
	case ".html":
		return []piece{{rel: rel, text: visibleText(raw), kind: page}}, nil
	}
	return []piece{{rel: rel, text: raw}}, nil
}

// recordPieces returns one piece per prose field a record holds.
func recordPieces(rel, raw string) []piece {
	var out []piece
	if err := walkStrings(raw, func(path, value string, off int) {
		if !prosePaths[path] || strings.TrimSpace(value) == "" {
			return
		}
		kind := record
		if path == "title" {
			kind = title
		}
		out = append(out, piece{
			rel:    rel,
			anchor: fmt.Sprintf("%s:%d", rel, lint.Line(raw, off)),
			text:   value,
			kind:   kind,
		})
	}); err != nil {
		// Whether a record is valid is the question `cs-ledger check` answers,
		// and it answers it against the schema the records were written to.
		// Reporting half a file here would have two tools speaking about the
		// same break, and disagreeing the moment one is a version behind.
		return nil
	}
	return out
}

// walkStrings visits every string in a JSON document, with the path it sits at
// and the offset it starts at.
//
// The document is read by its shape rather than into a struct of this tool's
// own. The ledger's schema allows fields this linter has never heard of, and a
// walk that names only what it wants to read keeps working when one arrives.
// A malformed file stops the walk, and its caller drops what the walk had
// reached.
func walkStrings(raw string, visit func(path, value string, off int)) error {
	dec := json.NewDecoder(strings.NewReader(raw))
	return walkValue(dec, "", visit)
}

func walkValue(dec *json.Decoder, path string, visit func(string, string, int)) error {
	// Taken before the token is read, so a value lands just after the key that
	// names it, on the line a reader would look for it on.
	off := dec.InputOffset()
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			for dec.More() {
				key, keyErr := dec.Token()
				if keyErr != nil {
					return keyErr
				}
				name, _ := key.(string)
				if err := walkValue(dec, join(path, name), visit); err != nil {
					return err
				}
			}
		case '[':
			for dec.More() {
				// The index is left out of the path. A rule is about the
				// shape of a field, and notes[0].text is the same field as
				// notes[9].text.
				if err := walkValue(dec, path+"[]", visit); err != nil {
					return err
				}
			}
		default:
			return nil // a closing delimiter, read by the loop that opened it
		}
		_, err := dec.Token() // the closing delimiter
		return err
	case string:
		visit(path, t, int(off))
	}
	return nil
}

func join(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

var (
	htmlHidden  = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(?:script|style)>|<!--.*?-->`)
	htmlTag     = regexp.MustCompile(`(?s)<[^>]*>`)
	blankBefore = regexp.MustCompile(`[ \t]+\n`)
)

// visibleText is what a reader sees on a rendered page.
//
// The scripts, the styles and the comments go first, then the tags. Every
// removal keeps the newlines it spanned, so a finding still carries the line
// it came from, which is the care mdtext.Prose takes for the same reason.
//
// A page that renders itself from data it carries leaves none of that data
// here: the ledger's records sit in a script block, and this drops the block.
// That is deliberate. The records are checked where they are written and where
// a fix belongs, so reading them out of the page again would report every
// finding twice, the second time against a generated file.
func visibleText(raw string) string {
	out := blankSpan(htmlHidden, raw)
	out = blankSpan(htmlTag, out)
	out = html.UnescapeString(out)
	return blankBefore.ReplaceAllString(out, "\n")
}

// blankSpan removes what a pattern matches, keeping the newlines it spanned so
// the text after it stays on its own line. A match within one line leaves a
// space behind it, because the words either side of a tag are two words.
func blankSpan(re *regexp.Regexp, text string) string {
	return re.ReplaceAllStringFunc(text, func(m string) string {
		if n := strings.Count(m, "\n"); n > 0 {
			return strings.Repeat("\n", n)
		}
		return " "
	})
}
