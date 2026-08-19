package mdtext

import (
	"strings"
	"testing"

	"github.com/codesweep-ai/lint/internal/lint"
)

func TestProseKeepsLineNumbers(t *testing.T) {
	// Every removal has to keep the newlines it spanned, or a finding after a
	// code fence carries a line number from before it.
	raw := "one\n\n```go\nfunc main() {}\n```\n\ntarget\n"
	got := Prose(raw)
	if strings.Count(got, "\n") != strings.Count(raw, "\n") {
		t.Fatalf("newline count changed: raw %d, prose %d",
			strings.Count(raw, "\n"), strings.Count(got, "\n"))
	}
	if strings.Contains(got, "func main") {
		t.Errorf("the fence survived: %q", got)
	}
	if want := 7; lint.Line(got, strings.Index(got, "target")) != want {
		t.Errorf("target is on line %d, want %d", lint.Line(got, strings.Index(got, "target")), want)
	}
}

func TestProseRemovesWhatIsNotProse(t *testing.T) {
	for _, tc := range []struct {
		name, raw, gone string
	}{
		{"table", "| a | b |\n", "|"},
		{"link definition", "[ref]: https://example.com\n", "https://"},
		{"html comment", "<!-- MARKER -->\n", "MARKER"},
		{"inline code", "run `go test` now\n", "go test"},
		{"raw html", "<img src=\"x.png\">\n", "img"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Prose(tc.raw); strings.Contains(got, tc.gone) {
				t.Errorf("Prose(%q) = %q, still carries %q", tc.raw, got, tc.gone)
			}
		})
	}
}

func TestBlockquoteContentIsProse(t *testing.T) {
	if got := Prose("> the quoted claim\n"); !strings.Contains(got, "the quoted claim") {
		t.Errorf("Prose dropped a blockquote's content: %q", got)
	}
}

func TestSentencesSplitsOnTerminators(t *testing.T) {
	s := NewSplitter(nil)
	got := s.Sentences("One thing. Another thing. A third.")
	if len(got) != 3 {
		t.Fatalf("got %d sentences, want 3: %q", len(got), got)
	}
	if got[0] != "One thing." {
		t.Errorf("first sentence is %q, want %q", got[0], "One thing.")
	}
}

func TestSentencesKeepsLowercaseStarters(t *testing.T) {
	// Without the starter the splitter joins the two and reports a length that
	// is not real.
	raw := "Nothing matches. cs-lint exits 1."
	if got := NewSplitter(nil).Sentences(raw); len(got) != 1 {
		t.Fatalf("without the starter: got %d, want 1: %q", len(got), got)
	}
	got := NewSplitter([]string{"cs-lint"}).Sentences(raw)
	if len(got) != 2 {
		t.Fatalf("with the starter: got %d, want 2: %q", len(got), got)
	}
}

func TestSentencesSplitsAcrossQuotationMarks(t *testing.T) {
	// A sentence can end inside quotation marks and the next can open with one.
	got := NewSplitter(nil).Sentences(`He called it "done." It shipped.`)
	if len(got) != 2 {
		t.Fatalf("got %d sentences, want 2: %q", len(got), got)
	}
}

func TestUnitsSplitsAList(t *testing.T) {
	// A list is one unit per item, not one blob: an em-dash budget counted over
	// a whole list would report a readable table as a wall of asides.
	got := Units("- one — first\n- two — second\n")
	if len(got) != 2 {
		t.Fatalf("got %d units, want 2: %q", len(got), got)
	}
}

func TestUnitsJoinsAnIndentedContinuation(t *testing.T) {
	got := Units("- one\n  continued here\n")
	if len(got) != 1 {
		t.Fatalf("got %d units, want 1: %q", len(got), got)
	}
	if !strings.Contains(got[0], "continued here") {
		t.Errorf("the continuation was dropped: %q", got[0])
	}
}

func TestQuotedFindsAMention(t *testing.T) {
	text := `the word "simply" is named here`
	pos := strings.Index(text, "simply")
	if !Quoted(text, pos) {
		t.Error("a word inside quotation marks reads as used rather than named")
	}
	plain := "simply do it"
	if Quoted(plain, 0) {
		t.Error("an unquoted word reads as named rather than used")
	}
}
