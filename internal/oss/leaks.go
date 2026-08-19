package oss

import (
	"os"
	"regexp"
	"strings"

	"github.com/codesweep-ai/lint/internal/lint"
)

// The leak patterns catch a class, never an instance.
//
// A list of private names is itself a disclosure: it publishes exactly what
// you consider private, and it is the first thing a curious reader opens. It
// also rots, because every new name has to be remembered. So a username is
// "the segment after /home/ that is not a placeholder this project ships", and
// the person running the check contributes their own name from the environment
// at run time without it ever being written down.
//
// The original expressed each allowance as a negative lookahead compiled into
// the pattern. Go's regexp has none, and the allowance reads better beside the
// match anyway: the pattern finds the shape, and a set decides whether this
// one is a person.
var (
	homePath  = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.])/home/([A-Za-z0-9_-][A-Za-z0-9_.-]*)`)
	usersPath = regexp.MustCompile(`/Users/([A-Za-z0-9_-][A-Za-z0-9_.-]*)`)
	statePath = regexp.MustCompile(`/(?:home|Users)/([A-Za-z0-9_.-]+)/` +
		`\.(?:cache|config|local|ssh|aws|gnupg|kube|docker|npm|claude|codex)\b`)
	mailAddr = regexp.MustCompile(`[A-Za-z0-9._%+-]+@([A-Za-z0-9.-]+\.[A-Za-z]{2,})`)
)

// allowed reports whether a captured segment is one of the names allowed,
// followed by a word boundary. "/home/user" is a placeholder and "/home/userb"
// is somebody's login, which is what the boundary in the original pattern said.
func allowed(captured string, names []string) bool {
	for _, name := range names {
		if !strings.HasPrefix(captured, name) {
			continue
		}
		rest := captured[len(name):]
		if rest == "" || !lint.IsWordByte(rest[0]) {
			return true
		}
	}
	return false
}

type leakPattern struct {
	rule string
	what string
	find func(*Linter, string) (index int, ok bool)
}

func (l *Linter) leakPatterns() []leakPattern {
	homeAllow := l.cfg.HomeAllow
	mailAllow := append(append([]string(nil), reservedDomains...), l.cfg.EmailAllow...)

	pats := []leakPattern{{
		rule: "OSS-301", what: "a home directory naming a person",
		find: firstNotAllowed(homePath, homeAllow),
	}, {
		rule: "OSS-301", what: "a macOS home directory naming a person",
		find: firstNotAllowed(usersPath, homeAllow),
	}, {
		rule: "OSS-302", what: "an absolute path into a person's own state",
		find: firstNotAllowed(statePath, homeAllow),
	}, {
		rule: "OSS-303", what: "a mail address",
		find: func(_ *Linter, body string) (int, bool) {
			for _, m := range mailAddr.FindAllStringSubmatchIndex(body, -1) {
				domain := body[m[2]:m[3]]
				if allowed(domain, mailAllow) || allowed(domain, []string{"noreply"}) {
					continue
				}
				tld := domain
				if i := strings.LastIndex(domain, "."); i >= 0 {
					tld = domain[i+1:]
				}
				if allowed(tld, reservedTLDs) {
					continue
				}
				return m[0], true
			}
			return 0, false
		},
	}}

	// The invoking user, from the environment — never written into this file.
	for _, name := range []string{os.Getenv("USER"), os.Getenv("LOGNAME")} {
		if len(name) <= 2 || allowed(name, homeAllow) {
			continue
		}
		re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(name) + `\b`)
		if err != nil {
			continue
		}
		pats = append(pats, leakPattern{"OSS-304", "the invoking user's own name", firstMatch(re)})
	}

	// Terms no pattern can infer. Absent by default and gitignored when
	// present, so the list of what you consider private never lands in the
	// repository.
	if body, err := os.ReadFile(l.repo.Path(".leakterms")); err == nil {
		for term := range strings.SplitSeq(string(body), "\n") {
			term = strings.TrimSpace(term)
			if term == "" || strings.HasPrefix(term, "#") {
				continue
			}
			re, err := regexp.Compile(`(?i)` + regexp.QuoteMeta(term))
			if err != nil {
				continue
			}
			pats = append(pats, leakPattern{"OSS-304", "a term from .leakterms", firstMatch(re)})
		}
	}
	return pats
}

func firstMatch(re *regexp.Regexp) func(*Linter, string) (int, bool) {
	return func(_ *Linter, body string) (int, bool) {
		if m := re.FindStringIndex(body); m != nil {
			return m[0], true
		}
		return 0, false
	}
}

func firstNotAllowed(re *regexp.Regexp, names []string) func(*Linter, string) (int, bool) {
	return func(_ *Linter, body string) (int, bool) {
		for _, m := range re.FindAllStringSubmatchIndex(body, -1) {
			if allowed(body[m[2]:m[3]], names) {
				continue
			}
			return m[0], true
		}
		return 0, false
	}
}

// scan makes one pass over every tracked text file, shared by the 3xx rules.
func (l *Linter) scan() []lint.Problem {
	if l.scanned {
		return l.leaks
	}
	l.scanned = true
	patterns := l.leakPatterns()
	l.scannable(func(path, body string) {
		for _, p := range patterns {
			index, ok := p.find(l, body)
			if !ok {
				continue
			}
			l.leaks = append(l.leaks, lint.Errorf(p.rule, "%s: %s", p.what, excerpt(body, index)).
				At(lint.At(path, body, index)))
			return // one report per file is enough to act on
		}
	})
	return l.leaks
}

func (l *Linter) leaksFor(id string) []lint.Problem {
	var out []lint.Problem
	for _, p := range l.scan() {
		if p.Rule == id {
			out = append(out, p)
		}
	}
	return out
}

var secretPatterns = []struct{ pattern, what string }{
	{`-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`, "a private key"},
	{`\bsk-ant-[A-Za-z0-9_-]{16,}`, "an Anthropic key"},
	{`\bsk-(?:proj-)?[A-Za-z0-9]{32,}`, "an OpenAI-shaped key"},
	{`\bgh[pousr]_[A-Za-z0-9]{30,}`, "a GitHub token"},
	{`\bgithub_pat_[A-Za-z0-9_]{50,}`, "a GitHub fine-grained token"},
	{`\bAKIA[0-9A-Z]{16}\b`, "an AWS access key id"},
	{`\bASIA[0-9A-Z]{16}\b`, "an AWS session key id"},
	{`\bAIza[0-9A-Za-z_-]{35}\b`, "a Google API key"},
	{`\bxox[abprs]-[0-9A-Za-z-]{10,}`, "a Slack token"},
	{`\bglpat-[0-9A-Za-z_-]{20,}`, "a GitLab token"},
	{`\bfw_[A-Za-z0-9]{20,}`, "a Fireworks key"},
	{`\bey[A-Za-z0-9_-]{14,}\.ey[A-Za-z0-9_-]{14,}\.[A-Za-z0-9_-]{14,}`, "a JWT"},
	{`(?i)\b(?:api[_-]?key|client[_-]?secret|passwd|password|auth[_-]?token)\b` +
		`\s*[:=]\s*['"][A-Za-z0-9/+_-]{20,}['"]`, "a literal credential"},
}

// fakeMarkers exempt a value that says of itself that it is fake.
//
// A test needs a credential-shaped string, and the only safe one says so in
// itself. This makes "spell your fixtures obviously fake" the rule the check
// enforces, rather than a rule a reviewer has to remember.
var fakeMarkers = regexp.MustCompile(`(?i)AAAA|XXXX|0000|example|fixture|sample|dummy|fake|` +
	`placeholder|redacted|not-?a-?real|replace-?me|your-?key|CREDENTIAL|TOKEN|SECRET|test-?only`)

var (
	credentialFiles = regexp.MustCompile(`(^|/)(\.env(\..*)?|\.npmrc|\.netrc|id_rsa|id_ed25519|` +
		`.*\.pem|.*\.p12|.*\.pfx|.*\.keystore|credentials\.json|service-account.*\.json)$`)
	buildOutput  = regexp.MustCompile(`^(bin|dist|build|out|target|node_modules|__pycache__|\.venv|coverage)/`)
	ignoresEnv   = regexp.MustCompile(`(?m)^/?\.env\b`)
	declaresLF   = regexp.MustCompile(`(?m)^\*\s+text=auto\s+eol=lf`)
	secretsBuilt []struct {
		re   *regexp.Regexp
		what string
	}
)

func init() {
	for _, p := range secretPatterns {
		secretsBuilt = append(secretsBuilt, struct {
			re   *regexp.Regexp
			what string
		}{regexp.MustCompile(p.pattern), p.what})
	}
}

var leakRules = []rule{{
	id: "OSS-301", severity: lint.Error, needsTree: true,
	title: "No tracked file names a person's home directory",
	why: "A path is the easiest leak to make and the hardest to notice: it reaches a " +
		"fixture, then a golden derived from it, then a manifest, and every copy carries a " +
		"login name. Placeholder names a shipped image uses go in homeAllow, and nothing else does.",
	check: func(l *Linter) []lint.Problem { return l.leaksFor("OSS-301") },
}, {
	id: "OSS-302", severity: lint.Error, needsTree: true,
	title: "No tracked file points into a person's own state",
	why: "A cache or config path leaks a login even after the /home prefix has been " +
		"rewritten, which is how a browser path once reached a committed manifest.",
	check: func(l *Linter) []lint.Problem { return l.leaksFor("OSS-302") },
}, {
	id: "OSS-303", severity: lint.Error, needsTree: true,
	title: "No tracked file carries a mail address",
	why: "An address in a public repository is scraped within days. The documentation " +
		"domains and reserved test TLDs are the exception, and a fixture that needs an " +
		"address should use one of them.",
	check: func(l *Linter) []lint.Problem { return l.leaksFor("OSS-303") },
}, {
	id: "OSS-304", severity: lint.Error, needsTree: true,
	title: "No tracked file names the person publishing it",
	why: "The name comes from the environment at run time, so the check works without " +
		"the name ever being written down.",
	check: func(l *Linter) []lint.Problem { return l.leaksFor("OSS-304") },
}, {
	id: "OSS-305", severity: lint.Error, needsTree: true,
	title: "No credential is in a tracked file",
	why: "A key in a published commit is compromised the moment the repository is public, " +
		"and deleting it later does not un-publish it. Rotate it as well as removing it. A " +
		"fixture that needs a key-shaped string spells it obviously fake, and is then exempt.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		l.scannable(func(path, body string) {
			for _, s := range secretsBuilt {
				for _, m := range s.re.FindAllStringIndex(body, -1) {
					if fakeMarkers.MatchString(body[m[0]:m[1]]) {
						continue
					}
					out = append(out, lint.Errorf("OSS-305", "%s: %s", s.what,
						shortExcerpt(body, m[0])).At(lint.At(path, body, m[0])))
					return
				}
			}
		})
		return out
	},
}, {
	id: "OSS-306", severity: lint.Error, needsTree: true,
	title: "Every tracked file can be read",
	why: "A file nobody can inspect must never be reported as clean. A committed editor " +
		"swap file smuggled a username past a scan of this kind exactly that way.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		for _, path := range l.unreadable {
			out = append(out, lint.Errorf("OSS-306",
				"cannot be read as text, so its contents were never checked; remove it, "+
					"or add its extension to binaryOK if it is a legitimate asset").At(path))
		}
		return out
	},
}, {
	id: "OSS-307", severity: lint.Error,
	title: "No credential file is tracked, and .env is ignored",
	why: "A local .env is where a key lands first. Ignoring it is what keeps the next " +
		"`git add -A` from publishing it.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		// The rule has two halves, and only one of them needs the tracked
		// list. The ignore file is read from disk, so it is still checked
		// where git answers for nothing — and the half that did not run says
		// so rather than passing in silence.
		if l.nothingToScan() {
			out = append(out, lint.Skipf("OSS-307",
				"no tracked file to read, so nothing was checked for a tracked credential"))
		}
		for _, p := range l.repo.Tracked() {
			if credentialFiles.MatchString(p) {
				out = append(out, lint.Errorf("OSS-307", "a credential file is tracked").At(p))
			}
		}
		ignore, _ := l.read(".gitignore")
		if !ignoresEnv.MatchString(ignore) {
			out = append(out, lint.Errorf("OSS-307", ".gitignore does not ignore .env"))
		}
		return out
	},
}, {
	id: "OSS-308", severity: lint.Error, needsTree: true,
	title: "No build output or dependency tree is tracked",
	why: "A committed artifact is a second copy of what the build makes, and it goes " +
		"stale silently. A committed dependency tree also republishes somebody else's code " +
		"without their licence.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		for _, p := range l.repo.Tracked() {
			if buildOutput.MatchString(p) {
				out = append(out, lint.Errorf("OSS-308", "build output is tracked").At(p))
			}
		}
		if len(out) > 20 {
			out = out[:20]
		}
		return out
	},
}, {
	id: "OSS-309", severity: lint.Warn,
	title: "The ignore file explains itself",
	why: "An ignore rule with no reason is one the next person deletes, and the output it " +
		"was keeping out of the tree comes back.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read(".gitignore")
		if !ok {
			return []lint.Problem{lint.Warnf("OSS-309", "no .gitignore")}
		}
		for line := range strings.SplitSeq(body, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				return nil
			}
		}
		return []lint.Problem{lint.Warnf("OSS-309",
			".gitignore carries no comment saying what it keeps out, or what it deliberately does not")}
	},
}, {
	id: "OSS-310", severity: lint.Warn,
	title: "The checkout is declared LF",
	why: "Without it a checkout on Windows rewrites every text file to CRLF, and a shell " +
		"script then fails with a message naming the interpreter rather than the script.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read(".gitattributes")
		if !ok {
			return []lint.Problem{lint.Warnf("OSS-310", "no .gitattributes declaring the line ending")}
		}
		if !declaresLF.MatchString(body) {
			return []lint.Problem{lint.Warnf("OSS-310",
				".gitattributes does not declare `* text=auto eol=lf`")}
		}
		return nil
	},
}}

// shortExcerpt is the tighter quote a credential gets: enough to find it,
// never enough to republish it in the linter's own output.
func shortExcerpt(text string, index int) string {
	start := index - 20
	lead := "…"
	if start <= 0 {
		start, lead = 0, ""
	}
	end := min(index+24, len(text))
	return lead + strings.Join(strings.Fields(text[start:end]), " ") + "…"
}
