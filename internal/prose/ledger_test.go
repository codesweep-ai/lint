package prose

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/lint"
)

// writeLedger puts one file in a scratch repository's ledger and returns the
// root and the path to check.
func writeLedger(t *testing.T, name, body string) (root, rel string) {
	t.Helper()
	root = t.TempDir()
	rel = filepath.Join(ledgerDir, name)
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, filepath.ToSlash(rel)
}

// checkLedger runs the linter over one ledger file.
func checkLedger(t *testing.T, cfg config.Prose, name, body string) []lint.Problem {
	t.Helper()
	root, rel := writeLedger(t, name, body)
	l, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	found, err := l.Check(root, rel)
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func ids(found []lint.Problem) []string {
	var out []string
	for _, p := range found {
		out = append(out, p.Rule)
	}
	return out
}

// A record holds its prose in fields, and the fields are what a writing rule
// is about. The rest of a record is dates, statuses and shas.
func TestRecordProseFieldsAreChecked(t *testing.T) {
	const rec = `{
  "id": "ABC-001",
  "title": "The check reads the fields that hold prose",
  "status": "closed",
  "foundBy": "rev-2 orchestrator",
  "stint": "2026-08-31 sweep",
  "opened": "2026-08-30",
  "evidence": {
    "commits": ["6b82039"],
    "verified": "Ran it twice and diffed the output — the second run matched."
  },
  "resolution": null,
  "details": "The page said one thing and the tool did another.",
  "notes": [
    {"date": "2026-08-31", "text": "A note that carries an em—dash of its own."}
  ]
}`
	found := checkLedger(t, config.Prose{}, "issues/ABC-001.json", rec)
	if n := len(found); n != 2 {
		t.Fatalf("want the two em-dashes, got %d: %v", n, found)
	}
	for _, p := range found {
		if p.Rule != "PROSE-104" {
			t.Errorf("unexpected rule %s: %v", p.Rule, p)
		}
	}
	// foundBy is an attribution, not a sentence. Reading it as prose would
	// report "rev-2 orchestrator" as an epigram with no verb.
	if slices.Contains(ids(found), "PROSE-102") {
		t.Error("an attribution was read as a sentence")
	}
}

// A finding names the line the field sits on, which is the line a writer opens
// the record at. A JSON string escapes its newlines, so however many
// paragraphs the field carries, that is one line.
func TestRecordFindingNamesTheFieldLine(t *testing.T) {
	const rec = `{
  "id": "ABC-002",
  "title": "A title",
  "details": "One paragraph.\n\nA second one holds an em—dash."
}`
	found := checkLedger(t, config.Prose{}, "issues/ABC-002.json", rec)
	if len(found) != 1 {
		t.Fatalf("want one finding, got %v", found)
	}
	if want := "ledger/issues/ABC-002.json:4"; found[0].Where != want {
		t.Errorf("reported at %q, want %q", found[0].Where, want)
	}
}

// Whether a record is valid is what cs-ledger check answers. Two tools
// reporting the same broken file disagree the moment one is a version behind.
func TestMalformedRecordReportsNothing(t *testing.T) {
	if found := checkLedger(t, config.Prose{}, "issues/ABC-003.json",
		`{"details": "an em—dash", `); len(found) != 0 {
		t.Errorf("a malformed record reported %v", found)
	}
}

// The page is checked for what a reader sees on it. What the scripts and the
// styles carry is not that.
func TestPageIsCheckedAsWhatAReaderSees(t *testing.T) {
	const html = `<!doctype html>
<html><head><title>the ledger</title>
<style>/* an em—dash in a comment */</style></head>
<body>
<p>Nothing here yet &mdash; file a record to see it.</p>
<script>var s = "an em—dash in a string";</script>
</body></html>`
	found := checkLedger(t, config.Prose{}, "ledger.html", html)
	if len(found) != 1 {
		t.Fatalf("want the one em-dash a reader sees, got %v", found)
	}
	if found[0].Rule != "PROSE-104" || !strings.HasSuffix(found[0].Where, "ledger.html:5") {
		t.Errorf("want PROSE-104 on line 5, got %s at %s", found[0].Rule, found[0].Where)
	}
}

// A control is not a sentence with its verb missing, and rewriting the filter
// bar into sentences would make the page worse.
func TestPageControlsAreNotEpigrams(t *testing.T) {
	const html = `<body>
<button>stale only</button><button>reset</button>
<span>status all</span>
</body>`
	if found := checkLedger(t, config.Prose{}, "ledger.html", html); len(found) != 0 {
		t.Errorf("the page's controls reported %v", found)
	}
	// The same words in a record are prose, and are read as prose.
	found := checkLedger(t, config.Prose{}, "issues/ABC-004.json",
		`{"details": "Stale only, status all."}`)
	if !slices.Contains(ids(found), "PROSE-102") {
		t.Errorf("a record's epigram went unreported: %v", found)
	}
}

// PROSE-101 asks a document to introduce a term. A record is read by somebody
// already inside the project, and each field is checked on its own, so the
// rule would ask every record to introduce the whole glossary again.
func TestGlossaryStandsDownOnTheLedger(t *testing.T) {
	cfg := config.Prose{Glossary: []string{"stint"}}
	if found := checkLedger(t, cfg, "issues/ABC-005.json",
		`{"details": "The stint closes tomorrow."}`); len(found) != 0 {
		t.Errorf("the glossary rule read a record: %v", found)
	}
	if found := checkLedger(t, cfg, "ledger.html",
		"<body><p>The stint closes tomorrow.</p></body>"); len(found) != 0 {
		t.Errorf("the glossary rule read a page: %v", found)
	}
	// The same sentence in a document is where the rule belongs.
	if got := check(t, cfg, "The stint closes tomorrow.\n"); !has(got, "PROSE-101") {
		t.Errorf("a document's first use of a term went unreported: %v", got)
	}
}

// The ledger's own files are checked, and the two Markdown documents cs-ledger
// renders into the tree are not: OSS-602 reports a repository that has edited
// either, so checking their style would ask for the one edit it forbids.
func TestLedgerFilesFound(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{
		"README.md",
		"ledger/ledger.json", "ledger/queue.json", "ledger/ledger.html",
		"ledger/issues/ABC-001.json", "ledger/drafts/ada/a-slug.json",
		"ledger/AGENTS.md", "ledger/GUIDE.md",
	} {
		full := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	l, err := New(config.Prose{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := l.Files(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"README.md", "ledger/drafts/ada/a-slug.json",
		"ledger/issues/ABC-001.json", "ledger/ledger.html", "ledger/ledger.json",
		"ledger/queue.json"}
	if !slices.Equal(got, want) {
		t.Errorf("checked %v, want %v", got, want)
	}

	// Naming the tree in skipExtra turns the whole family off.
	off, err := New(config.Prose{SkipExtra: []string{ledgerDir}})
	if err != nil {
		t.Fatal(err)
	}
	got, err = off.Files(root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"README.md"}) {
		t.Errorf("skipExtra left %v", got)
	}
}
