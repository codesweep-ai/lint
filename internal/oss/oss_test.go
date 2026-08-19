package oss

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/lint"
	"github.com/codesweep-ai/lint/internal/lint/linttest"
)

// run applies one rule and returns what it found.
func run(t *testing.T, id string, cfg config.OSS, files map[string]string) []lint.Problem {
	t.Helper()
	base := config.Default().OSS
	if cfg.DocSet == nil {
		cfg.DocSet = base.DocSet
	}
	if cfg.HomeAllow == nil {
		cfg.HomeAllow = base.HomeAllow
	}
	if cfg.BinaryOK == nil {
		cfg.BinaryOK = base.BinaryOK
	}
	l := New(cfg, linttest.Repo(t, files))
	for _, r := range rules {
		if r.id == id {
			return l.runOne(r)
		}
	}
	t.Fatalf("no rule %s", id)
	return nil
}

func firstError(problems []lint.Problem) *lint.Problem {
	for i, p := range problems {
		if p.Severity == lint.Error {
			return &problems[i]
		}
	}
	return nil
}

func TestLicenceRules(t *testing.T) {
	t.Run("missing licence", func(t *testing.T) {
		got := run(t, "OSS-101", config.OSS{}, map[string]string{"README.md": "# x\n"})
		if firstError(got) == nil {
			t.Error("a repository with no LICENSE passed")
		}
	})
	t.Run("a summary is not a grant", func(t *testing.T) {
		got := run(t, "OSS-102", config.OSS{}, map[string]string{"LICENSE": "Apache-2.0\n"})
		if firstError(got) == nil {
			t.Error("an SPDX line passed as the full text")
		}
	})
	t.Run("the appendix placeholders", func(t *testing.T) {
		got := run(t, "OSS-103", config.OSS{}, map[string]string{
			"LICENSE": "Copyright [yyyy] [name of copyright owner]\n"})
		if firstError(got) == nil {
			t.Error("a licence granting rights on behalf of nobody passed")
		}
	})
	t.Run("a filled-in copyright line", func(t *testing.T) {
		got := run(t, "OSS-103", config.OSS{}, map[string]string{
			"LICENSE": "Copyright 2026 Codesweep\n"})
		if firstError(got) != nil {
			t.Errorf("a real copyright line was reported: %v", got)
		}
	})
}

func TestLeakScanCatchesTheClass(t *testing.T) {
	for _, tc := range []struct{ name, rule, body string }{
		{"home directory", "OSS-301", "the build ran in /home/jdoe/work\n"},
		{"macOS home", "OSS-301", "see /Users/jdoe/Library for it\n"},
		// A state path is caught under OSS-301: the scan reports the first
		// pattern that matches a file, and every state path is also a home
		// directory. One report per file is enough to act on.
		{"state path", "OSS-301", "reads /home/jdoe/.ssh/config first\n"},
		{"mail address", "OSS-303", "write to ada@realcompany.co.uk about it\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, tc.rule, config.OSS{}, map[string]string{"notes.txt": tc.body})
			if firstError(got) == nil {
				t.Errorf("%s did not catch %q", tc.rule, tc.body)
			}
		})
	}
}

func TestLeakScanPermitsAPlaceholder(t *testing.T) {
	// A username is the segment after /home/ that is not a placeholder the
	// project ships. The allowance is by whole name, so /home/user passes and
	// /home/username does not.
	got := run(t, "OSS-301", config.OSS{}, map[string]string{"notes.txt": "under /home/user/work\n"})
	if firstError(got) != nil {
		t.Errorf("the placeholder /home/user was reported: %v", got)
	}
	got = run(t, "OSS-301", config.OSS{}, map[string]string{"notes.txt": "under /home/username/work\n"})
	if firstError(got) == nil {
		t.Error("/home/username passed as the placeholder /home/user")
	}
}

func TestMailAllowanceIsByWholeDomain(t *testing.T) {
	got := run(t, "OSS-303", config.OSS{}, map[string]string{"n.txt": "ada@example.com\n"})
	if firstError(got) != nil {
		t.Errorf("a documentation domain was reported: %v", got)
	}
	got = run(t, "OSS-303", config.OSS{}, map[string]string{"n.txt": "ada@example.company.io\n"})
	if firstError(got) == nil {
		t.Error("example.company.io passed as the reserved example.com")
	}
}

func TestSecretsAreCaughtAndFixturesExempt(t *testing.T) {
	// Not a repeated character: the fake markers are matched without regard to
	// case, so a run of four a's would exempt the fixture as "AAAA".
	real := "token = \"ghp_b1c2d3e4f5g6h7i8j9k1l2m3n4o5p6q7r8\"\n"
	if firstError(run(t, "OSS-305", config.OSS{}, map[string]string{"c.txt": real})) == nil {
		t.Error("a GitHub-shaped token passed")
	}
	// A test needs a credential-shaped string, and the only safe one says of
	// itself that it is fake.
	fake := "token = \"sk-ant-api03-" + strings.Repeat("A", 20) + "\"\n"
	if firstError(run(t, "OSS-305", config.OSS{}, map[string]string{"c.txt": fake})) != nil {
		t.Error("an obviously fake fixture was reported")
	}
}

func TestOneReportPerFile(t *testing.T) {
	// One report per file is enough to act on; a generated page would otherwise
	// print thousands.
	body := strings.Repeat("/home/jdoe/work\n", 50)
	got := run(t, "OSS-301", config.OSS{}, map[string]string{"n.txt": body})
	if len(got) != 1 {
		t.Errorf("got %d reports for one file, want 1", len(got))
	}
}

func TestSkipPathsNeedAReason(t *testing.T) {
	// A waiver with no reason is one nobody can review, so the reason travels
	// with the finding and is printed.
	files := map[string]string{"fixtures/x.txt": "/home/jdoe/work\n"}
	got := run(t, "OSS-301", config.OSS{
		SkipPaths: map[string]string{"fixtures/": "captured upstream, scrubbed by make scrub"},
	}, files)
	if firstError(got) != nil {
		t.Errorf("a declared skip still reported: %v", got)
	}
}

func TestWaiverDowngradesAndSaysWhy(t *testing.T) {
	problems := []lint.Problem{lint.Errorf("OSS-204", "no licence badge")}
	got := lint.Waive(problems, map[string]string{"OSS-204": "the badge lives in the docs site"})
	if got[0].Severity != lint.Skip {
		t.Errorf("a waived error is %s, want skip", got[0].Severity)
	}
	if !strings.Contains(got[0].Message, "the badge lives in the docs site") {
		t.Errorf("the waiver's reason is not printed: %q", got[0].Message)
	}
}

func TestUnreadableFileIsNeverClean(t *testing.T) {
	// A file nobody can inspect must never be reported as clean.
	got := run(t, "OSS-306", config.OSS{}, map[string]string{"blob.dat": "\xff\xfe\x00binary"})
	if firstError(got) == nil {
		t.Error("a file that cannot be read as text passed as clean")
	}
}

func TestBinaryAssetsAreAllowed(t *testing.T) {
	got := run(t, "OSS-306", config.OSS{}, map[string]string{"logo.png": "\xff\xd8\xff\x00"})
	if firstError(got) != nil {
		t.Errorf("a known binary asset was reported: %v", got)
	}
}

func TestDocumentSetAndRouter(t *testing.T) {
	files := map[string]string{"README.md": "# x\n", "AGENTS.md": "This file routes.\n"}
	if firstError(run(t, "OSS-201", config.OSS{}, files)) == nil {
		t.Error("an incomplete document set passed")
	}
	got := run(t, "OSS-202", config.OSS{DocSet: []string{"README.md", "SPEC.md", "AGENTS.md"}}, files)
	if firstError(got) == nil {
		t.Error("a router that omits a document passed")
	}
}

func TestLinksAndAnchorsResolve(t *testing.T) {
	files := map[string]string{
		"README.md": "See [gone](docs/NOPE.md) and [bad](SPEC.md#no-such).\n",
		"SPEC.md":   "# Real Heading\n",
	}
	if firstError(run(t, "OSS-206", config.OSS{}, files)) == nil {
		t.Error("a link to a file that is not there passed")
	}
	if firstError(run(t, "OSS-207", config.OSS{}, files)) == nil {
		t.Error("an anchor matching no heading passed")
	}
	ok := map[string]string{
		"README.md": "See [real](SPEC.md#real-heading).\n",
		"SPEC.md":   "# Real Heading\n",
	}
	if firstError(run(t, "OSS-207", config.OSS{}, ok)) != nil {
		t.Error("an anchor that resolves was reported")
	}
}

func TestSecurityContactMustNameAChannel(t *testing.T) {
	vague := map[string]string{"CONTRIBUTING.md": "For a security-sensitive issue, " +
		"please ask for a private contact.\n"}
	if firstError(run(t, "OSS-212", config.OSS{}, vague)) == nil {
		t.Error("a promise with nothing behind it passed")
	}
	named := map[string]string{"CONTRIBUTING.md": "For a security issue, use GitHub's " +
		"private vulnerability reporting on this repository's Security tab.\n"}
	if firstError(run(t, "OSS-212", config.OSS{}, named)) != nil {
		t.Error("a named mechanism was reported")
	}
}

func TestActionsArePinned(t *testing.T) {
	files := map[string]string{".github/workflows/ci.yml": "permissions:\n  contents: read\njobs:\n" +
		"  a:\n    steps:\n      - uses: actions/checkout@main\n"}
	if firstError(run(t, "OSS-404", config.OSS{}, files)) == nil {
		t.Error("an action pinned to @main passed")
	}
	files[".github/workflows/ci.yml"] = strings.Replace(files[".github/workflows/ci.yml"],
		"@main", "@v7", 1)
	if firstError(run(t, "OSS-404", config.OSS{}, files)) != nil {
		t.Error("an action pinned to a version was reported")
	}
}

func TestCheckMustReachBothLinters(t *testing.T) {
	// A target that hangs off nothing is a target nobody runs.
	files := map[string]string{"Makefile": "check: fmt-check vet test\n\t@true\n"}
	got := run(t, "OSS-411", config.OSS{}, files)
	if len(got) != 2 {
		t.Errorf("got %d findings, want 2 (docs and oss): %v", len(got), got)
	}
	files["Makefile"] = "check: fmt-check vet test docs oss\n\t@true\ndocs:\n\tcs-lint docs\noss:\n\tcs-lint oss\n"
	if firstError(run(t, "OSS-411", config.OSS{}, files)) != nil {
		t.Error("a check target that reaches both linters was reported")
	}
}

func TestCheckTargetRecipeStopsAtTheTarget(t *testing.T) {
	// [^\S\n] rather than \s: a greedy pattern crosses the newline and swallows
	// the tab-indented recipe, so a delegating target reads as its own
	// prerequisite.
	prereqs, recipe, ok := checkTarget("check: fmt vet\n\tcs-lint docs\n\nother:\n\techo no\n")
	if !ok {
		t.Fatal("no check target found")
	}
	if len(prereqs) != 2 {
		t.Errorf("prerequisites are %v, want two", prereqs)
	}
	if strings.Contains(recipe, "echo no") {
		t.Errorf("the recipe swallowed the next target: %q", recipe)
	}
}

func TestSiblingCheckoutJudgedByTheTail(t *testing.T) {
	// A tail that names something in the tree is a path back into it, however
	// many levels it climbs first.
	files := map[string]string{
		"Makefile":         "\tcp ../../elsewhere/thing .\n",
		"scripts/build.sh": "cp ../../vendor/thing .\n",
		"vendor/thing":     "x\n",
	}
	got := run(t, "OSS-502", config.OSS{}, files)
	if firstError(got) == nil {
		t.Error("a path leaving the repository passed")
	}
	for _, p := range got {
		if strings.Contains(p.Where, "scripts/build.sh") {
			t.Errorf("a path back into the tree was reported: %v", p)
		}
	}
}

func TestHistoryRulesBecomeAdviceOncePublished(t *testing.T) {
	// Published history cannot be rewritten, so the rule becomes advice.
	private := New(config.OSS{Published: false}, lint.NewRepo(t.TempDir()))
	if private.historySeverity() != lint.Error {
		t.Error("an unpublished repository's history rules are not errors")
	}
	public := New(config.OSS{Published: true}, lint.NewRepo(t.TempDir()))
	if public.historySeverity() != lint.Warn {
		t.Error("a published repository's history rules are not warnings")
	}
}

func TestEveryRuleIsExplained(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Explain() {
		if seen[r.ID] {
			t.Errorf("%s is listed twice", r.ID)
		}
		seen[r.ID] = true
		if r.Title == "" || r.Why == "" {
			t.Errorf("%s has no title or no reason", r.ID)
		}
	}
	if len(seen) < 60 {
		t.Errorf("only %d rules are registered", len(seen))
	}
}

func TestABrokenCheckDoesNotHideTheRest(t *testing.T) {
	l := New(config.OSS{}, lint.NewRepo(t.TempDir()))
	got := l.runOne(rule{id: "OSS-999", severity: lint.Error,
		check: func(*Linter) []lint.Problem { panic("boom") }})
	if len(got) != 1 || got[0].Severity != lint.Warn {
		t.Fatalf("a panicking check produced %v", got)
	}
	if !strings.Contains(got[0].Message, "the check itself failed") {
		t.Errorf("the failure is not reported as one: %q", got[0].Message)
	}
}

// wellFormed is a repository that satisfies as much of the rule set as a
// fixture can, so a full run exercises every rule's success path.
func wellFormed() map[string]string {
	return map[string]string{
		"LICENSE": "Apache License\nVersion 2.0, January 2004\n" +
			"TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION\n" +
			"Copyright 2026 Codesweep\n",
		"README.md": "# thing\n\n> **One sentence.**\n\n" +
			"[![CI](https://github.com/acme/thing/actions/workflows/ci.yml/badge.svg)]" +
			"(https://github.com/acme/thing/actions/workflows/ci.yml)\n" +
			"[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)\n\n" +
			"## License\n\n[Apache-2.0](LICENSE).\n",
		"INSTALL.md": "# Install\n\nGo 1.26+ and a release download.\n",
		"MANUAL.md":  "# Manual\n\n## Notes for agents\n\nNon-interactive.\n",
		"SPEC.md":    "# Spec\n\n**R1.** It **MUST** run.\n",
		"CONTRIBUTING.md": "# Contributing\n\n## Commits\n\nRun `make check`. Drop any " +
			"session trailer.\n\n## Writing\n\nUse short sentences. For a security issue, " +
			"use GitHub's private vulnerability reporting.\n",
		"AGENTS.md": "# Working here\n\nThis file routes.\n\n- README.md\n- INSTALL.md\n" +
			"- MANUAL.md\n- SPEC.md\n- CONTRIBUTING.md\n",
		"Makefile": ".DEFAULT_GOAL := help\nhelp:\n\t@true\nbuild:\n\t@true\ntest:\n\t@true\n" +
			"install:\n\t@true\nuninstall:\n\t@true\nfmt:\n\t@true\nfmt-check:\n\t@true\n" +
			"vet:\n\t@true\nlint:\n\tgolangci-lint run\ndeadcode:\n\t@true\n" +
			"docs:\n\tcs-lint docs\noss:\n\tcs-lint oss\nclean:\n\t@true\n" +
			"check: fmt-check vet lint deadcode test docs oss\n\t@true\n",
		".gitignore":     "# what never enters the tree\n/bin/\n/.env\n",
		".gitattributes": "* text=auto eol=lf\n",
		"go.mod":         "module github.com/acme/thing\n\ngo 1.26.0\n",
		".github/workflows/ci.yml": "name: ci\non:\n  push:\n  pull_request:\n  workflow_dispatch:\n" +
			"permissions:\n  contents: read\njobs:\n  build:\n    steps:\n" +
			"      - uses: actions/checkout@v7\n      - run: make check\n" +
			"  docs:\n    steps:\n      - run: make docs\n" +
			"  oss:\n    steps:\n      - uses: actions/checkout@v7\n        with:\n" +
			"          fetch-depth: 0\n      - run: make oss\n",
		".github/workflows/release.yml": "name: release\non:\n  push:\n    tags: [\"v*\"]\n" +
			"permissions:\n  contents: write\njobs:\n  release:\n    steps:\n" +
			"      - uses: actions/checkout@v7\n",
		".cs-lint.yaml": "docs:\n  glossary: [thing]\n",
	}
}

func TestAFullRunExercisesEveryRule(t *testing.T) {
	// Every rule runs against a well-formed repository. Nothing here asserts a
	// particular verdict: the point is that no rule panics, and that the
	// success path of each is walked.
	cfg := config.Default().OSS
	cfg.Project = "thing"
	cfg.GitHubRepo = "acme/thing"
	l := New(cfg, linttest.Repo(t, wellFormed()))
	got := l.Run()
	if len(got) == 0 {
		t.Fatal("a full run reported nothing at all, not even a skip")
	}
	seen := map[string]bool{}
	for _, p := range got {
		seen[p.Rule] = true
		if strings.Contains(p.Message, "the check itself failed") {
			t.Errorf("%s panicked: %s", p.Rule, p.Message)
		}
	}
	// A rule that finds nothing is silent, so a well-formed repository proves
	// the success paths ran by reporting few errors rather than many.
	errors, _, _ := lint.Count(got)
	if errors > 4 {
		t.Errorf("a well-formed repository reported %d errors:", errors)
		for _, p := range got {
			if p.Severity == lint.Error {
				t.Logf("  %s", p.Format())
			}
		}
	}
}

func TestAFullRunOnABareRepository(t *testing.T) {
	// The other extreme: a repository with nothing in it. Every rule has to
	// survive the absence of everything it reads.
	l := New(config.Default().OSS, linttest.Repo(t, map[string]string{"a.txt": "x\n"}))
	got := l.Run()
	errors, _, skips := 0, 0, 0
	for _, p := range got {
		if strings.Contains(p.Message, "the check itself failed") {
			t.Errorf("%s panicked on a bare repository: %s", p.Rule, p.Message)
		}
		switch p.Severity {
		case lint.Error:
			errors++
		case lint.Skip:
			skips++
		}
	}
	if errors == 0 {
		t.Error("a repository with no licence and no documents passed")
	}
	if skips == 0 {
		t.Error("nothing reported a skip, so a run that verified little read as complete")
	}
}

func TestOnlineRulesSkipUnlessAsked(t *testing.T) {
	l := New(config.Default().OSS, linttest.Repo(t, map[string]string{"a.txt": "x\n"}))
	for _, p := range l.Run() {
		if p.Rule == "OSS-801" || p.Rule == "OSS-802" {
			if p.Severity != lint.Skip {
				t.Errorf("%s ran without --online: %v", p.Rule, p)
			}
		}
	}
}

func TestExcerptIsBounded(t *testing.T) {
	// A generated page puts its whole payload on one line, so an unbounded
	// quote would print the page.
	long := strings.Repeat("x", 5000)
	if got := excerpt(long, 2000); len(got) > 200 {
		t.Errorf("excerpt is %d characters", len(got))
	}
	if got := shortExcerpt(long, 2000); len(got) > 100 {
		t.Errorf("shortExcerpt is %d characters", len(got))
	}
}

func TestProjectAndSlugAreInferred(t *testing.T) {
	l := New(config.OSS{}, linttest.Repo(t, map[string]string{
		"Makefile": "BIN := bin/cs-thing\n",
	}))
	if got := l.project(); got != "cs-thing" {
		t.Errorf("project inferred as %q, want cs-thing", got)
	}
	l = New(config.OSS{Project: "override"}, linttest.Repo(t, map[string]string{"a.txt": "x\n"}))
	if got := l.project(); got != "override" {
		t.Errorf("the configured project was ignored: %q", got)
	}
}

// illFormed is a repository that trips as many rules as one fixture can, so a
// full run walks the finding branch of each rather than only the success path.
func illFormed() map[string]string {
	files := wellFormed()
	files["LICENSE"] = "Apache-2.0. See the website.\n"
	files["README.md"] = "Not an H1.\n\nNo badge, no claim, no licence section.\n" +
		"[gone](docs/NOPE.md)\n"
	files["AGENTS.md"] = "# Working here\n\n" + strings.Repeat("A line of knowledge.\n", 45)
	files["CONTRIBUTING.md"] = "# Contributing\n\nFor a security-sensitive issue, please " +
		"ask for a private contact.\n"
	files["Makefile"] = "build:\n\t@true\n"
	// No ignore rule, so the tracked build output below actually reaches the
	// index and the .env rule has something to report.
	files[".gitignore"] = "/dist/\n"
	files[".gitattributes"] = "# nothing about line endings\n"
	files[".github/workflows/ci.yml"] = "name: ci\non:\n  push:\njobs:\n  a:\n    steps:\n" +
		"      - uses: actions/checkout@main\n      - run: echo hi\n"
	delete(files, ".github/workflows/release.yml")
	delete(files, "SPEC.md")
	// One file reports one leak, so the two classes need a file each.
	files["notes.txt"] = "built under /home/jdoe/work\n"
	files["contacts.txt"] = "mail ada@realcompany.co.uk about it\n"
	files["bin/artifact"] = "committed build output\n"
	files[".env"] = "SECRET=1\n"
	return files
}

func TestAFullRunWalksTheFindingBranches(t *testing.T) {
	cfg := config.Default().OSS
	cfg.Project = "thing"
	cfg.GitHubRepo = "acme/thing"
	l := New(cfg, linttest.Repo(t, illFormed()))
	got := l.Run()

	seen := map[string]bool{}
	for _, p := range got {
		if strings.Contains(p.Message, "the check itself failed") {
			t.Errorf("%s panicked: %s", p.Rule, p.Message)
		}
		if p.Severity == lint.Error {
			seen[p.Rule] = true
		}
	}
	// The rules a repository in this shape must fail, one per family, so a
	// silent family is a family whose finding branch never ran.
	for _, id := range []string{
		"OSS-102", // the licence is a summary rather than the text
		"OSS-201", // SPEC.md is missing
		"OSS-206", // the README links a file that is not there
		"OSS-212", // a security report is invited and no channel is named
		"OSS-301", // a home directory naming a person
		"OSS-303", // a mail address
		"OSS-307", // .env is tracked and not ignored
		"OSS-308", // build output is tracked
		"OSS-404", // an action pinned to @main
		"OSS-407", // no release workflow
		"OSS-410", // the task runner is missing most of the vocabulary
		"OSS-411", // check reaches neither linter
	} {
		if !seen[id] {
			t.Errorf("%s did not fire on a repository that violates it", id)
		}
	}
}

func TestVendoredTreeCarriesItsLicence(t *testing.T) {
	files := map[string]string{"vendor/thing/code.go": "package thing\n"}
	if len(run(t, "OSS-105", config.OSS{}, files)) == 0 {
		t.Error("a vendored tree with no licence passed")
	}
	files["vendor/thing/LICENSE"] = "MIT\n"
	if len(run(t, "OSS-105", config.OSS{}, files)) != 0 {
		t.Error("a vendored tree carrying its licence was reported")
	}
}

func TestTwoLicencesLeaveAReaderGuessing(t *testing.T) {
	files := map[string]string{"LICENSE": "a\n", "COPYING": "b\n"}
	if len(run(t, "OSS-106", config.OSS{}, files)) == 0 {
		t.Error("two root licence files passed")
	}
}

func TestStrayRootDocumentIsReported(t *testing.T) {
	files := map[string]string{"NOTES.md": "# notes\n"}
	if len(run(t, "OSS-210", config.OSS{}, files)) == 0 {
		t.Error("a root document outside the set passed")
	}
}

func TestManifestPathsMustExist(t *testing.T) {
	files := map[string]string{
		".goreleaser.yaml": "files:\n  - deploy/gone.yaml\n  - README.md\n",
		"README.md":        "# x\n",
	}
	got := run(t, "OSS-211", config.OSS{}, files)
	if len(got) != 1 || !strings.Contains(got[0].Message, "deploy/gone.yaml") {
		t.Errorf("got %v, want one finding naming the path that is not there", got)
	}
}

func TestLocalDependencyIsReported(t *testing.T) {
	files := map[string]string{"go.mod": "module x\n\nreplace a => ../elsewhere\n"}
	if len(run(t, "OSS-501", config.OSS{}, files)) == 0 {
		t.Error("a module replaced by a path outside the repository passed")
	}
}

func TestModulePathMatchesTheRemote(t *testing.T) {
	files := map[string]string{"go.mod": "module github.com/acme/other\n"}
	got := run(t, "OSS-503", config.OSS{GitHubRepo: "acme/thing"}, files)
	if firstError(got) == nil {
		t.Error("a module path that is not where it lives passed")
	}
	files["go.mod"] = "module github.com/acme/thing\n"
	if firstError(run(t, "OSS-503", config.OSS{GitHubRepo: "acme/thing"}, files)) != nil {
		t.Error("a module path that matches the remote was reported")
	}
}

func TestLedgerRulesSkipWithoutALedger(t *testing.T) {
	files := map[string]string{"a.txt": "x\n"}
	for _, id := range []string{"OSS-601", "OSS-602", "OSS-603", "OSS-604"} {
		got := run(t, id, config.OSS{}, files)
		if len(got) != 1 || got[0].Severity != lint.Skip {
			t.Errorf("%s on a repository with no ledger gave %v", id, got)
		}
	}
}

func TestSessionTrailerInTheHistoryIsReported(t *testing.T) {
	repo := linttest.Repo(t, map[string]string{"a.txt": "x\n"})
	cmd := exec.Command("git", "commit", "--allow-empty", "-qm",
		"Add a thing\n\nClaude-Session: https://claude.ai/code/session_x")
	cmd.Dir = repo.Root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	l := New(config.Default().OSS, repo)
	for _, r := range rules {
		if r.id == "OSS-701" {
			got := l.runOne(r)
			if firstError(got) == nil {
				t.Errorf("a session trailer in the history passed: %v", got)
			}
			return
		}
	}
}

func TestDeclaredSkipsApplyToTheHistoryToo(t *testing.T) {
	// A waiver says "this path is not evidence". Honouring it in the tree scan
	// and not the history scan leaves a repository clean on one and red on the
	// other, for the same file and the same declared reason.
	repo := linttest.Repo(t, map[string]string{
		"fixtures/sample.txt": "the run wrote to /home/jdoe/work\n",
		"README.md":           "# x\n",
	})
	cfg := config.Default().OSS
	cfg.SkipPaths = map[string]string{"fixtures/": "captured upstream, scrubbed by make scrub"}
	l := New(cfg, repo)
	for _, r := range rules {
		if r.id != "OSS-708" {
			continue
		}
		if got := l.runOne(r); firstError(got) != nil {
			t.Errorf("a declared skip still reported from the history: %v", got)
		}
		// And without the waiver the same history does report.
		bare := New(config.Default().OSS, repo)
		if got := bare.runOne(r); firstError(got) == nil {
			t.Error("the history scan found nothing with no waiver in place")
		}
		return
	}
	t.Fatal("no OSS-708 rule")
}

func TestScansWithNothingToReadReportASkip(t *testing.T) {
	// R4: a check that could not run reports a skip rather than a pass. A leak
	// scan that inspected zero files must never be indistinguishable from one
	// that inspected them all and found nothing.
	l := New(config.Default().OSS, linttest.Tree(t, map[string]string{
		"notes.txt": "the run wrote to /home/jdoe/work and mailed ada@realcompany.co.uk\n",
	}))
	got := l.Run()

	reported := map[string]lint.Severity{}
	for _, p := range got {
		if _, seen := reported[p.Rule]; !seen || p.Severity == lint.Skip {
			reported[p.Rule] = p.Severity
		}
	}
	// Every rule that reads the tracked tree, including the leak scan that has
	// a real leak sitting in front of it on disk.
	for _, id := range []string{
		"OSS-105", "OSS-106", "OSS-206", "OSS-207", "OSS-210",
		"OSS-301", "OSS-302", "OSS-303", "OSS-304", "OSS-305",
		"OSS-306", "OSS-307", "OSS-308", "OSS-501", "OSS-502",
	} {
		sev, seen := reported[id]
		if !seen {
			t.Errorf("%s reported nothing at all, so a run that read no file "+
				"reads as a run that found none", id)
			continue
		}
		if sev != lint.Skip {
			t.Errorf("%s reported %s, want a skip when there is nothing to read", id, sev)
		}
	}
}

func TestTheIgnoreHalfStillRunsWithNothingTracked(t *testing.T) {
	// OSS-307 has two halves and only one needs the tracked list. The ignore
	// file is read from disk, so it is still checked.
	l := New(config.Default().OSS, linttest.Tree(t, map[string]string{".gitignore": "/bin/\n"}))
	var skips, errors int
	for _, p := range l.Run() {
		if p.Rule != "OSS-307" {
			continue
		}
		switch p.Severity {
		case lint.Skip:
			skips++
		case lint.Error:
			errors++
		}
	}
	if skips != 1 {
		t.Errorf("the half that could not run reported %d skips, want 1", skips)
	}
	if errors != 1 {
		t.Errorf("the half that could run reported %d errors, want 1 (.env not ignored)", errors)
	}
}

func TestAnEmptyRepositoryAlsoSkips(t *testing.T) {
	// A repository git answers for, with nothing committed, reads nothing just
	// as a plain directory does.
	repo := linttest.Repo(t, map[string]string{"a.txt": "x\n"})
	if _, err := repo.Git("rm", "-q", "--cached", "a.txt"); err != nil {
		t.Fatal(err)
	}
	l := New(config.Default().OSS, lint.NewRepo(repo.Root))
	for _, p := range l.Run() {
		if p.Rule == "OSS-301" && p.Severity != lint.Skip {
			t.Errorf("an empty repository gave OSS-301 %s, want a skip", p.Severity)
		}
	}
}

func TestScansStillRunWhenThereIsSomethingToRead(t *testing.T) {
	// The guard must not swallow a real finding.
	l := New(config.Default().OSS, linttest.Repo(t, map[string]string{
		"notes.txt": "the run wrote to /home/jdoe/work\n",
	}))
	for _, p := range l.Run() {
		if p.Rule == "OSS-301" {
			if p.Severity != lint.Error {
				t.Errorf("a real leak reported %s, want an error", p.Severity)
			}
			return
		}
	}
	t.Error("OSS-301 reported nothing against a tracked file carrying a leak")
}
