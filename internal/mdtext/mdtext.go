// Package mdtext reduces a Markdown document to the prose in it, and splits
// that prose into the units a writing rule is about.
//
// Code fences, tables, link definitions and raw HTML are removed throughout:
// they are not prose, and none of the writing rules are about them. What is
// left is what a reader reads.
package mdtext

import (
	"regexp"
	"strings"

	"github.com/codesweep-ai/lint/internal/lint"
)

var (
	fence     = regexp.MustCompile("(?s)```.*?```")
	htmlNote  = regexp.MustCompile(`(?s)<!--.*?-->`)
	table     = regexp.MustCompile(`(?m)^\s*\|.*$`)
	linkDef   = regexp.MustCompile(`(?m)^\s*\[[^\]]+\]:.*$`)
	blockQuot = regexp.MustCompile(`(?m)^\s*> ?`)
	inlineCod = regexp.MustCompile("`[^`]*`")
	heading   = regexp.MustCompile(`(?m)^#+ .*$`)

	// Raw HTML blocks. The tag list is explicit on purpose: a bare "starts
	// with <word>" pattern also eats a line beginning with a placeholder such
	// as <name>, and swallowing its closing backtick corrupts every code span
	// after it.
	rawHTML = regexp.MustCompile(`(?im)^\s*</?(?:p|div|img|a|sub|sup|i|b|em|strong|br|hr|span|table|tr|td|th` +
		`|details|summary|picture|source|video|h[1-6]|!--)\b[^>]*>.*$`)
)

// Prose returns the document with everything that is not prose removed.
//
// Every removal keeps the newlines it spanned, so an offset into the result
// still lands on the line it came from and a finding can carry a line number.
// Nothing else about the text is preserved: what is left is the prose, closed
// up.
func Prose(text string) string {
	text = blankOut(fence, text)
	// HTML comments, including the marker pairs a tool injects to own a block
	// of a file. The raw-HTML pattern below cannot catch these: its \b after
	// the tag name never matches the space in "<!-- MARKER -->".
	text = blankOut(htmlNote, text)
	text = table.ReplaceAllString(text, "")
	text = linkDef.ReplaceAllString(text, "")
	text = rawHTML.ReplaceAllString(text, "")
	text = blockQuot.ReplaceAllString(text, "") // the content of a quote is prose
	text = inlineCod.ReplaceAllString(text, "CODE")
	return text
}

// blankOut replaces every match with the newlines it spanned, so the text
// after it keeps its line number. The line-anchored patterns need no such care:
// they match within a line and leave its terminator alone.
func blankOut(re *regexp.Regexp, text string) string {
	return re.ReplaceAllStringFunc(text, func(m string) string {
		return strings.Repeat("\n", strings.Count(m, "\n"))
	})
}

// StripHeadings removes heading lines. A heading names a term without
// introducing it, so the first-use check reads the body without them.
func StripHeadings(text string) string { return heading.ReplaceAllString(text, "") }

// Splitter cuts prose into sentences. It carries the project's own lower-case
// sentence starters, which are nearly always its command name: without them
// "Nothing matches. cs-lint exits 1." reads as one six-word sentence and
// reports a length that is not real.
type Splitter struct {
	starters []string
	boundary *regexp.Regexp
}

// closers and openers sit on both sides of a boundary and belong in both
// classes. A sentence can end inside quotation marks, as `he called it "done."
// It shipped` does, and the next can open with one. Without them the splitter
// joins the two.
const (
	closers = "*`)\\]\"'”’"
	openers = "A-Z`*\\[\"'“‘"
)

// NewSplitter returns a splitter that also breaks before the words given.
func NewSplitter(lowercaseStarters []string) *Splitter {
	return &Splitter{
		starters: lowercaseStarters,
		// A terminator, then any closing markup, then whitespace. Whether
		// what follows opens a sentence is checked in Sentences, because Go's
		// regexp has no lookahead.
		boundary: regexp.MustCompile(`[.!?][` + closers + `]*\s+`),
	}
}

var openerRE = regexp.MustCompile(`^[` + openers + `]`)

// opensSentence reports whether the text at this point starts a new sentence:
// a capital, a markdown marker, or one of the project's own lower-case names.
func (s *Splitter) opensSentence(rest string) bool {
	if openerRE.MatchString(rest) {
		return true
	}
	for _, w := range s.starters {
		if strings.HasPrefix(rest, w) {
			after := rest[len(w):]
			// A word boundary, so "cs-lintish" does not start a sentence.
			if after == "" || !lint.IsWordByte(after[0]) {
				return true
			}
		}
	}
	return false
}

// Sentences splits a paragraph into sentences.
//
// This is not a full sentence splitter: abbreviations and version numbers
// would break one. Splitting on a terminator followed by something that opens
// a sentence is enough for a length check, and errs towards longer rather than
// shorter.
func (s *Splitter) Sentences(paragraph string) []string {
	var out []string
	prev := 0
	for _, m := range s.boundary.FindAllStringIndex(paragraph, -1) {
		start, end := m[0], m[1]
		if start < prev {
			continue
		}
		if !s.opensSentence(paragraph[end:]) {
			continue
		}
		// The terminator stays with the sentence it ends; the closing markup
		// and the whitespace after it belong to neither.
		if cut := strings.TrimSpace(paragraph[prev : start+1]); cut != "" {
			out = append(out, cut)
		}
		prev = end
	}
	if cut := strings.TrimSpace(paragraph[prev:]); cut != "" {
		out = append(out, cut)
	}
	return out
}

var listItem = regexp.MustCompile(`^\s*([-*+]|\d+\.)\s`)

// Units returns a paragraph's prose units: a list is one unit per item, not
// one blob. An em-dash budget counted over a whole list would report a
// readable "**Term** - meaning" table as a wall of asides.
func Units(paragraph string) []string {
	var out []string
	var buf []string
	inItem := false
	flush := func() {
		if len(buf) > 0 {
			out = append(out, strings.Join(buf, " "))
			buf = nil
		}
	}
	for line := range strings.SplitSeq(paragraph, "\n") {
		switch {
		case listItem.MatchString(line):
			flush()
			buf = []string{strings.TrimSpace(line)}
			inItem = true
		case inItem && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")):
			// An indented line continues the item above it.
			buf = append(buf, strings.TrimSpace(line))
		default:
			if inItem {
				flush()
				inItem = false
			}
			buf = append(buf, strings.TrimSpace(line))
		}
	}
	flush()
	kept := out[:0]
	for _, u := range out {
		if strings.TrimSpace(u) != "" {
			kept = append(kept, u)
		}
	}
	return kept
}

// Flatten collapses all whitespace in a string to single spaces.
func Flatten(s string) string { return strings.Join(strings.Fields(s), " ") }

// Quoted reports whether the offset sits inside quotation marks on its own
// line. Naming a word is not using it, so a rule about a word skips the place
// the document quotes it.
func Quoted(text string, pos int) bool {
	start := strings.LastIndexByte(text[:pos], '\n') + 1
	return strings.Count(text[start:pos], `"`)%2 == 1
}

// Context returns the text around an offset, flattened, as the quote that
// proves a finding.
func Context(text string, pos int) string {
	lo := max(pos-60, 0)
	hi := min(pos+90, len(text))
	return Flatten(text[lo:hi])
}
