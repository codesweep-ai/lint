package oss

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/codesweep-ai/lint/internal/lint"
)

var (
	sessionTrailer = regexp.MustCompile(`(?im)^(?:Claude-Session|Session|Codex-Session|Transcript|` +
		`Agent-Session):|claude\.ai/code/session|chatgpt\.com/codex/|Generated with \[`)

	// privateBranch names a branch that says "this is the history I removed".
	// Not anchored: the branch that held one project's de-shipped design
	// document was called go-port-backup-blueprint, and a leading-anchor
	// pattern walked past it.
	privateBranch = regexp.MustCompile(`(?i)(?:^|[-_/])(?:backup|bak|wip|old|tmp|temp|scratch|orig|snapshot)` +
		`(?:[-_/]|\d|$)|^pre-`)
)

// historySeverity is Error until the repository is public. Published history
// cannot be rewritten, so from that point the rule is advice.
func (l *Linter) historySeverity() lint.Severity {
	if l.cfg.Published {
		return lint.Warn
	}
	return lint.Error
}

// historyProbes are the high-signal half of the leak patterns, in the syntax
// `git log -G` speaks. A candidate commit is re-read with the confirming
// pattern, because that syntax cannot say "any name except the placeholder".
var historyProbes = []struct {
	probe   string
	confirm *regexp.Regexp
	what    string
}{
	// Any home directory, not only one leading into a dot-directory.
	// Requiring the dot was a real hole: a test constant naming a developer's
	// own home, with an ordinary path after it, sat in eighty commits and
	// every history probe passed it.
	{`home/[A-Za-z0-9_.-]+/`,
		regexp.MustCompile(`(?m)(?:^|[^A-Za-z0-9_.])/?(?:home|Users)/([A-Za-z0-9_-][A-Za-z0-9_.-]*)/`),
		"a home directory naming a person"},
	{`home/[A-Za-z0-9_.-]+/\.(cache|config|local|ssh|aws|gnupg)`,
		regexp.MustCompile(`(?:home|Users)/([A-Za-z0-9_.-]+)/\.(?:cache|config|local|ssh|aws|gnupg)`),
		"a path into a person's own state"},
	{`-----BEGIN [A-Z ]*PRIVATE KEY-----`,
		regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`), "a private key"},
	{`sk-ant-[A-Za-z0-9_-]{16,}`, regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{16,}`), "an Anthropic key"},
	{`gh[pousr]_[A-Za-z0-9]{30,}`, regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{30,}`), "a GitHub token"},
	{`AKIA[0-9A-Z]{16}`, regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "an AWS access key id"},
}

// historyHit reports whether a match found in a past diff is a real leak.
//
// A capturing group means the pattern needs the name it caught checked against
// the placeholders this project ships. Everything else is judged by whether
// the value says of itself that it is a fixture.
func (l *Linter) historyHit(m []string) bool {
	if len(m) > 1 {
		return !allowed(m[1], l.cfg.HomeAllow)
	}
	return !fakeMarkers.MatchString(m[0])
}

// excludePaths turns the declared skips into git pathspecs, so a path the
// tree scan is told to ignore is ignored by the history scan too.
//
// A waiver says "this path is not evidence". Honouring it in one scan and not
// the other leaves a repository that reports clean on its tree and red on its
// history, for the same file and the same declared reason.
func (l *Linter) excludePaths() []string {
	if len(l.cfg.SkipPaths) == 0 {
		return nil
	}
	out := []string{"--", "."}
	for _, prefix := range lint.SortedKeys(l.cfg.SkipPaths) {
		out = append(out, ":(exclude)"+prefix)
	}
	return out
}

var historyRules = []rule{{
	id: "OSS-701", severity: lint.Error,
	title: "No commit message links an agent session",
	why: "A session link is private to whoever ran it and dead to everyone else. It is " +
		"also the one part of a commit message that cannot be fixed after the repository " +
		"is public.",
	check: func(l *Linter) []lint.Problem {
		log, err := l.repo.Git("log", "--format=%H%x00%B%x00%x00")
		if err != nil {
			return []lint.Problem{lint.Skipf("OSS-701", "no git history")}
		}
		var hits []string
		for entry := range strings.SplitSeq(log, "\x00\x00") {
			sha, message, found := strings.Cut(entry, "\x00")
			if !found {
				continue
			}
			if sessionTrailer.MatchString(message) {
				hits = append(hits, shortSHA(sha))
			}
		}
		if len(hits) == 0 {
			return nil
		}
		return []lint.Problem{{
			Rule:     "OSS-701",
			Severity: l.historySeverity(),
			Message: fmt.Sprintf("%d commit message(s) link an agent session; the newest is %s",
				len(hits), hits[0]),
			Where: hits[0],
		}}
	},
}, {
	id: "OSS-702", severity: lint.Warn,
	title: "Commit subjects read the way CONTRIBUTING says",
	why: "The history is published too, and it is the first thing a reader who wants to " +
		"trust the project scrolls through.",
	check: func(l *Linter) []lint.Problem {
		log, err := l.repo.Git("log", "--format=%h %s")
		if err != nil {
			return []lint.Problem{lint.Skipf("OSS-702", "no git history")}
		}
		var lines []string
		for l := range strings.SplitSeq(log, "\n") {
			if strings.TrimSpace(l) != "" {
				lines = append(lines, l)
			}
		}
		var badCase, tooLong, trailing []string
		for _, line := range lines {
			sha, subject, _ := strings.Cut(line, " ")
			if subject == "" {
				continue
			}
			if r := rune(subject[0]); r >= 'a' && r <= 'z' {
				badCase = append(badCase, sha)
			}
			if len(subject) > 60 {
				tooLong = append(tooLong, sha)
			}
			if strings.HasSuffix(subject, ".") {
				trailing = append(trailing, sha)
			}
		}
		var out []lint.Problem
		for _, g := range []struct {
			label string
			hits  []string
		}{
			{"open in lower case", badCase},
			{"are over 60 characters", tooLong},
			{"end with a full stop", trailing},
		} {
			if len(g.hits) > 0 {
				out = append(out, lint.Warnf("OSS-702", "%d of %d commit subjects %s",
					len(g.hits), len(lines), g.label).At(g.hits[0]))
			}
		}
		return out
	},
}, {
	id: "OSS-703", severity: lint.Warn,
	title: "No branch was kept as a private backup",
	why: "A branch named for a rewrite that has already happened carries the history the " +
		"rewrite removed. Publishing the repository publishes it.",
	check: func(l *Linter) []lint.Problem {
		listing, err := l.repo.Git("branch", "--format=%(refname:short)")
		if err != nil {
			return []lint.Problem{lint.Skipf("OSS-703", "no git history")}
		}
		var suspect []string
		for b := range strings.SplitSeq(listing, "\n") {
			if b = strings.TrimSpace(b); b != "" && privateBranch.MatchString(b) {
				suspect = append(suspect, b)
			}
		}
		if len(suspect) > 0 {
			return []lint.Problem{lint.Warnf("OSS-703",
				"local branches that must not be pushed: %s", strings.Join(lint.First(suspect, 8), ", "))}
		}
		return nil
	},
}, {
	id: "OSS-704", severity: lint.Warn,
	title: "Every address the history publishes is meant to be public",
	why: "A Co-Authored-By trailer publishes an address. A machine identity is fine; a " +
		"person's is theirs to publish, not yours.",
	check: func(l *Linter) []lint.Problem {
		log, err := l.repo.Git("log", "--format=%B")
		if err != nil {
			return []lint.Problem{lint.Skipf("OSS-704", "no git history")}
		}
		mailAllow := append(append([]string(nil), reservedDomains...), l.cfg.EmailAllow...)
		found := map[string]bool{}
		for _, m := range mailAddr.FindAllStringSubmatch(log, -1) {
			if allowed(m[1], mailAllow) || allowed(m[1], []string{"noreply"}) ||
				strings.HasPrefix(m[0], "noreply@") {
				continue
			}
			found[m[0]] = true
		}
		if len(found) > 0 {
			return []lint.Problem{lint.Warnf("OSS-704",
				"the history publishes these addresses; confirm each is meant to be public: %s",
				strings.Join(lint.First(lint.SortedKeys(found), 6), ", "))}
		}
		return nil
	},
}, {
	id: "OSS-705", severity: lint.Warn,
	title: "The working tree is clean",
	why:   "A check run against a dirty tree reports on something that is not what would be published.",
	check: func(l *Linter) []lint.Problem {
		status, err := l.repo.Git("status", "--porcelain")
		if err != nil {
			return []lint.Problem{lint.Skipf("OSS-705", "no git repository")}
		}
		var dirty []string
		for line := range strings.SplitSeq(status, "\n") {
			if strings.TrimSpace(line) != "" {
				dirty = append(dirty, strings.TrimSpace(line))
			}
		}
		if len(dirty) > 0 {
			return []lint.Problem{lint.Warnf("OSS-705",
				"%d uncommitted change(s); the first is %s", len(dirty), dirty[0])}
		}
		return nil
	},
}, {
	id: "OSS-706", severity: lint.Error,
	title: "No rewrite left its backup behind",
	why: "`git filter-branch` saves the history it rewrote under refs/original. That ref " +
		"still reaches every commit the rewrite was meant to remove, and `git push " +
		"--mirror` publishes it.",
	check: func(l *Linter) []lint.Problem {
		listing, err := l.repo.Git("for-each-ref", "--format=%(refname)", "refs/original/")
		if err != nil {
			return []lint.Problem{lint.Skipf("OSS-706", "no git repository")}
		}
		var refs []string
		for r := range strings.SplitSeq(listing, "\n") {
			if r = strings.TrimSpace(r); r != "" {
				refs = append(refs, r)
			}
		}
		if len(refs) > 0 {
			return []lint.Problem{lint.Errorf("OSS-706",
				"a filter-branch backup survives: %s. Delete it with `git update-ref -d`, "+
					"then expire the reflog and gc", strings.Join(lint.First(refs, 4), ", "))}
		}
		return nil
	},
}, {
	id: "OSS-708", severity: lint.Error,
	title: "No commit in the history carries a leak",
	why: "Publishing a repository publishes every commit in it. A path or a key that was " +
		"removed later is still in the blob the earlier commit points at, and a clone taken " +
		"on the first day keeps it. This reads the text of each diff, so a leak inside a " +
		"binary blob that was later deleted is past it: for those, check what `git log " +
		"--diff-filter=A --name-only --all` ever added.",
	check: func(l *Linter) []lint.Problem {
		if _, err := l.repo.Git("rev-parse", "HEAD"); err != nil {
			return []lint.Problem{lint.Skipf("OSS-708", "no git history")}
		}
		type probe struct {
			pattern string
			confirm *regexp.Regexp
			what    string
		}
		probes := make([]probe, 0, len(historyProbes)+2)
		for _, p := range historyProbes {
			probes = append(probes, probe{p.probe, p.confirm, p.what})
		}
		names := map[string]bool{}
		for _, n := range []string{os.Getenv("USER"), os.Getenv("LOGNAME")} {
			if len(n) > 2 && !allowed(n, l.cfg.HomeAllow) {
				names[n] = true
			}
		}
		for _, n := range lint.SortedKeys(names) {
			probes = append(probes, probe{regexp.QuoteMeta(n), nil, "the invoking user's own name"})
		}
		var out []lint.Problem
		for _, p := range probes {
			args := append([]string{"log", "--all", "--format=%h", "-G" + p.pattern},
				l.excludePaths()...)
			found, err := l.repo.Git(args...)
			if err != nil || strings.TrimSpace(found) == "" {
				continue
			}
			shas := lint.First(strings.Fields(found), 80)
			var hits []string
			for _, sha := range shas {
				if p.confirm == nil {
					hits = append(hits, sha)
					continue
				}
				diff, _ := l.repo.Git(append([]string{"show", "--format=", sha},
					l.excludePaths()...)...)
				if slices.ContainsFunc(p.confirm.FindAllStringSubmatch(diff, -1), l.historyHit) {
					hits = append(hits, sha)
				}
			}
			if len(hits) > 0 {
				out = append(out, lint.Problem{
					Rule:     "OSS-708",
					Severity: l.historySeverity(),
					Message: fmt.Sprintf("%s is in %d commit(s), the newest %s; rewriting is "+
						"possible now and not after publication", p.what, len(hits), hits[0]),
					Where: hits[0],
				})
			}
		}
		return out
	},
}}

func shortSHA(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 9 {
		return sha[:9]
	}
	return sha
}
