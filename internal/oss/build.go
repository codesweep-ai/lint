package oss

import (
	"regexp"
	"slices"
	"strings"

	"github.com/codesweep-ai/lint/internal/lint"
)

var (
	actionRef      = regexp.MustCompile(`(?m)^\s*(?:-\s*)?uses:\s*([^\s#]+)`)
	pinnedVersion  = regexp.MustCompile(`^(?:v?\d[\w.+-]*|[0-9a-f]{40})$`)
	permissionsTop = regexp.MustCompile(`(?m)^permissions:`)
	makeTarget     = regexp.MustCompile(`(?m)^([a-zA-Z][\w.-]*):`)
	runsCheck      = regexp.MustCompile(`\bmake check\b|\bnpm (run )?check\b|scripts/check`)
	secretRead     = regexp.MustCompile(`secrets\.([A-Z_][A-Z0-9_]*)`)
	versionTag     = regexp.MustCompile(`^v\d+\.\d+\.\d+[\w.+-]*$`)
	goDeclared     = regexp.MustCompile(`(?m)^go\s+(\d+)\.(\d+)`)
	// The floor a page states, however it words it. Three repositories wrote
	// "Go 1.26 or newer" and went on claiming a version they had moved off,
	// because the pattern read only the `+` spelling.
	goClaimed      = regexp.MustCompile(`Go\s+(\d+)\.(\d+)(?:\+|\s+or\s+(?:newer|later|above))`)
	checkLine      = regexp.MustCompile(`(?m)^check:[^\S\n]*(.*)$`)
	delegateScript = regexp.MustCompile(`([\w./-]*check\.sh)`)
	releaseTagGlob = regexp.MustCompile(`tags:.*\bv\*`)

	// The triggers a workflow declares, and the two jobs a gate needs, each
	// compiled once rather than on every run.
	ciTrigger = map[string]*regexp.Regexp{
		"pull_request":      regexp.MustCompile(`(?m)^\s{2}pull_request:`),
		"workflow_dispatch": regexp.MustCompile(`(?m)^\s{2}workflow_dispatch:`),
	}
	proseJob      = regexp.MustCompile(`(?m)^\s{2}prose:`)
	refsJob       = regexp.MustCompile(`(?m)^\s{2}refs:`)
	ossJob        = regexp.MustCompile(`(?m)^\s{2}oss:`)
	writesRelease = regexp.MustCompile(`contents:\s*write`)
)

// Each of these is what a recipe may name to reach one linter. The vendored
// Python script is still recognised, because a repository that has not moved
// to the binary yet is gated all the same.
var (
	proseGate     = []string{"cs-lint prose", "lint-docs"}
	refsGate      = []string{"cs-lint refs"}
	readinessGate = []string{"cs-lint oss", "lint-oss"}
)

func (l *Linter) releaseWorkflow() (string, bool) {
	if b, ok := l.read(".github/workflows/release.yml"); ok {
		return b, true
	}
	return l.read(".github/workflows/release.yaml")
}

// continued matches the backslash a Makefile line ends with to carry on.
var continued = regexp.MustCompile(`\\\n[^\S\n]*`)

// unfold joins the lines a backslash continues, so a prerequisite list written
// across two lines reads as the one list it is. Every reader below wants the
// logical line rather than the physical one.
func unfold(body string) string { return continued.ReplaceAllString(body, " ") }

// checkTarget returns the check target's prerequisites and its recipe.
func checkTarget(body string) (prereqs []string, recipe string, ok bool) {
	body = unfold(body)
	// The pattern stops at the newline rather than crossing it: a greedy \s
	// would swallow the tab-indented recipe, and a delegating target would
	// then read as its own prerequisite.
	m := checkLine.FindStringSubmatchIndex(body)
	if m == nil {
		return nil, "", false
	}
	prereqs = strings.Fields(body[m[2]:m[3]])
	rest := strings.TrimLeft(body[m[1]:], "\n")
	var b strings.Builder
	for line := range strings.SplitSeq(rest, "\n") {
		if line != "" && !isSpace(line[0]) {
			break
		}
		b.WriteString(line + "\n")
	}
	return prereqs, b.String(), true
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\r' }

// reaches reports whether check runs a target, directly or through a
// delegating script.
func (l *Linter) reaches(prereqs []string, recipe, target string, needles []string) bool {
	if slices.Contains(prereqs, target) {
		return true
	}
	for _, needle := range needles {
		if strings.Contains(recipe, needle) {
			return true
		}
	}
	// A project that routes its gates through one script satisfies this by
	// calling the tool there instead.
	if m := delegateScript.FindStringSubmatch(recipe); m != nil {
		delegated, _ := l.read(strings.TrimPrefix(m[1], "./"))
		if strings.Contains(delegated, "make "+target) {
			return true
		}
		for _, needle := range needles {
			if strings.Contains(delegated, needle) {
				return true
			}
		}
	}
	return false
}

func containsAny(body string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(body, n) {
			return true
		}
	}
	return false
}

var buildRules = []rule{{
	id: "OSS-401", severity: lint.Error,
	title: "There is a CI workflow",
	why: "An outside contributor cannot run the host-specific half of the suite. CI is " +
		"what tells them their change is sound, and what tells you a merge is safe.",
	check: func(l *Linter) []lint.Problem {
		if len(l.workflows()) > 0 {
			return nil
		}
		return []lint.Problem{lint.Errorf("OSS-401", "no workflow under .github/workflows/")}
	},
}, {
	id: "OSS-402", severity: lint.Warn,
	title: "CI runs on the default branch, on every pull request, and on demand",
	why: "A workflow that only runs on push never sees a fork's pull request, which is " +
		"every outside contribution.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.ci()
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-402", "no ci.yml")}
		}
		var out []lint.Problem
		for _, trigger := range []string{"pull_request", "workflow_dispatch"} {
			if !ciTrigger[trigger].MatchString(body) {
				out = append(out, lint.Warnf("OSS-402", "ci.yml has no %s trigger", trigger))
			}
		}
		return out
	},
}, {
	id: "OSS-403", severity: lint.Error,
	title: "Every workflow declares the permissions it needs",
	why: "Without a permissions block a workflow gets the repository default, which on " +
		"an older repository is write access to everything. A public repository runs " +
		"workflow code proposed by strangers.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		wf := l.workflows()
		for _, path := range lint.SortedKeys(wf) {
			if !permissionsTop.MatchString(wf[path]) {
				out = append(out, lint.Errorf("OSS-403", "no top-level permissions block").At(path))
			}
		}
		return out
	},
}, {
	id: "OSS-404", severity: lint.Error,
	title: "Every action is pinned to a version",
	why: "`@main` means the workflow runs whatever that repository holds today. The " +
		"build that passed yesterday is not the build that runs now.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		wf := l.workflows()
		for _, path := range lint.SortedKeys(wf) {
			for _, m := range actionRef.FindAllStringSubmatch(wf[path], -1) {
				ref := m[1]
				if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "docker://") {
					continue
				}
				_, version, found := strings.Cut(ref, "@")
				switch {
				case !found || version == "":
					out = append(out, lint.Errorf("OSS-404", "%s has no version", ref).At(path))
				case !pinnedVersion.MatchString(version):
					out = append(out, lint.Errorf("OSS-404", "%s is not pinned to a version", ref).At(path))
				}
			}
		}
		return out
	},
}, {
	id: "OSS-405", severity: lint.Error,
	title: "CI runs the same gate a contributor runs",
	why: "Two lists of checks drift, and the one that drifts is always the one nobody " +
		"runs locally.",
	check: func(l *Linter) []lint.Problem {
		bodies := l.allWorkflows()
		if bodies == "" {
			return []lint.Problem{lint.Skipf("OSS-405", "no workflows")}
		}
		if runsCheck.MatchString(bodies) {
			return nil
		}
		return []lint.Problem{lint.Errorf("OSS-405", "no CI job runs the project's own check target")}
	},
}, {
	id: "OSS-406", severity: lint.Error,
	title: "Each linter has its own CI job",
	why: "A linter needs no toolchain and answers in seconds, so on its own job it " +
		"reports even when the tests are failing. Buried behind a test matrix it reports " +
		"nothing. The readiness job also needs the whole history, which the default " +
		"shallow checkout does not fetch.",
	check: func(l *Linter) []lint.Problem {
		bodies := l.allWorkflows()
		if bodies == "" {
			return []lint.Problem{lint.Skipf("OSS-406", "no workflows")}
		}
		var out []lint.Problem
		for _, g := range []struct {
			name    string
			job     *regexp.Regexp
			needles []string
		}{
			{"prose", proseJob, proseGate},
			{"refs", refsJob, refsGate},
		} {
			if !g.job.MatchString(bodies) ||
				!(strings.Contains(bodies, "make "+g.name) || containsAny(bodies, g.needles)) {
				out = append(out, lint.Errorf("OSS-406", "no %s job runs the %s linter",
					g.name, g.name))
			}
		}
		hasOSSJob := ossJob.MatchString(bodies) &&
			(strings.Contains(bodies, "make oss") || containsAny(bodies, readinessGate))
		switch {
		case !hasOSSJob:
			out = append(out, lint.Errorf("OSS-406", "no oss job runs the readiness linter"))
		case !strings.Contains(bodies, "fetch-depth: 0"):
			out = append(out, lint.Warnf("OSS-406",
				"the oss job takes a shallow checkout, so the history rules see nothing and pass"))
		}
		return out
	},
}, {
	id: "OSS-407", severity: lint.Error,
	title: "A tagged release builds the artifacts",
	why: "The first thing a reader does is look for a binary. A releases page with " +
		"nothing on it is the install path every new user takes.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.releaseWorkflow()
		if !ok {
			return []lint.Problem{lint.Errorf("OSS-407", "no release workflow under .github/workflows/")}
		}
		if !strings.Contains(body, "tags:") {
			return []lint.Problem{lint.Errorf("OSS-407", "the release workflow is not triggered by a tag")}
		}
		if !writesRelease.MatchString(body) {
			return []lint.Problem{lint.Errorf("OSS-407",
				"the release workflow cannot create a release (no contents: write)")}
		}
		return nil
	},
}, {
	id: "OSS-408", severity: lint.Error,
	title: "The release manifest validates",
	why: "A manifest that names a document the rework deleted breaks the release build " +
		"outright, and schema validation of the manifest does not catch it.",
	check: func(l *Linter) []lint.Problem {
		if _, ok := l.goreleaser(); !ok {
			return []lint.Problem{lint.Skipf("OSS-408", "no goreleaser manifest")}
		}
		if !lint.Have("goreleaser") {
			return []lint.Problem{lint.Skipf("OSS-408", "goreleaser is not installed")}
		}
		out, ok := l.repo.Run("goreleaser", "check")
		if !ok {
			return []lint.Problem{lint.Errorf("OSS-408", "goreleaser check failed: %s", lastLine(out))}
		}
		return nil
	},
}, {
	id: "OSS-409", severity: lint.Warn,
	title: "The release is reproducible and verifiable",
	why: "A downloaded binary is trusted on the strength of what shipped beside it: a " +
		"checksum, a signature over that checksum, and a bill of materials.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.goreleaser()
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-409", "no goreleaser manifest")}
		}
		wants := []struct{ pattern, what string }{
			{`CGO_ENABLED=0`, "the binary is not built static (CGO_ENABLED=0)"},
			{`-trimpath`, "the build does not pass -trimpath, so paths from the build machine reach the binary"},
			{`mod_timestamp`, "no mod_timestamp, so the archive is not reproducible"},
			{`(?m)^checksum:`, "no checksum block"},
			{`(?m)^sboms:`, "no SBOM block"},
			{`(?m)^signs:`, "no signature block"},
		}
		var out []lint.Problem
		for _, w := range wants {
			if !regexp.MustCompile(w.pattern).MatchString(body) {
				out = append(out, lint.Warnf("OSS-409", "%s", w.what))
			}
		}
		return out
	},
}, {
	id: "OSS-410", severity: lint.Error,
	title: "The task runner carries the family's vocabulary",
	why: "Someone who has worked on one of these repositories knows the targets. " +
		"Renaming them costs that knowledge for no gain.",
	check: func(l *Linter) []lint.Problem {
		body := l.makefile()
		if body == "" {
			return []lint.Problem{lint.Skipf("OSS-410", "no Makefile")}
		}
		have := map[string]bool{}
		for _, m := range makeTarget.FindAllStringSubmatch(unfold(body), -1) {
			have[m[1]] = true
		}
		var out []lint.Problem
		if missing := absent(l.cfg.RequiredTargets, have); len(missing) > 0 {
			out = append(out, lint.Errorf("OSS-410", "the Makefile has no %s target",
				strings.Join(missing, ", ")))
		}
		if missing := absent(l.cfg.ExpectedTargets, have); len(missing) > 0 {
			out = append(out, lint.Warnf("OSS-410", "the Makefile has no %s target",
				strings.Join(missing, ", ")))
		}
		if have["help"] && !strings.Contains(body, ".DEFAULT_GOAL") {
			out = append(out, lint.Warnf("OSS-410", "make with no argument does not print the help menu"))
		}
		return out
	},
}, {
	id: "OSS-411", severity: lint.Error,
	title: "The check target reaches every linter that needs no build",
	why: "The one command a contributor runs before pushing is the contract between them " +
		"and CI. A gate it does not reach is one they meet after the fact, in a red build. " +
		"The interface linter is left out because it reads a binary, and not every " +
		"repository builds one.",
	check: func(l *Linter) []lint.Problem {
		body := l.makefile()
		if body == "" {
			return []lint.Problem{lint.Skipf("OSS-411", "no Makefile")}
		}
		prereqs, recipe, ok := checkTarget(body)
		if !ok {
			return []lint.Problem{lint.Errorf("OSS-411", "the Makefile has no check target")}
		}
		var out []lint.Problem
		for _, g := range []struct {
			target  string
			needles []string
		}{{"prose", proseGate}, {"refs", refsGate}, {"oss", readinessGate}} {
			if !l.reaches(prereqs, recipe, g.target, g.needles) {
				out = append(out, lint.Errorf("OSS-411", "check does not reach the %s linter", g.target))
			}
		}
		return out
	},
}, {
	id: "OSS-412", severity: lint.Warn,
	title: "The toolchain floor is declared once",
	why: "A version pinned in the build file and again in CI drifts, and CI is the copy " +
		"that wins while everyone reads the other one.",
	check: func(l *Linter) []lint.Problem {
		bodies := l.allWorkflows()
		if bodies == "" || !l.has("go.mod") {
			return []lint.Problem{lint.Skipf("OSS-412", "not a Go project with workflows")}
		}
		if !strings.Contains(bodies, "setup-go") {
			return []lint.Problem{lint.Skipf("OSS-412", "CI does not set up Go")}
		}
		var out []lint.Problem
		if !strings.Contains(bodies, "go-version-file") {
			out = append(out, lint.Warnf("OSS-412", "CI pins a Go version of its own instead of reading go.mod"))
		}
		gomod, _ := l.read("go.mod")
		install, _ := l.read("INSTALL.md")
		declared := goDeclared.FindStringSubmatch(gomod)
		claimed := goClaimed.FindStringSubmatch(install)
		if declared != nil && claimed != nil &&
			(declared[1] != claimed[1] || declared[2] != claimed[2]) {
			out = append(out, lint.Warnf("OSS-412",
				"go.mod declares Go %s.%s and INSTALL.md claims %s.%s+",
				declared[1], declared[2], claimed[1], claimed[2]))
		}
		return out
	},
}, {
	id: "OSS-413", severity: lint.Warn,
	title: "A release exists to download",
	why: "INSTALL.md tells the reader to grab an archive. Until a version tag has been " +
		"pushed, the first install path every new user takes points at an empty page.",
	check: func(l *Linter) []lint.Problem {
		install, _ := l.read("INSTALL.md")
		if !strings.Contains(strings.ToLower(install), "release") {
			return []lint.Problem{lint.Skipf("OSS-413", "INSTALL.md offers no release download")}
		}
		out, err := l.repo.Git("tag", "--list", "v*")
		if err != nil || len(strings.Fields(out)) == 0 {
			return []lint.Problem{lint.Warnf("OSS-413",
				"no v* tag has been pushed, so the releases page INSTALL.md points at is empty")}
		}
		return nil
	},
}, {
	id: "OSS-414", severity: lint.Warn,
	title: "No gate needs a secret an outside contributor lacks",
	why: "A pull request from a fork gets no secrets. A job that needs one fails for " +
		"every contributor, and the failure looks like their change.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		wf := l.workflows()
		for _, path := range lint.SortedKeys(wf) {
			if strings.Contains(path, "release") {
				continue
			}
			body := wf[path]
			for _, m := range secretRead.FindAllStringSubmatchIndex(body, -1) {
				name := body[m[2]:m[3]]
				if name == "GITHUB_TOKEN" {
					continue
				}
				out = append(out, lint.Warnf("OSS-414", "a job reads secrets.%s", name).
					At(lint.At(path, body, m[0])))
			}
		}
		if len(out) > 10 {
			out = out[:10]
		}
		return out
	},
}, {
	id: "OSS-415", severity: lint.Warn,
	title: "The workflows themselves lint clean",
	why: "A workflow error surfaces as a run that never starts, and the message the forge " +
		"shows for it names a line rather than a cause.",
	check: func(l *Linter) []lint.Problem {
		if len(l.workflows()) == 0 {
			return []lint.Problem{lint.Skipf("OSS-415", "no workflows")}
		}
		if !lint.Have("actionlint") {
			return []lint.Problem{lint.Skipf("OSS-415", "actionlint is not installed")}
		}
		out, ok := l.repo.Run("actionlint")
		if !ok {
			return []lint.Problem{lint.Warnf("OSS-415", "actionlint reports problems: %s", firstLine(out))}
		}
		return nil
	},
}, {
	id: "OSS-416", severity: lint.Error,
	title: "No tag fires the release workflow by accident",
	why: "The release workflow triggers on a tag glob. A tag that matches the glob but is " +
		"not a version cuts a release named after it, or poisons the version stamp `git " +
		"describe` produces.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.releaseWorkflow()
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-416", "no release workflow")}
		}
		if !releaseTagGlob.MatchString(body) {
			return []lint.Problem{lint.Skipf("OSS-416", "the release trigger is not a v* glob")}
		}
		listing, err := l.repo.Git("tag", "--list", "v*")
		if err != nil {
			return []lint.Problem{lint.Skipf("OSS-416", "no git history")}
		}
		var bad []string
		for t := range strings.FieldsSeq(listing) {
			if !versionTag.MatchString(t) {
				bad = append(bad, t)
			}
		}
		if len(bad) > 0 {
			return []lint.Problem{lint.Errorf("OSS-416",
				"tags that match the release trigger but are not versions: %s",
				strings.Join(lint.First(bad, 6), ", "))}
		}
		return nil
	},
}, {
	id: "OSS-418", severity: lint.Error,
	title: "The ci target runs what the CI workflow runs",
	why: "A red build found after pushing is a round trip nobody needed. `make ci` is " +
		"the workflow's job list on one machine, and it is worth only what it still " +
		"covers: a job added to the workflow and not to the target leaves the gate " +
		"reading green on a laptop and red on the forge. Only the targets both sides " +
		"route through make are compared, because a workflow step is arbitrary shell " +
		"and matching one against a recipe would be guessing. A step that needs a " +
		"privileged host is named in ciSkip with the reason, because a gate left out in " +
		"silence is a gate deleted in private. Where there is no workflow there is " +
		"nothing to mirror, and the missing target is advice.",
	check: func(l *Linter) []lint.Problem {
		body := l.makefile()
		if body == "" {
			return []lint.Problem{lint.Skipf("OSS-418", "no Makefile")}
		}
		rules := makeRules(body)
		_, hasCI := rules["ci"]
		workflow, ok := l.ci()
		if !ok {
			if !hasCI {
				return []lint.Problem{lint.Warnf("OSS-418",
					"no ci target, and no workflow for one to mirror")}
			}
			return nil
		}
		wanted := makeCalls(workflow)
		delete(wanted, "ci")
		for t := range l.cfg.CISkip {
			delete(wanted, t)
		}
		if !hasCI {
			return []lint.Problem{lint.Errorf("OSS-418",
				"the workflow runs %d make target(s) and the Makefile has no ci target "+
					"to run them here", len(wanted))}
		}
		if len(wanted) == 0 {
			return []lint.Problem{lint.Skipf("OSS-418",
				"the workflow routes nothing through make, so there is nothing to compare")}
		}
		reached := l.reachedFrom(rules, "ci")
		var missing []string
		for _, t := range lint.SortedKeys(wanted) {
			if !reached[t] {
				missing = append(missing, t)
			}
		}
		if len(missing) == 0 {
			return nil
		}
		return []lint.Problem{lint.Errorf("OSS-418",
			"the workflow runs `make %s`, which the ci target does not reach",
			strings.Join(lint.First(missing, 6), "`, `make "))}
	},
}, {
	id: "OSS-417", severity: lint.Error,
	title: "The language's own static analysis is in the gate",
	why: "gofmt and go vet catch what will not compile or is plainly wrong. They say " +
		"nothing about a stdlib call written out by hand, a parameter nothing reads, or a " +
		"helper that already exists under another name. That residue accumulates " +
		"fastest in generated code. A `lint` target nobody runs is the same as no target: " +
		"it has to hang off `check` and off CI, or it reports only to whoever remembers it exists.",
	check: func(l *Linter) []lint.Problem {
		if !l.has("go.mod") {
			return []lint.Problem{lint.Skipf("OSS-417", "no go.mod; this rule is Go-specific")}
		}
		var out []lint.Problem
		found := slices.ContainsFunc([]string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"}, l.has)
		if !found {
			out = append(out, lint.Errorf("OSS-417",
				"no .golangci.yml, so golangci-lint runs its default set and every linter "+
					"that finds this residue is off"))
		}
		if body := l.makefile(); body != "" {
			prereqs, recipe, ok := checkTarget(body)
			switch {
			case !ok:
				out = append(out, lint.Errorf("OSS-417", "the Makefile has no check target"))
			case !l.reaches(prereqs, recipe, "lint", []string{"golangci-lint"}):
				out = append(out, lint.Errorf("OSS-417", "check does not reach golangci-lint"))
			}
			if !strings.Contains(body, "deadcode") {
				// `unused` is package-scoped: a function whose only caller lives
				// in another package looks used to it, and an exported one
				// nothing calls is invisible. Whole-program reachability is a
				// separate tool.
				out = append(out, lint.Warnf("OSS-417",
					"no deadcode target; golangci-lint's `unused` cannot see across packages"))
			}
		}
		bodies := l.allWorkflows()
		if bodies != "" && !strings.Contains(bodies, "golangci-lint") && !strings.Contains(bodies, "make check") {
			out = append(out, lint.Errorf("OSS-417", "no CI job runs golangci-lint"))
		}
		return out
	},
}}

func (l *Linter) allWorkflows() string {
	wf := l.workflows()
	var b strings.Builder
	for _, path := range lint.SortedKeys(wf) {
		b.WriteString(wf[path])
		b.WriteString("\n")
	}
	return b.String()
}

// makeCall matches a target invoked through make, in a workflow step or in a
// recipe. The flags between are skipped, so `$(MAKE) --no-print-directory
// check` names check.
//
// It has to sit where a command sits: opening a line, or after a pipe, an
// ampersand, a semicolon or a `run:`, with make's own `@` and `-` prefixes
// allowed in between. Without that anchor the word is found in an apt package
// list, where `git make gcc` reads as a target called gcc, and inside every
// comment that mentions a target in passing.
var makeCall = regexp.MustCompile(`(?m)(?:^|[|&;(]|\brun:)[^\S\n]*[@+-]*[^\S\n]*` +
	`(?:\$\(MAKE\)|\$\{MAKE\}|make)[^\S\n]+(?:-\S+[^\S\n]+)*([a-zA-Z][\w.-]*)`)

// shellMakeCall is the same, for a body that is all commands: a recipe, or a
// script a recipe hands the work to. No anchor, because a helper wraps the
// call in this family's scripts, as `run "prose" make prose`.
var shellMakeCall = regexp.MustCompile(
	`(?:\$\(MAKE\)|\$\{MAKE\}|\bmake)[^\S\n]+(?:-\S+[^\S\n]+)*([a-zA-Z][\w.-]*)`)

// makeCalls are the targets a workflow invokes through make.
//
// Only make. A workflow step is arbitrary shell, and a rule that tried to
// match one against a recipe would be guessing at what two spellings of the
// same thing look like. What a project routes through its task runner is the
// half both sides name identically, and it is the half a reader can act on.
func makeCalls(body string) map[string]bool {
	return found(makeCall, body)
}

// shellMakeCalls are the targets a recipe or a script invokes through make.
//
// The looser pattern is deliberate, and so is which side of the comparison it
// reads. A stray match in a workflow invents a gate the project never had. A
// stray match here only forgives one, which is the smaller wrong of the two.
func shellMakeCalls(body string) map[string]bool {
	return found(shellMakeCall, body)
}

func found(re *regexp.Regexp, body string) map[string]bool {
	out := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(body, -1) {
		out[m[1]] = true
	}
	return out
}

// rule is one Makefile entry: what it depends on, and what it runs.
type makeRule struct {
	prereqs []string
	recipe  string
}

// makeRules reads a Makefile into its entries.
//
// A line opening in a tab belongs to the entry above it. A variable
// assignment carries a colon too, so an entry is a name whose colon is not
// followed by an equals sign.
func makeRules(body string) map[string]makeRule {
	body = unfold(body)
	out := map[string]makeRule{}
	name := ""
	var recipe strings.Builder
	flush := func() {
		if name != "" {
			r := out[name]
			r.recipe += recipe.String()
			out[name] = r
		}
		recipe.Reset()
	}
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(line, "\t") {
			recipe.WriteString(line + "\n")
			continue
		}
		m := makeEntry.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		flush()
		name = m[1]
		out[name] = makeRule{prereqs: strings.Fields(m[2])}
	}
	flush()
	return out
}

var makeEntry = regexp.MustCompile(`^([a-zA-Z][\w.-]*):(?:[^=\n](.*))?$`)

// gateScript matches a shell script a recipe hands the work to.
var gateScript = regexp.MustCompile(`((?:scripts|tools|hack)/[\w./-]+\.sh)`)

// reachedFrom returns the targets one target pulls in, directly or through
// another. A prerequisite and a recipe line that calls make mean the same
// thing to a reader: this runs too.
//
// A recipe that hands the work to a script is followed into it. A project that
// routes its gates through one file is gated all the same, and reading only
// the Makefile would report every gate in that file as unreached.
func (l *Linter) reachedFrom(rules map[string]makeRule, from string) map[string]bool {
	seen := map[string]bool{}
	pending := []string{from}
	for len(pending) > 0 {
		t := pending[0]
		pending = pending[1:]
		if seen[t] {
			continue
		}
		seen[t] = true
		r, ok := rules[t]
		if !ok {
			continue
		}
		pending = append(pending, r.prereqs...)
		for child := range shellMakeCalls(r.recipe) {
			pending = append(pending, child)
		}
		for _, m := range gateScript.FindAllStringSubmatch(r.recipe, -1) {
			script, ok := l.read(m[1])
			if !ok {
				continue
			}
			for child := range shellMakeCalls(script) {
				pending = append(pending, child)
			}
		}
	}
	return seen
}

func absent(want []string, have map[string]bool) []string {
	var out []string
	for _, t := range want {
		if !have[t] {
			out = append(out, t)
		}
	}
	return out
}

func lastLine(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return lint.Truncate(lines[len(lines)-1], 200)
}

func firstLine(out string) string {
	return lint.Truncate(strings.Split(strings.TrimSpace(out), "\n")[0], 200)
}
