package oss

import (
	"regexp"
	"strings"

	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/lint"
)

var localDeps = []struct{ pattern, what string }{
	{`"[^"]*"\s*:\s*"file:\.\.[^"]*"`, "an npm dependency resolved from a path outside the repository"},
	{`"[^"]*"\s*:\s*"link:\.\.[^"]*"`, "an npm dependency linked from outside the repository"},
	{`(?m)^\s*replace\s+\S+\s+=>\s+\.\.`, "a Go module replaced by a path outside the repository"},
	{`path\s*=\s*"\.\.`, "a Cargo dependency resolved from a path outside the repository"},
	{`-e\s+\.\./`, "a Python dependency installed from a path outside the repository"},
}

var (
	// climbsBack is the tail of a `../`-prefixed path. A dependency that
	// climbs out of its directory and back into this tree is in the tree,
	// whatever the path looks like.
	climbsBack = regexp.MustCompile(`(?:\.\./)+([\w.-]+(?:/[\w.-]+)*)`)
	sibling    = regexp.MustCompile(`((?:\.\./)+)([\w.-]+(?:/[\w.-]+)*)/?`)
	modulePath = regexp.MustCompile(`(?m)^module\s+(\S+)`)

	localDepsBuilt []struct {
		re   *regexp.Regexp
		what string
	}
)

func init() {
	for _, d := range localDeps {
		localDepsBuilt = append(localDepsBuilt, struct {
			re   *regexp.Regexp
			what string
		}{regexp.MustCompile(d.pattern), d.what})
	}
}

// escapesTheRepo reports whether a `../`-prefixed path leaves the repository.
//
// Judged by the tail rather than by the base, because the base a comment or a
// script means is not always the file it sits in. A tail that names something
// in the tree is a path back into it, however many levels it climbs first:
// `../../vendor/thing` from apps/viewer is this repository, and `../../thing`
// is not.
func (l *Linter) escapesTheRepo(tail string) bool { return !l.repo.Exists(tail) }

var cloneRules = []rule{{
	id: "OSS-501", severity: lint.Error, needsTree: true,
	title: "Nothing resolves from outside the repository",
	why: "A dependency read from a sibling directory builds on the machine it was written " +
		"on and nowhere else. A stranger's clone is one directory, and the failure names a " +
		"path they have never seen.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		l.scannable(func(path, body string) {
			if !hasAnySuffix(path, ".json", ".mod", ".toml", ".txt", ".yaml", ".yml") {
				return
			}
			if strings.Contains(path, "lock") {
				return
			}
			for _, d := range localDepsBuilt {
				for _, m := range d.re.FindAllStringIndex(body, -1) {
					if tail := climbsBack.FindStringSubmatch(body[m[0]:m[1]]); len(tail) > 1 &&
						l.repo.Exists(tail[1]) {
						continue
					}
					out = append(out, lint.Errorf("OSS-501", "%s: %s", d.what,
						narrowExcerpt(body, m[0])).At(lint.At(path, body, m[0])))
				}
			}
		})
		if len(out) > 20 {
			out = out[:20]
		}
		return out
	},
}, {
	id: "OSS-502", severity: lint.Error, needsTree: true,
	title: "No gate needs a checkout beside this one",
	why: "A build that requires a second repository as a sibling fails for reasons " +
		"unrelated to the change, and the message names a directory the reader does not have.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		l.scannable(func(path, body string) {
			if !strings.HasPrefix(path, "scripts/") && path != "Makefile" &&
				path != "makefile" && !strings.HasPrefix(path, ".github/workflows/") {
				return
			}
			for _, m := range sibling.FindAllStringSubmatchIndex(body, -1) {
				// The original bounded the match with a lookbehind: a `../`
				// that continues a word, a path or a shell expansion is not
				// the start of one.
				if m[0] > 0 && strings.ContainsRune(`abcdefghijklmnopqrstuvwxyz`+
					`ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_./$({`, rune(body[m[0]-1])) {
					continue
				}
				if !l.escapesTheRepo(body[m[4]:m[5]]) {
					continue
				}
				out = append(out, lint.Errorf("OSS-502", "a path outside the repository: %s",
					midExcerpt(body, m[0])).At(lint.At(path, body, m[0])))
				return
			}
		})
		return out
	},
}, {
	id: "OSS-503", severity: lint.Error,
	title: "The module path is the published one",
	why: "A module whose declared path is not where it lives cannot be installed with " +
		"`go install`, and the error names a repository that does not exist.",
	check: func(l *Linter) []lint.Problem {
		gomod, ok := l.read("go.mod")
		slug := l.slug()
		if !ok || slug == "" {
			return []lint.Problem{lint.Skipf("OSS-503", "no go.mod, or no origin remote")}
		}
		m := modulePath.FindStringSubmatch(gomod)
		if m == nil {
			return []lint.Problem{lint.Errorf("OSS-503", "go.mod declares no module path")}
		}
		if !strings.HasSuffix(strings.ToLower(m[1]), strings.ToLower(slug)) {
			return []lint.Problem{lint.Errorf("OSS-503",
				"go.mod declares %s, and the remote is %s", m[1], slug)}
		}
		return nil
	},
}, {
	id: "OSS-504", severity: lint.Error,
	title: "The prose gate is tuned",
	why: "OSS-411 checks that the gate is wired in. This checks that it verifies " +
		"anything once it runs: an empty glossary disables the most valuable prose " +
		"check, which leaves the target passing while it reads almost nothing. The " +
		"terms a reader of a project cannot infer are the project's own, so no default " +
		"can supply them.",
	check: func(l *Linter) []lint.Problem {
		cfg, err := config.Load(l.repo.Root)
		if err != nil {
			return []lint.Problem{lint.Errorf("OSS-504", "%s does not parse: %v", config.Name, err)}
		}
		if len(cfg.Docs.Prose.Glossary) == 0 {
			return []lint.Problem{lint.Errorf("OSS-504",
				"docs.prose.glossary in %s is empty, which disables the most valuable prose check",
				config.Name)}
		}
		return nil
	},
}}

func hasAnySuffix(s string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}

func window(text string, index, before, after int) string {
	start := index - before
	lead := "…"
	if start <= 0 {
		start, lead = 0, ""
	}
	end := min(index+after, len(text))
	return lead + strings.Join(strings.Fields(text[start:end]), " ") + "…"
}

func narrowExcerpt(text string, index int) string { return window(text, index, 10, 70) }
func midExcerpt(text string, index int) string    { return window(text, index, 30, 60) }
