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

	// A category label opening a subject: a conventional-commit type, one of
	// the words projects reach for instead, or a bracketed tag. All three fail
	// the same way. The category is already in the diff, and the subject is the
	// one line a reader has for what the change does.
	//
	// Matched without regard to case, because `Fix:` is the same label as
	// `fix:`. The label has to be the whole first token and take a colon or a
	// bracket, so a subject that opens with one of these words and goes on to
	// say something is left alone.
	commitPrefix = regexp.MustCompile(`(?i)^(?:\[[^\]]+\]\s*|(?:` +
		`feat|feature|fix|bugfix|hotfix|patch|chore|chores|docs?|style|` +
		`refactor|refactoring|perf|test|tests|build|ci|cd|revert|deps|dep|` +
		`release|merge|wip|misc|cleanup|clean-up|init|security|breaking|` +
		`improvement|enhancement|update|add|remove|rename|typo|nit|minor|major` +
		`)(?:\([^)]*\))?!?:\s)`)

	// A trailer: a key and a value on one line at the foot of a message.
	// Metadata rather than prose, so the length rules do not count it.
	trailerLine = regexp.MustCompile(`^[A-Za-z][A-Za-z-]*:\s`)

	// A body line that narrates the work rather than describing the change.
	narratesProcess = regexp.MustCompile(`(?i)^\s*(?:[-*]\s*)?(?:as (?:requested|discussed|agreed)|` +
		`per (?:the )?(?:review|feedback|request)|this commit |we (?:then|also|now) |` +
		`after (?:investigat|debugg|trying|some)|i (?:tried|noticed|found that)|` +
		`(?:turns out|it turned out)|the user (?:asked|wanted|reported))`)
)

// bodyBullets returns the bullet lines of a commit body, trailers excluded.
func bodyBullets(body string) []string {
	var out []string
	for line := range strings.SplitSeq(body, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
			out = append(out, strings.TrimSpace(t[2:]))
		}
	}
	return out
}

// bodyProse returns a commit body's word and paragraph counts, with trailers
// and code fences left out. Both are shapes rather than prose, and a message
// quoting a stack trace is not the thing this measures.
func bodyProse(body string) (words, paragraphs int) {
	var fenced, inPara bool
	for line := range strings.SplitSeq(body, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			fenced = !fenced
			continue
		}
		if fenced || trailerLine.MatchString(t) {
			continue
		}
		if t == "" {
			inPara = false
			continue
		}
		if !inPara {
			paragraphs++
			inPara = true
		}
		words += len(strings.Fields(t))
	}
	return words, paragraphs
}

// historySeverity is Error until the repository is public. Published history
// cannot be rewritten, so from that point the rule is advice.
func (l *Linter) historySeverity() lint.Severity {
	if l.cfg.Published {
		return lint.Warn
	}
	return lint.Error
}

// readReach asks git which commits no remote in this clone carries.
//
// Three answers, and the middle one is the reason this is not a one-liner. A
// repository with no remote at all has published nothing, so every commit it
// holds is still the author's to rewrite. A repository whose remote-tracking
// refs are there gets the real answer. A repository that has a remote but no
// ref from it cannot say, and a check that guessed there would fail a clone
// for the state of somebody else's fetch.
func (l *Linter) readReach() {
	if l.askedGit {
		return
	}
	l.askedGit = true
	remotes, err := l.repo.Git("remote")
	if err != nil {
		return
	}
	if strings.TrimSpace(remotes) == "" {
		l.canTell = true
		out, err := l.repo.Git("rev-list", "HEAD")
		if err != nil {
			l.canTell = false
			return
		}
		l.loose = strings.Fields(out)
		return
	}
	refs, err := l.repo.Git("for-each-ref", "--format=%(refname)", "refs/remotes")
	if err != nil || strings.TrimSpace(refs) == "" {
		return
	}
	out, err := l.repo.Git("rev-list", "HEAD", "--not", "--remotes")
	if err != nil {
		return
	}
	l.canTell = true
	l.loose = strings.Fields(out)
}

// rewritable reports whether the commit given is still the author's to change:
// no remote in this clone carries it, so an amend or a rebase reaches it and
// nobody else has a copy to be broken.
//
// A short identifier is matched as the prefix it is, because the rules that
// report on commits shorten one for display before they ever compare it.
func (l *Linter) rewritable(sha string) bool {
	l.readReach()
	if !l.canTell {
		return false
	}
	for _, full := range l.loose {
		if strings.HasPrefix(full, sha) {
			return true
		}
	}
	return false
}

// byReach splits commits into the ones still open to an amend and the ones a
// remote already carries.
func (l *Linter) byReach(shas []string) (loose, sent []string) {
	for _, sha := range shas {
		if l.rewritable(sha) {
			loose = append(loose, sha)
		} else {
			sent = append(sent, sha)
		}
	}
	return loose, sent
}

// pastFindings reports one group of commits as up to two findings.
//
// A commit no remote carries costs an amend to fix, so it fails the run
// wherever the repository is in its life. One a remote already carries costs a
// rewrite of every clone somebody else made, and what that is worth is the
// caller's to say: a leak reads differently from a subject over sixty
// characters. The two halves are reported apart because the reader acts on
// them differently, and only one of them can be acted on for free.
func (l *Linter) pastFindings(rule string, carried lint.Severity, shas []string,
	render func(n int, reach string) string) []lint.Problem {
	loose, sent := l.byReach(shas)
	var out []lint.Problem
	if len(loose) > 0 {
		out = append(out, lint.Problem{
			Rule:     rule,
			Severity: lint.Error,
			Message:  render(len(loose), "no remote carries them yet, so an amend or a rebase reaches them"),
			Where:    loose[0],
		})
	}
	if len(sent) > 0 {
		out = append(out, lint.Problem{
			Rule:     rule,
			Severity: carried,
			Message:  render(len(sent), "a remote already carries them"),
			Where:    sent[0],
		})
	}
	return out
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
		return l.pastFindings("OSS-701", l.historySeverity(), hits, func(n int, reach string) string {
			return fmt.Sprintf("%d commit message(s) link an agent session, and %s", n, reach)
		})
	},
}, {
	id: "OSS-702", severity: lint.Error,
	title: "Commit subjects read the way CONTRIBUTING says",
	why: "The history is published too, and it is the first thing a reader who wants to " +
		"trust the project scrolls through. A category label opening the subject fails the " +
		"run rather than warning: it is the one of these that spreads, because the next " +
		"contributor copies the last subject they saw, and it is free to fix before the " +
		"commit is pushed. Publishing does not stop it spreading, so it stays an error " +
		"after publication, where a history that already carries labels waives it with the " +
		"reason. The other three print and pass once a remote has the commit.",
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
		var badCase, tooLong, trailing, prefixed []string
		for _, line := range lines {
			sha, subject, _ := strings.Cut(line, " ")
			if subject == "" {
				continue
			}
			if commitPrefix.MatchString(subject) {
				prefixed = append(prefixed, sha)
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
			out = append(out, l.pastFindings("OSS-702", lint.Warn, g.hits,
				func(n int, reach string) string {
					return fmt.Sprintf("%d of %d commit subjects %s, and %s",
						n, len(lines), g.label, reach)
				})...)
		}
		if len(prefixed) > 0 {
			// An unpushed commit is amended in a second, so this fails the run.
			// It goes on failing after publication, unlike the leak scans over
			// the history: those describe what is already out, and this one
			// describes what the next contributor will copy. A repository
			// whose published history already carries labels waives it.
			out = append(out, lint.Errorf("OSS-702",
				"%d of %d commit subjects open with a category label, which names a "+
					"category rather than a change; `git commit --amend` before you push",
				len(prefixed), len(lines)).At(prefixed[0]))
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
				out = append(out, l.pastFindings("OSS-708", l.historySeverity(), hits,
					func(n int, reach string) string {
						return fmt.Sprintf("%s is in %d commit(s), and %s", p.what, n, reach)
					})...)
			}
		}
		return out
	},
}, {
	id: "OSS-709", severity: lint.Error,
	title: "A commit body carries no bullet that did not earn its place",
	why: "A stated maximum becomes a target. Where a convention named three as the rare " +
		"maximum, one repository put exactly three in 31 of 149 commits, tying two for " +
		"the commonest non-zero count. The bullets read well one at a time, so only the " +
		"distribution shows it. This reports the shapes that produce it: a run longer than " +
		"the convention describes, and a line that restates the subject in other words.",
	check: func(l *Linter) []lint.Problem {
		log, err := l.repo.Git("log", "--format=%H%x00%s%x00%b%x1e")
		if err != nil {
			return []lint.Problem{lint.Skipf("OSS-709", "no git history")}
		}
		var padded, echoed []string
		var total int
		for rec := range strings.SplitSeq(log, "\x1e") {
			parts := strings.SplitN(strings.TrimLeft(rec, "\n"), "\x00", 3)
			if len(parts) < 3 {
				continue
			}
			sha, subject, body := shortSHA(parts[0]), parts[1], parts[2]
			bullets := bodyBullets(body)
			if len(bullets) == 0 {
				continue
			}
			total++
			if len(bullets) > maxBodyBullets {
				padded = append(padded, sha)
			}
			for _, b := range bullets {
				if restates(subject, b) {
					echoed = append(echoed, sha)
					break
				}
			}
		}
		if total == 0 {
			return []lint.Problem{lint.Skipf("OSS-709", "no commit body uses bullets")}
		}
		out := l.pastFindings("OSS-709", lint.Warn, padded, func(n int, reach string) string {
			return fmt.Sprintf("%d of %d bulleted bodies run past %d points, and %s; read them "+
				"for the one added to fill the shape", n, total, maxBodyBullets, reach)
		})
		return append(out, l.pastFindings("OSS-709", lint.Warn, echoed, func(n int, reach string) string {
			return fmt.Sprintf("%d of %d bulleted bodies open a line that restates the subject, "+
				"and %s", n, total, reach)
		})...)
	},
}, {
	id: "OSS-710", severity: lint.Error,
	title: "No commit body narrates the work instead of describing the change",
	why: "A reader of the history was not there. \"As requested\", \"after investigating\" " +
		"and \"this commit\" describe the session rather than the software, and an agent " +
		"writing from a transcript reaches for them by default.",
	check: func(l *Linter) []lint.Problem {
		log, err := l.repo.Git("log", "--format=%H%x00%b%x1e")
		if err != nil {
			return []lint.Problem{lint.Skipf("OSS-710", "no git history")}
		}
		var hits []string
		var quote string
		for rec := range strings.SplitSeq(log, "\x1e") {
			sha, body, found := strings.Cut(strings.TrimLeft(rec, "\n"), "\x00")
			if !found {
				continue
			}
			for line := range strings.SplitSeq(body, "\n") {
				if narratesProcess.MatchString(line) {
					hits = append(hits, shortSHA(sha))
					if quote == "" {
						quote = strings.TrimSpace(line)
					}
					break
				}
			}
		}
		if len(hits) == 0 {
			return nil
		}
		if len(quote) > 70 {
			quote = quote[:70]
		}
		return l.pastFindings("OSS-710", lint.Warn, hits, func(n int, reach string) string {
			return fmt.Sprintf("%d commit body(s) narrate the work, and %s: %q", n, reach, quote)
		})
	},
}, {
	id: "OSS-711", severity: lint.Error,
	title: "No commit body has grown into a report of the session",
	why: "A convention that asks for quality and never mentions length produces long " +
		"messages, because plain English, whole sentences and writing for somebody who was " +
		"not there are each satisfied by writing more. One project's bodies ran to a median " +
		"of 20 words under a convention with an implicit ceiling, and averaged 98 in the " +
		"first nine commits after it was dropped. A body answers the question the subject " +
		"leaves and then stops. The rest is the pull request's job.",
	check: func(l *Linter) []lint.Problem {
		log, err := l.repo.Git("log", "--format=%H%x00%b%x1e")
		if err != nil {
			return []lint.Problem{lint.Skipf("OSS-711", "no git history")}
		}
		var long, sprawling []string
		var total, worst int
		for rec := range strings.SplitSeq(log, "\x1e") {
			sha, body, found := strings.Cut(strings.TrimLeft(rec, "\n"), "\x00")
			if !found {
				continue
			}
			words, paras := bodyProse(body)
			if words == 0 {
				continue
			}
			total++
			if words > maxBodyWords {
				long = append(long, shortSHA(sha))
			}
			if paras > maxBodyParagraphs {
				sprawling = append(sprawling, shortSHA(sha))
			}
			worst = max(worst, words)
		}
		if total == 0 {
			return []lint.Problem{lint.Skipf("OSS-711", "no commit carries a body")}
		}
		out := l.pastFindings("OSS-711", lint.Warn, long, func(n int, reach string) string {
			return fmt.Sprintf("%d of %d bodies run past %d words, and %s; the longest in the "+
				"history runs to %d", n, total, maxBodyWords, reach, worst)
		})
		return append(out, l.pastFindings("OSS-711", lint.Warn, sprawling, func(n int, reach string) string {
			return fmt.Sprintf("%d of %d bodies run to more than %d paragraphs, and %s; read "+
				"them for the one that reports the session rather than the change",
				n, total, maxBodyParagraphs, reach)
		})...)
	},
}}

// maxBodyWords and maxBodyParagraphs are where a body stops answering the
// question the subject left and starts reporting the session. Set well past
// what a good body needs, so the rule fires on the shape rather than on one
// long sentence. Not a convention to state in CONTRIBUTING: a number stated
// there becomes the length messages get written to.
const (
	maxBodyWords      = 120
	maxBodyParagraphs = 2
)

// maxBodyBullets is where a body stops carrying points and starts being
// filled to a shape. Not a convention to state in CONTRIBUTING: naming a
// number there is what produces the padding this reports.
const maxBodyBullets = 3

// restates reports whether a body line says what the subject already said.
// Content words shared with the subject, over a line short enough that the
// overlap is most of it.
func restates(subject, line string) bool {
	stop := map[string]bool{"the": true, "a": true, "an": true, "and": true, "or": true,
		"to": true, "of": true, "in": true, "on": true, "for": true, "is": true, "it": true,
		"that": true, "with": true, "not": true, "no": true, "so": true, "as": true}
	want := map[string]bool{}
	for w := range strings.FieldsSeq(strings.ToLower(subject)) {
		w = strings.Trim(w, ".,:;`\"'()")
		if len(w) > 3 && !stop[w] {
			want[w] = true
		}
	}
	if len(want) < 2 {
		return false
	}
	var words, shared int
	for w := range strings.FieldsSeq(strings.ToLower(line)) {
		w = strings.Trim(w, ".,:;`\"'()")
		if len(w) <= 3 || stop[w] {
			continue
		}
		words++
		if want[w] {
			shared++
		}
	}
	return words > 0 && words <= 8 && shared*2 >= words
}

func shortSHA(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 9 {
		return sha[:9]
	}
	return sha
}
