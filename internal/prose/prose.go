// Package prose checks the prose in a repository's Markdown against the writing
// rules.
//
// Every check here is mechanical and quotable. Anything that needs judgement is
// left to review, because a linter that guesses produces noise, and noise gets
// ignored.
//
// Rules that follow published guidance say which guide, so a writer arguing
// with one knows whether it is this house's preference or an industry
// convention. The rest came out of repeated review of what confuses readers of
// these documents.
package prose

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/lint"
	"github.com/codesweep-ai/lint/internal/mdtext"
)

// The measurements the rules are written to.
const (
	// None. An em-dash is nearly always a sentence that would not commit: the
	// aside it introduces is either a full stop, a comma, or a cut. It is also
	// the punctuation a model reaches for first, so a document full of them
	// reads as unedited whoever wrote it.
	maxEmDashesPerParagraph = 0
	// Long enough for a qualified statement, short enough to hold one idea.
	maxSentenceWords = 30
	// An epigram is short by nature: "Two version numbers, one verdict, one
	// remedy." Above this length a sentence that trips the verb check is
	// almost always a false positive with a verb the list does not carry, so
	// the check gives up rather than cry wolf.
	maxEpigramWords = 12
	// Three occurrences rather than two, and deliberately: repeating a word
	// twice is often the clearest thing to do, and a check that argued about
	// it would be noise.
	maxEchoes = 3
)

// Rule describes one prose rule, for the explain verb.
type Rule struct {
	ID     string
	Title  string
	Why    string
	Source Source
}

// Rules is every rule the prose linter carries, in reporting order.
var Rules = []Rule{
	{"PROSE-101", "A glossary term is introduced where a document first uses it",
		"A reader should never meet a word the docs have not explained. " +
			"Introducing it means glossing it in the same sentence, defining it in a " +
			"glossary table, or linking to the page that defines it.", Google},
	{"PROSE-102", "Every sentence has a subject and a verb",
		"\"Two version numbers, one verdict, one remedy\" reads as knowing rather " +
			"than clear. Say what the thing is.", House},
	{"PROSE-103", "A sentence carries one idea and stays under thirty words",
		"Past thirty words a sentence is holding more than one idea, and the reader " +
			"has to take it apart before they can act on it.", RedHat},
	{"PROSE-104", "No em-dash",
		"The aside an em-dash introduces is a full stop, a comma, or a cut. It is also " +
			"the first punctuation a model reaches for, so a page full of them reads as " +
			"unedited whoever wrote it.", Both},
	{"PROSE-105", "A command runs only a script the document showed",
		"A reader should never meet a file they were not given. If a step invokes a " +
			"script, show the script first.", House},
	{"PROSE-106", "The writing does not comment on itself",
		"\"It is worth stating plainly\", \"put simply\", \"the point is\": delete the " +
			"frame and keep the sentence.", RedHat},
	{"PROSE-107", "A sentence does not circle its own subject",
		"One content word three times in a sentence is a sentence saying the same " +
			"thing twice and landing nowhere.", House},
	{"PROSE-108", "The words this house has decided against",
		"Plain English, preferred spellings, inclusive language, product names, " +
			"Latin abbreviations, time-bound words, typography, and a colour naming a " +
			"control a reader has to find.", Both},
	{"PROSE-109", "No word is written twice",
		"\"the the\" is read as one word by the eye that wrote it.", RedHat},
	{"PROSE-110", "An -ly adverb takes no hyphen",
		"It already modifies what follows it, so \"interactively-authenticated\" is a " +
			"hyphen doing nothing.", Both},
	{"PROSE-111", "No merge conflict marker survives in the text",
		"A merge left half-resolved, committed, and read by nobody since.", RedHat},
	{"PROSE-112", "The README does not carry a section of negatives",
		"A reader arrives at a README to find out what the software does. A section " +
			"listing what it will not do answers a question nobody asked yet, and it " +
			"belongs in the spec, where non-goals and hard limits are the point.", House},
	{"PROSE-113", "Prose does not assert a number the repository counts itself",
		"A count written into a sentence is right on the day it is written and wrong " +
			"by the next commit, and nothing fails when it drifts. Name the thing that " +
			"reports the number instead. Reads the countable list, and an empty one " +
			"disables it.", House},
}

// Linter checks a repository's prose.
type Linter struct {
	cfg      config.Prose
	splitter *mdtext.Splitter
	verbs    *regexp.Regexp
	counted  *regexp.Regexp
	terms    []compiledTerm
	termsPr  []compiledTerm
	glossary []glossaryTerm
}

// glossaryTerm is a term and the pattern that finds its first use. The pattern
// is built once here rather than per document, because a run compiles it for
// every term in every file otherwise.
type glossaryTerm struct {
	word string
	re   *regexp.Regexp
}

type compiledTerm struct {
	re     *regexp.Regexp
	advice string
	source Source
}

// skipDefault are the trees and files that are not this project's prose.
//
// CODE_OF_CONDUCT.md is a third-party document carried word for word, and
// OSS-109 fails a repository that has edited it. Checking its style here would
// ask for the one edit the readiness linter forbids, so the two rules would
// leave no file that satisfies both. The licence and NOTICE are held the same
// way by OSS-107 and OSS-108, and are listed in their Markdown spellings
// because that is the only shape of them this linter can see.
var skipDefault = []string{"node_modules", "vendor", "dist", "bin", "target",
	"build", ".git", "third_party", "testdata", "CHANGELOG.md",
	"CODE_OF_CONDUCT.md", "LICENSE.md", "LICENCE.md", "NOTICE.md"}

// New returns a prose linter tuned by the configuration given.
func New(cfg config.Prose) (*Linter, error) {
	// The list is written across lines to stay readable. Go's regexp has no
	// verbose flag, so the layout is removed rather than declared.
	verbs := strings.Join(strings.Fields(sharedVerbs), "")
	var verbsSb115 strings.Builder
	for _, v := range cfg.ProjectVerbs {
		verbsSb115.WriteString("|" + v)
	}
	verbs += verbsSb115.String()
	verbRE, err := regexp.Compile(`(?i)\b(` + verbs + `)\b`)
	if err != nil {
		return nil, fmt.Errorf("verb list: %w", err)
	}
	l := &Linter{
		cfg:      cfg,
		splitter: mdtext.NewSplitter(cfg.LowercaseStarters),
		verbs:    verbRE,
	}
	if len(cfg.Countable) > 0 {
		// The leading class rejects a number that continues one: a heading
		// numbered "9.1 cassette.yaml" is a section, and reading the "1" out
		// of it as a count is the rule crying wolf.
		l.counted, err = regexp.Compile(`(?i)(^|[^\d.,])(\d[\d,]{0,6})\s+(` +
			strings.Join(cfg.Countable, "|") + `)\b`)
		if err != nil {
			return nil, fmt.Errorf("countable list: %w", err)
		}
	}
	for _, term := range cfg.Glossary {
		re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(term) + `\w*\b`)
		if err != nil {
			return nil, fmt.Errorf("glossary term %q: %w", term, err)
		}
		l.glossary = append(l.glossary, glossaryTerm{term, re})
	}
	if l.terms, err = compileTerms(sharedTerms, cfg.Terms); err != nil {
		return nil, err
	}
	if l.termsPr, err = compileTerms(sharedTermsProse, cfg.TermsProse); err != nil {
		return nil, err
	}
	return l, nil
}

func compileTerms(shared []Term, extra map[string]string) ([]compiledTerm, error) {
	all := append([]Term(nil), shared...)
	for pattern, advice := range extra {
		all = append(all, Term{Pattern: pattern, Advice: advice, Source: House})
	}
	out := make([]compiledTerm, 0, len(all))
	for _, t := range all {
		re, err := regexp.Compile(`(?i)` + t.Pattern)
		if err != nil {
			return nil, fmt.Errorf("declined term %q: %w", t.Pattern, err)
		}
		out = append(out, compiledTerm{re: re, advice: t.Advice, source: t.Source})
	}
	return out, nil
}

// Files returns every file the linter checks: the Markdown at the repository
// root, in docs/ or doc/ if either exists, and the ledger's own records and
// rendered page where the repository keeps one. A project that keeps prose
// somewhere else adds it through skipExtra in reverse, by naming what to leave
// out.
func (l *Linter) Files(root string) ([]string, error) {
	skip := map[string]bool{}
	for _, s := range append(skipDefault, l.cfg.SkipExtra...) {
		skip[s] = true
	}
	var found []string
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && !skip[e.Name()] {
			found = append(found, e.Name())
		}
	}
	sort.Strings(found)
	for _, sub := range []string{"docs", "doc"} {
		dir := filepath.Join(root, sub)
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		var nested []string
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
				if skip[part] {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
				nested = append(nested, rel)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		sort.Strings(nested)
		found = append(found, nested...)
	}
	return append(found, l.ledgerFiles(root, skip)...), nil
}

var specRequirement = regexp.MustCompile(`(?m)^\*\*R\d+[a-z]?\.\*\*`)

// docClass says which kind of document this is.
//
// A spec is not a README with more detail. It states obligations in the
// passive, numbers them, and is written for a reader who already has the
// vocabulary, so a rule written for prose can fire constantly there and mean
// nothing. A document that numbers requirements in bold is a spec, which is
// the convention the document set is written to and needs no configuration.
func docClass(raw string) string {
	if len(specRequirement.FindAllString(raw, -1)) >= 3 {
		return "spec"
	}
	return "prose"
}

// Check reads one file and returns what is wrong with the prose in it.
func (l *Linter) Check(root, rel string) ([]lint.Problem, error) {
	pieces, err := l.pieces(root, rel)
	if err != nil {
		return nil, err
	}
	var out []lint.Problem
	for _, p := range pieces {
		out = append(out, l.check(p)...)
	}
	return out, nil
}

// check runs every rule over one run of prose.
func (l *Linter) check(p piece) []lint.Problem {
	raw, rel := p.text, p.rel
	text := mdtext.Prose(raw)

	var out []lint.Problem
	at := func(p lint.Problem, body string, off int) lint.Problem {
		return p.At(fmt.Sprintf("%s:%d", rel, lint.Line(body, off)))
	}

	// A record's title is its headline, and the page renders it as one. The
	// three rules measured over a paragraph pass over a Markdown heading for
	// the same reason: a headline is not a sentence, and asking it to be one
	// would ask for "Initial implementation" to grow a verb.
	if p.kind != title {
		out = append(out, l.paragraphRules(text, rel, p.kind)...)
	}
	out = append(out, l.declinedTerms(text, docClass(raw), rel)...)
	out = append(out, l.repeatedWords(text, rel)...)
	out = append(out, l.lyHyphens(text, rel)...)
	for _, m := range conflictMarker.FindAllStringIndex(raw, -1) {
		out = append(out, at(lint.Errorf("PROSE-111",
			"a merge conflict marker is still in the text"), raw, m[0]).
			Quoting(strings.TrimSpace(raw[m[0]:m[1]])))
	}
	if rel == "README.md" {
		for _, m := range negativeHeading.FindAllStringIndex(raw, -1) {
			out = append(out, lint.Errorf("PROSE-112",
				"a README section of negatives; non-goals and hard limits belong in the spec").
				At(fmt.Sprintf("%s:%d", rel, lint.Line(raw, m[0]))).
				Quoting(strings.TrimSpace(raw[m[0]:m[1]])))
		}
	}
	out = append(out, l.assertedCounts(text, rel)...)
	// PROSE-101 asks a document to introduce a term where it first uses one,
	// and only a document can. A record is read by somebody already inside the
	// project, with the documents beside it, and each of its fields is checked
	// on its own, so the rule would ask every record to introduce the whole
	// glossary again. A page label has nowhere to put a gloss at all.
	if p.kind == document {
		out = append(out, l.undefinedTerms(raw, rel)...)
	}
	out = append(out, l.throatClearing(text, rel)...)
	out = append(out, l.echoes(text, rel)...)
	out = append(out, l.unshownScripts(raw, rel)...)
	if p.anchor != "" {
		for i := range out {
			out[i] = out[i].At(p.anchor)
		}
	}
	return out
}

var conflictMarker = regexp.MustCompile(`(?m)^(?:<{7}|={7}|>{7})(?:\s.*)?$`)

// negativeHeading matches a section that says what the software is not.
//
// A spec earns these: non-goals and hard limits are what it exists to state.
// A README does not, which is why the check reads only that file.
var negativeHeading = regexp.MustCompile(`(?im)^#{2,6}\s+.*\b(?:` +
	`what it (?:will not|won't|does not|doesn't) do|` +
	`what (?:this|it) is ?n[o']t|` +
	`non-?goals?|limitations?|caveats?|known issues|shortcomings|drawbacks` +
	`)\b.*$`)

// paragraphRules covers the three rules measured over a paragraph: the em-dash
// budget, the sentence length, and the verbless epigram.
func (l *Linter) paragraphRules(text, rel string, kind pieceKind) []lint.Problem {
	// A page is split by the line rather than by the blank line. Markup, not
	// spacing, is what separates one block from the next in it, and the blocks
	// arrive here already stripped: run them together and a heading joins the
	// sentence under it into one long sentence that nobody wrote. A block that
	// wraps across source lines is read as its parts, which can only report
	// less than the whole would, never more.
	sep := "\n\n"
	if kind == page {
		sep = "\n"
	}
	var out []lint.Problem
	offset := 0
	for para := range strings.SplitSeq(text, sep) {
		start := offset
		offset += len(para) + len(sep)
		flat := mdtext.Flatten(para)
		if flat == "" || strings.HasPrefix(flat, "#") {
			continue
		}
		line := lint.Line(text, start)
		where := fmt.Sprintf("%s:%d", rel, line)

		// Counted per unit, not per paragraph: a list of "**Term** - meaning"
		// entries is a readable pattern, and one em-dash belongs to each item
		// rather than all of them to the list.
		for _, u := range mdtext.Units(para) {
			one := mdtext.Flatten(u)
			if n := strings.Count(one, "—"); n > maxEmDashesPerParagraph {
				out = append(out, lint.Errorf("PROSE-104",
					"%d em-dash(es); use a full stop, a comma, or cut the aside", n).
					At(where).Quoting(one))
			}
		}

		for _, u := range mdtext.Units(para) {
			for _, s := range l.splitter.Sentences(mdtext.Flatten(u)) {
				out = append(out, l.sentenceRules(s, where, kind)...)
			}
		}
	}
	return out
}

var linkOnly = regexp.MustCompile(`^[\[!]`)

func (l *Linter) sentenceRules(s, where string, kind pieceKind) []lint.Problem {
	words := strings.Fields(s)
	// Bullets and headings are not sentences.
	for _, p := range []string{"-", "*", ">", "#", "|", "[!["} {
		if strings.HasPrefix(strings.TrimLeft(s, " \t"), p) {
			return nil
		}
	}
	// A line that is only a link, or only a link plus a gloss, is a list entry
	// in prose clothing.
	if linkOnly.MatchString(s) && strings.Contains(s, "](") && len(words) < 14 {
		return nil
	}
	var out []lint.Problem
	if len(words) > maxSentenceWords {
		out = append(out, lint.Errorf("PROSE-103", "%d-word sentence (max %d)",
			len(words), maxSentenceWords).At(where).Quoting(s))
	}
	// A rendered page is checked for what a reader reads, and most of what a
	// reader reads on one is a control: "status all", "stale only", "reset".
	// None of those is a sentence missing its verb, and rewriting them into
	// sentences would make the page worse.
	if kind == page {
		return out
	}
	if len(words) >= 3 && len(words) <= maxEpigramWords && !l.verbs.MatchString(s) {
		out = append(out, lint.Errorf("PROSE-102",
			"no verb: an epigram, not a sentence").At(where).Quoting(s))
	}
	return out
}

func (l *Linter) declinedTerms(text, kind, rel string) []lint.Problem {
	terms := l.terms
	if kind != "spec" {
		terms = append(append([]compiledTerm(nil), terms...), l.termsPr...)
	}
	var out []lint.Problem
	for _, t := range terms {
		for _, m := range t.re.FindAllStringIndex(text, -1) {
			if mdtext.Quoted(text, m[0]) {
				continue // naming a word is not using it
			}
			found := strings.TrimSpace(text[m[0]:m[1]])
			msg := fmt.Sprintf("%s (found %q)", t.advice, found)
			if t.source != House {
				msg += fmt.Sprintf(" [%s]", t.source)
			}
			out = append(out, lint.Errorf("PROSE-108", "%s", msg).
				At(fmt.Sprintf("%s:%d", rel, lint.Line(text, m[0]))).
				Quoting(mdtext.Context(text, m[0])))
		}
	}
	return out
}

var repeated = regexp.MustCompile(`(?i)\b(\w+)[ \t]+(\w+)\b`)

func (l *Linter) repeatedWords(text, rel string) []lint.Problem {
	var out []lint.Problem
	for _, m := range repeated.FindAllStringSubmatchIndex(text, -1) {
		first := text[m[2]:m[3]]
		second := text[m[4]:m[5]]
		if !strings.EqualFold(first, second) {
			continue
		}
		switch strings.ToLower(first) {
		// "that that" is grammatical, and CODE is what the extraction above
		// leaves where an inline code span was.
		case "that", "had", "is", "code":
			continue
		}
		out = append(out, lint.Errorf("PROSE-109", "%q is written twice", first).
			At(fmt.Sprintf("%s:%d", rel, lint.Line(text, m[0]))).
			Quoting(mdtext.Context(text, m[0])))
	}
	return out
}

var lyHyphen = regexp.MustCompile(`(\w+ly)-(\w+)`)

func (l *Linter) lyHyphens(text, rel string) []lint.Problem {
	var out []lint.Problem
	for _, m := range lyHyphen.FindAllStringSubmatchIndex(text, -1) {
		// The preceding character must not continue a word, which is what the
		// original expressed as a negative lookbehind.
		if m[0] > 0 {
			if c := text[m[0]-1]; c == '-' || lint.IsWordByte(c) {
				continue
			}
		}
		adverb := strings.ToLower(text[m[2]:m[3]])
		if lyAdjectives[adverb] {
			continue
		}
		out = append(out, lint.Errorf("PROSE-110",
			"%q takes no hyphen: an -ly adverb already modifies what follows it",
			text[m[0]:m[1]]).
			At(fmt.Sprintf("%s:%d", rel, lint.Line(text, m[0]))).
			Quoting(mdtext.Context(text, m[0])))
	}
	return out
}

func (l *Linter) throatClearing(text, rel string) []lint.Problem {
	var out []lint.Problem
	lower := strings.ToLower(text)
	for _, phrase := range throatClearing {
		from := 0
		for {
			i := strings.Index(lower[from:], phrase)
			if i < 0 {
				break
			}
			pos := from + i
			from = pos + len(phrase)
			if mdtext.Quoted(text, pos) {
				continue // naming a phrase is not using it
			}
			out = append(out, lint.Errorf("PROSE-106",
				"%q comments on the writing; delete the frame", phrase).
				At(fmt.Sprintf("%s:%d", rel, lint.Line(text, pos))).
				Quoting(mdtext.Context(text, pos)))
		}
	}
	return out
}

var (
	mdLink      = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)
	contentWord = regexp.MustCompile(`[a-zA-Z][a-zA-Z'-]{3,}`)
)

// echoes reports one content word three times in a sentence, which reads as
// circling. It catches "X is the difference, and it is the difference that …":
// the sentence that says the same thing twice and lands nowhere.
func (l *Linter) echoes(text, rel string) []lint.Problem {
	var out []lint.Problem
	offset := 0
	for para := range strings.SplitSeq(text, "\n\n") {
		start := offset
		offset += len(para) + 2
		where := fmt.Sprintf("%s:%d", rel, lint.Line(text, start))
		for _, u := range mdtext.Units(para) {
			for _, sent := range l.splitter.Sentences(mdtext.Flatten(u)) {
				bare := mdLink.ReplaceAllString(sent, "") // link text and target
				seen := map[string]int{}
				for _, w := range contentWord.FindAllString(strings.ToLower(bare), -1) {
					if !common[w] {
						seen[w]++
					}
				}
				keys := make([]string, 0, len(seen))
				for w := range seen {
					keys = append(keys, w)
				}
				sort.Strings(keys)
				for _, w := range keys {
					if seen[w] >= maxEchoes {
						out = append(out, lint.Errorf("PROSE-107",
							"%q %d times in one sentence; it circles", w, seen[w]).
							At(where).Quoting(sent))
						break
					}
				}
			}
		}
	}
	return out
}

var (
	shownComment = regexp.MustCompile(`#\s*([\w.-]+\.(?:sh|py|js|rb))`)
	shownWritten = regexp.MustCompile(`(?:cat|tee)\s*>?\s*([\w./-]+\.(?:sh|py))`)
	scriptRun    = regexp.MustCompile(`\./([\w.-]+\.(?:sh|py|js|rb))`)
)

// unshownScripts reports a command that runs a script the document never
// showed the reader.
func (l *Linter) unshownScripts(raw, rel string) []lint.Problem {
	shown := map[string]bool{}
	for _, m := range shownComment.FindAllStringSubmatch(raw, -1) {
		shown[m[1]] = true
	}
	for _, m := range shownWritten.FindAllStringSubmatch(raw, -1) {
		shown[m[1]] = true
	}
	var out []lint.Problem
	for _, m := range scriptRun.FindAllStringSubmatchIndex(raw, -1) {
		name := raw[m[2]:m[3]]
		if shown[name] || strings.HasPrefix(name, "scripts/") {
			continue
		}
		// Inside backticks it is being named, not run.
		lineStart := strings.LastIndexByte(raw[:m[0]], '\n') + 1
		if strings.Count(raw[lineStart:m[0]], "`")%2 == 1 {
			continue
		}
		out = append(out, lint.Errorf("PROSE-105",
			"./%s is run but never shown to the reader", name).
			At(fmt.Sprintf("%s:%d", rel, lint.Line(raw, m[0]))).
			Quoting(mdtext.Context(raw, m[0])))
	}
	return out
}

var (
	// What counts as introducing a term on the spot.
	gloss      = regexp.MustCompile(`(?i)\b(is|are|means|holds|records|names|covers)\b|[:(]|\bcalled\b|\bthat is\b`)
	emphasis   = regexp.MustCompile(`\*([^*\n]+)\*`)
	glossTable = regexp.MustCompile(`(?m)^\|\s*\*\*([\w -]+)\*\*\s*\|`)
)

// undefinedTerms reports a glossary term used before anything introduces it.
// assertedCounts reports a number stated next to something this repository
// counts for itself.
//
// The text is prose with the fences, tables and raw HTML already removed, so a
// number inside a recorded sample is not read as a claim: that number is what
// the command printed, and the sample check is what holds it.
func (l *Linter) assertedCounts(text, rel string) []lint.Problem {
	if l.counted == nil {
		return nil
	}
	// A heading is a label rather than a claim, and a numbered one is where a
	// section number sits next to the noun the section is about.
	body := mdtext.StripHeadings(text)
	var out []lint.Problem
	for _, m := range l.counted.FindAllStringSubmatchIndex(body, -1) {
		phrase := strings.TrimSpace(body[m[2]:m[1]])
		noun := body[m[6]:m[7]]
		out = append(out, lint.Errorf("PROSE-113",
			"%q states a count that changes without this sentence; name what reports %s instead",
			phrase, noun).
			At(fmt.Sprintf("%s:%d", rel, lint.Line(body, m[0]))))
	}
	return out
}

func (l *Linter) undefinedTerms(raw, rel string) []lint.Problem {
	// Headings name a term without introducing it, and "# Cassettes" is not a
	// first use in any sense a reader cares about.
	body := mdtext.StripHeadings(mdtext.Prose(raw))
	body = mentions(body)

	// A glossary table introduces every term in it, wherever it sits.
	defined := map[string]bool{}
	for _, m := range glossTable.FindAllStringSubmatch(raw, -1) {
		defined[strings.ToLower(m[1])] = true
	}

	var out []lint.Problem
	for _, term := range l.glossary {
		if definedNearby(defined, term.word) {
			continue
		}
		m := term.re.FindStringIndex(body)
		if m == nil {
			continue
		}
		// The paragraph it lands in, and everything before it.
		before := body[:m[0]]
		paraStart := strings.LastIndex(body[:m[0]], "\n\n") + 2
		paraEnd := strings.Index(body[m[1]:], "\n\n")
		if paraEnd < 0 {
			paraEnd = len(body)
		} else {
			paraEnd += m[1]
		}
		para := body[paraStart:paraEnd]
		if gloss.MatchString(para) || strings.Contains(before, "](") {
			continue
		}
		out = append(out, lint.Errorf("PROSE-101",
			"%q used before anything introduces it", term.word).
			At(fmt.Sprintf("%s:%d", rel, lint.Line(body, m[0]))).
			Quoting(lint.Truncate(mdtext.Flatten(para), 110)))
	}
	return out
}

func definedNearby(defined map[string]bool, term string) bool {
	for d := range defined {
		if strings.HasPrefix(d, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

// mentions blanks single-asterisk emphasis, which is the use/mention
// convention: a sentence about the word *draft* is not a sentence that uses
// drafts. Double-asterisk bold is ordinary emphasis and stays.
func mentions(body string) string {
	var b strings.Builder
	last := 0
	for _, m := range emphasis.FindAllStringIndex(body, -1) {
		if m[0] > 0 && body[m[0]-1] == '*' {
			continue
		}
		if m[1] < len(body) && body[m[1]] == '*' {
			continue
		}
		b.WriteString(body[last:m[0]])
		b.WriteString("MENTION")
		last = m[1]
	}
	b.WriteString(body[last:])
	return b.String()
}

// Stats are the per-file measurements the stats flag prints.
type Stats struct {
	File      string
	Words     int
	AvgLength float64
	EmDashes  float64 // per hundred words
	You       int
}

var youRE = regexp.MustCompile(`(?i)\byou\b|\byour\b`)

// Stats measures one file. A file holding its prose in fields is measured
// whole: the record is the unit a writer thinks in, not the field.
func (l *Linter) Stats(root, rel string) (Stats, error) {
	pieces, err := l.pieces(root, rel)
	if err != nil {
		return Stats{}, err
	}
	var joined strings.Builder
	for _, p := range pieces {
		joined.WriteString(p.text)
		joined.WriteString("\n\n")
	}
	text := mdtext.Prose(joined.String())
	words := len(strings.Fields(text))
	var sents []string
	for para := range strings.SplitSeq(text, "\n\n") {
		for _, u := range mdtext.Units(para) {
			for _, s := range l.splitter.Sentences(mdtext.Flatten(u)) {
				if len(strings.Fields(s)) > 2 {
					sents = append(sents, s)
				}
			}
		}
	}
	total := 0
	for _, s := range sents {
		total += len(strings.Fields(s))
	}
	avg := 0.0
	if len(sents) > 0 {
		avg = float64(total) / float64(len(sents))
	}
	em := 0.0
	if words > 0 {
		em = float64(strings.Count(text, "—")) / float64(words) * 100
	}
	return Stats{rel, words, avg, em, len(youRE.FindAllString(text, -1))}, nil
}
