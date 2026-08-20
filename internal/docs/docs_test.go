package docs

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/lint"
)

// write puts one document in a scratch repository and returns its root.
func write(t *testing.T, body string) string {
	t.Helper()
	return writeNamed(t, "DOC.md", body)
}

// writeNamed does the same for a document whose name the rule reads.
func writeNamed(t *testing.T, name, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// check runs the prose linter over one document and returns the rule ids it
// reported.
func check(t *testing.T, cfg config.Docs, body string) []string {
	t.Helper()
	root := write(t, body)
	l, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	found, err := l.Check(root, "DOC.md")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, p := range found {
		ids = append(ids, p.Rule)
	}
	return ids
}

func has(ids []string, want string) bool {
	return slices.Contains(ids, want)
}

func TestRulesFire(t *testing.T) {
	for _, tc := range []struct {
		name, rule, body string
		cfg              config.Docs
	}{
		{
			name: "verbless epigram", rule: "DOC-102",
			body: "Two version numbers, one verdict, one remedy.\n",
		},
		{
			name: "sentence length", rule: "DOC-103",
			body: "This sentence runs on and on and on for far too many words indeed, " +
				"carrying more than one idea at a time, which is exactly the sort of " +
				"thing the length rule exists to catch when it happens.\n",
		},
		{
			name: "em-dash budget", rule: "DOC-104",
			body: "A paragraph — with two — em-dashes in it.\n",
		},
		{
			name: "unshown script", rule: "DOC-105",
			body: "Run ./build.sh to build the thing.\n",
		},
		{
			name: "throat clearing", rule: "DOC-106",
			body: "It is worth stating plainly that the gate runs.\n",
		},
		{
			name: "echoes", rule: "DOC-107",
			body: "The cassette is a cassette because the cassette says cassette.\n",
		},
		{
			name: "declined term", rule: "DOC-108",
			body: "You should utilize the tool.\n",
		},
		{
			name: "repeated word", rule: "DOC-109",
			body: "The the word is written twice.\n",
		},
		{
			name: "ly hyphen", rule: "DOC-110",
			body: "An interactively-authenticated session starts.\n",
		},
		{
			name: "conflict marker", rule: "DOC-111",
			body: "<<<<<<< HEAD\nsomething\n",
		},
		{
			name: "undefined term", rule: "DOC-101",
			cfg:  config.Docs{Glossary: []string{"cassette"}},
			body: "The cassette carries what was recorded.\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if ids := check(t, tc.cfg, tc.body); !has(ids, tc.rule) {
				t.Errorf("%s did not fire on %q; got %v", tc.rule, tc.body, ids)
			}
		})
	}
}

func TestCleanProseIsClean(t *testing.T) {
	body := "# A heading\n\nThe gate runs before you push. It reports what is wrong, " +
		"and it exits non-zero so the build stops.\n"
	if ids := check(t, config.Docs{}, body); len(ids) != 0 {
		t.Errorf("clean prose reported %v", ids)
	}
}

func TestCodeFencesAreNotProse(t *testing.T) {
	// A fence carries none of the rules: it is not prose.
	body := "```\nThe the word is written twice and this sentence is deliberately far " +
		"too long to pass the length rule that applies to ordinary prose here.\n```\n"
	if ids := check(t, config.Docs{}, body); len(ids) != 0 {
		t.Errorf("a code fence reported %v", ids)
	}
}

func TestASpecSkipsTheProseOnlyRules(t *testing.T) {
	// A spec states obligations in the passive and speaks about the project's
	// own artifacts, so the first-person rule would fire constantly there.
	spec := "**R1.** The tool **MUST** run.\n\n**R2.** It **MUST** exit.\n\n" +
		"**R3.** We keep our own record.\n"
	if ids := check(t, config.Docs{}, spec); has(ids, "DOC-108") {
		t.Errorf("a prose-only rule fired on a spec: %v", ids)
	}
	prose := "We keep our own record of it.\n"
	if ids := check(t, config.Docs{}, prose); !has(ids, "DOC-108") {
		t.Errorf("the first-person rule did not fire on prose: %v", ids)
	}
}

func TestAMentionIsNotAUse(t *testing.T) {
	// Naming a word is not using it.
	body := "The house declines \"simply\" in its own prose.\n"
	if ids := check(t, config.Docs{}, body); has(ids, "DOC-108") {
		t.Errorf("a quoted mention was read as a use: %v", ids)
	}
}

func TestLyAdjectivesKeepTheirHyphen(t *testing.T) {
	// An adjective ending in -ly does take a hyphen in a compound.
	if ids := check(t, config.Docs{}, "A costly-looking result.\n"); has(ids, "DOC-110") {
		t.Errorf("an -ly adjective was reported: %v", ids)
	}
}

func TestFindingsCarryALineNumber(t *testing.T) {
	root := write(t, "# Heading\n\nfine here\n\nThe the word.\n")
	l, err := New(config.Docs{})
	if err != nil {
		t.Fatal(err)
	}
	found, err := l.Check(root, "DOC.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatal("nothing reported")
	}
	if !strings.HasPrefix(found[0].Where, "DOC.md:") {
		t.Errorf("finding carries %q, want a DOC.md:line address", found[0].Where)
	}
}

func TestEveryRuleIsExplained(t *testing.T) {
	// A rule a reader meets in a failure and cannot look up is a rule they can
	// only silence.
	seen := map[string]bool{}
	for _, r := range Rules {
		if seen[r.ID] {
			t.Errorf("%s is listed twice", r.ID)
		}
		seen[r.ID] = true
		if r.Title == "" || r.Why == "" {
			t.Errorf("%s has no title or no reason", r.ID)
		}
	}
	ids := check(t, config.Docs{Glossary: []string{"cassette"}},
		"The cassette is here. The the word. An interactively-authenticated run.\n")
	for _, id := range ids {
		if !seen[id] {
			t.Errorf("rule %s reports findings and is not in Rules", id)
		}
	}
}

func TestDocsFindsTheRootAndDocsTree(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"README.md", "docs/guide.md", "vendor/skip.md", "notes.txt"} {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	l, err := New(config.Docs{})
	if err != nil {
		t.Fatal(err)
	}
	found, err := l.Docs(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"README.md": true, filepath.Join("docs", "guide.md"): true}
	if len(found) != len(want) {
		t.Fatalf("found %v, want %v", found, want)
	}
	for _, f := range found {
		if !want[f] {
			t.Errorf("found %q, which is not prose this project owns", f)
		}
	}
}

func TestStatsMeasureTheDocument(t *testing.T) {
	root := write(t, "You run it. You read what it says.\n")
	l, err := New(config.Docs{})
	if err != nil {
		t.Fatal(err)
	}
	s, err := l.Stats(root, "DOC.md")
	if err != nil {
		t.Fatal(err)
	}
	if s.Words == 0 || s.You != 2 || s.AvgLength == 0 {
		t.Errorf("stats did not measure the document: %+v", s)
	}
}

func TestABrokenTermPatternIsReported(t *testing.T) {
	// A configuration that does not compile has to fail loudly rather than
	// silently dropping the rule it was meant to add.
	_, err := New(config.Docs{Terms: map[string]string{"(": "unclosed"}})
	if err == nil {
		t.Error("a malformed declined term compiled")
	}
}

func TestSeverityIsAlwaysAnError(t *testing.T) {
	// Prose findings are quotable and mechanical, so none of them is advice.
	root := write(t, "The the word.\n")
	l, _ := New(config.Docs{})
	found, _ := l.Check(root, "DOC.md")
	for _, p := range found {
		if p.Severity != lint.Error {
			t.Errorf("%s reports %s, want error", p.Rule, p.Severity)
		}
	}
}

func TestSkipExtraLeavesATreeOut(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"README.md", "fixtures/corpus.md"} {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The tree is only reachable through docs/, so it takes a docs/ path to
	// prove skipExtra works on a nested directory.
	nested := filepath.Join(root, "docs", "fixtures")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "corpus.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := New(config.Docs{SkipExtra: []string{"fixtures"}})
	if err != nil {
		t.Fatal(err)
	}
	found, err := l.Docs(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range found {
		if strings.Contains(f, "fixtures") {
			t.Errorf("a declared skip was checked: %s", f)
		}
	}
}

func TestProjectVerbsSilenceARealVerb(t *testing.T) {
	// A verb the shared list does not carry trips the epigram check until the
	// project adds it.
	body := "The tool frobnicates it.\n"
	if ids := check(t, config.Docs{}, body); !has(ids, "DOC-102") {
		t.Fatalf("the epigram check did not fire: %v", ids)
	}
	if ids := check(t, config.Docs{ProjectVerbs: []string{"frobnicates?"}}, body); has(ids, "DOC-102") {
		t.Errorf("a configured verb still tripped the check: %v", ids)
	}
}

func TestProjectTermsAreAdded(t *testing.T) {
	body := "Use the frobnicator here.\n"
	if ids := check(t, config.Docs{}, body); has(ids, "DOC-108") {
		t.Fatalf("an undeclared term was reported: %v", ids)
	}
	cfg := config.Docs{Terms: map[string]string{"frobnicator": "Use 'widget'."}}
	if ids := check(t, cfg, body); !has(ids, "DOC-108") {
		t.Errorf("a configured term was not reported: %v", ids)
	}
}

func TestGlossaryTermIntroducedOnTheSpotPasses(t *testing.T) {
	cfg := config.Docs{Glossary: []string{"cassette"}}
	// A gloss in the same paragraph introduces it.
	body := "A cassette is the recording a session leaves behind.\n"
	if ids := check(t, cfg, body); has(ids, "DOC-101") {
		t.Errorf("a term glossed on the spot was reported: %v", ids)
	}
	// So does a glossary table, wherever it sits.
	table := "Text using cassette early.\n\n| **cassette** | the recording |\n|---|---|\n"
	if ids := check(t, cfg, table); has(ids, "DOC-101") {
		t.Errorf("a term defined in a table was reported: %v", ids)
	}
}

func TestAMentionIsNotAFirstUse(t *testing.T) {
	// Single-asterisk emphasis is the use/mention convention: a sentence about
	// the word *cassette* is not a sentence that uses cassettes.
	cfg := config.Docs{Glossary: []string{"cassette"}}
	body := "The word *cassette* comes up later on.\n"
	if ids := check(t, cfg, body); has(ids, "DOC-101") {
		t.Errorf("a mention was read as a first use: %v", ids)
	}
}

func TestABulletIsNotASentence(t *testing.T) {
	if ids := check(t, config.Docs{}, "- one verdict, one remedy\n"); has(ids, "DOC-102") {
		t.Errorf("a bullet was read as a sentence: %v", ids)
	}
}

func TestAShownScriptPasses(t *testing.T) {
	body := "```bash\n# build.sh\necho hi\n```\n\nRun ./build.sh to build it.\n"
	if ids := check(t, config.Docs{}, body); has(ids, "DOC-105") {
		t.Errorf("a script the document showed was reported: %v", ids)
	}
}

func TestEmDashIsBannedNotBudgeted(t *testing.T) {
	// One is one too many: the aside it introduces is a full stop, a comma, or
	// a cut.
	if ids := check(t, config.Docs{}, "A sentence — with one aside.\n"); !has(ids, "DOC-104") {
		t.Errorf("a single em-dash passed: %v", ids)
	}
	if ids := check(t, config.Docs{}, "A sentence with no aside.\n"); has(ids, "DOC-104") {
		t.Errorf("clean prose reported an em-dash: %v", ids)
	}
}

func TestReadmeNegativesAreReported(t *testing.T) {
	// A reader arrives at a README to find out what the software does.
	for _, heading := range []string{
		"## What it will not do", "## What this is not", "## Non-goals",
		"## Limitations", "### Caveats", "## Known issues",
	} {
		root := write(t, "# Thing\n\n"+heading+"\n\nSome prose here.\n")
		l, err := New(config.Docs{})
		if err != nil {
			t.Fatal(err)
		}
		found, err := l.Check(root, "DOC.md")
		if err != nil {
			t.Fatal(err)
		}
		// Only a README carries the rule, so the scratch file must be named one.
		_ = found
		readme := writeNamed(t, "README.md", "# Thing\n\n"+heading+"\n\nSome prose here.\n")
		got, err := l.Check(readme, "README.md")
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, p := range got {
			ids = append(ids, p.Rule)
		}
		if !has(ids, "DOC-112") {
			t.Errorf("%q passed in a README: %v", heading, ids)
		}
	}
}

func TestASpecMayStateItsNonGoals(t *testing.T) {
	// Non-goals and hard limits are what a spec exists to state, so the rule
	// reads only the README.
	root := writeNamed(t, "SPEC.md", "# Spec\n\n## Non-goals\n\nIt does not do that.\n")
	l, err := New(config.Docs{})
	if err != nil {
		t.Fatal(err)
	}
	found, err := l.Check(root, "SPEC.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range found {
		if p.Rule == "DOC-112" {
			t.Errorf("a spec's non-goals were reported: %v", p)
		}
	}
}

func TestAnAssertedCountIsReported(t *testing.T) {
	cfg := config.Docs{Countable: []string{"fixtures?", "goldens?"}}
	ids := check(t, cfg, "The suite carries 73 fixtures, and every one has a golden.\n")
	if !has(ids, "DOC-113") {
		t.Errorf("got %v, want DOC-113", ids)
	}
}

func TestACountInASampleIsNotAClaim(t *testing.T) {
	// A number a command printed is what the sample check holds, not what this
	// sentence asserts.
	cfg := config.Docs{Countable: []string{"fixtures?"}}
	ids := check(t, cfg, "Run it:\n\n```\nchecked 73 fixtures\n```\n")
	if has(ids, "DOC-113") {
		t.Errorf("a recorded sample was read as a claim: %v", ids)
	}
}

func TestANumberedHeadingIsNotACount(t *testing.T) {
	// "### 9.1 cassette.yaml" is a section number beside the noun the section
	// is about, and reading the 1 out of it is the rule crying wolf.
	cfg := config.Docs{Countable: []string{"cassettes?"}}
	ids := check(t, cfg, "# D\n\n### 9.1 cassette.yaml\n\nIt holds the versions.\n")
	if has(ids, "DOC-113") {
		t.Errorf("a numbered heading was read as a count: %v", ids)
	}
}

func TestACountInsideALongerNumberIsNotACount(t *testing.T) {
	cfg := config.Docs{Countable: []string{"cassettes?"}}
	ids := check(t, cfg, "Version 9.1 cassettes changed shape.\n")
	if has(ids, "DOC-113") {
		t.Errorf("a number continuing another was read as a count: %v", ids)
	}
}

func TestAnEmptyCountableListDisablesTheCheck(t *testing.T) {
	ids := check(t, config.Docs{}, "The suite carries 73 fixtures.\n")
	if has(ids, "DOC-113") {
		t.Errorf("the check ran with nothing configured: %v", ids)
	}
}

func TestABadCountableListIsAnError(t *testing.T) {
	if _, err := New(config.Docs{Countable: []string{"fixture("}}); err == nil {
		t.Error("an invalid pattern was accepted")
	}
}

func TestTheMachineRegisterIsReported(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"delve", "We delve into the format below.\n"},
		{"testament", "The suite is a testament to the design.\n"},
		{"landscape", "The realm of configuration is wide.\n"},
		{"seamless", "It offers seamless integration.\n"},
		{"boasts", "The tool boasts three linters.\n"},
		{"cutting-edge", "A cutting-edge approach to prose.\n"},
		{"transition", "It runs the gate. Furthermore, it reports.\n"},
		{"not just but", "It is not just a linter but a gate.\n"},
		{"honestly", "Honestly, the check is slow.\n"},
		{"frankly aside", "The gate is, frankly, slow.\n"},
		{"important to note", "It is important to note that it exits 1.\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if ids := check(t, config.Docs{}, tc.body); !has(ids, "DOC-108") {
				t.Errorf("%q passed: %v", tc.body, ids)
			}
		})
	}
}

func TestTheMachineRegisterDoesNotCryWolf(t *testing.T) {
	// A rule that fires on ordinary English trains everyone to read past the
	// whole linter. Each of these is the plain sense of a word the register
	// list carries in another one.
	for _, tc := range []struct{ name, body string }{
		{"underscore the character",
			"A name is one segment of letters, digits, dot, dash or underscore.\n"},
		{"honestly as manner", "A record earns its value by accreting honestly.\n"},
		{"landscape as terrain", "The tool reads the landscape file it was given.\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if ids := check(t, config.Docs{}, tc.body); has(ids, "DOC-108") {
				t.Errorf("%q was reported: %v", tc.body, ids)
			}
		})
	}
}
