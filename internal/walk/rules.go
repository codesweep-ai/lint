package walk

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/codesweep-ai/lint/internal/lint"
)

var (
	verbWord  = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	flagWord  = regexp.MustCompile(`^--[a-z][a-z0-9-]+$`)
	pathToken = regexp.MustCompile("[`\\[(]([\\w.@-]+/[\\w./@-]+)[`\\])]")
	citation  = regexp.MustCompile(`(\w+\.md)\s*(?:§|section\s+)([\d.]+)`)
	// The same citation as it appears in source, where the document is often
	// left out because there is only one that numbers anything.
	sourceCitation = regexp.MustCompile(`(?:(\w+\.md)\s*)?§\s?([\d.]+)`)
	semver         = regexp.MustCompile(`\b\d+\.\d+\.\d+\b`)
	commandDash    = regexp.MustCompile(`command -v ([\w.-]+)`)
	shortExt       = regexp.MustCompile(`^[a-z0-9]{1,6}$`)
)

// placeholders are the shapes a path takes when it is the reader's to supply.
var placeholders = []*regexp.Regexp{
	regexp.MustCompile(`~/projects/\S+`),
	regexp.MustCompile(`/path/to/\S+`),
	regexp.MustCompile(`<your[-\w]*>`),
	regexp.MustCompile(`~/my-\S+`),
	regexp.MustCompile(`/my-\S+`),
	regexp.MustCompile(`<PATH>`),
	regexp.MustCompile(`<name-of-\S+>`),
}

func isShell(lang string) bool {
	switch lang {
	case "bash", "sh", "shell", "console":
		return true
	}
	return false
}

var rules = []rule{{
	id: "WALK-101", severity: lint.Error,
	title: "Every command a document names exists",
	why: "A reader who types what the page says and gets 'unknown command' stops trusting " +
		"the page, and cannot tell which of the rest is also stale.",
	check: func(l *Linter) []lint.Problem {
		if l.Binary() == "" {
			return []lint.Problem{lint.Skipf("WALK-101", "no %s binary to ask", l.Tool())}
		}
		tool := l.Tool()
		carried := l.Verbs()
		named := map[string]string{}
		for _, block := range l.Blocks() {
			for _, command := range block.Commands {
				words := strings.Fields(command)
				if len(words) == 0 || filepath.Base(words[0]) != tool {
					continue
				}
				var path []string
				for _, word := range words[1:] {
					if !verbWord.MatchString(word) {
						break
					}
					path = append(path, word)
					if carried[strings.Join(path, " ")] {
						continue
					}
					break
				}
				if len(path) > 0 {
					key := strings.Join(path, " ")
					if _, seen := named[key]; !seen {
						named[key] = block.Where()
					}
				}
			}
		}
		var out []lint.Problem
		for _, verb := range lint.SortedKeys(named) {
			if carried[verb] {
				continue
			}
			// A leading word may be an argument rather than a verb, so only
			// the first word is a claim about the surface.
			if carried[strings.Fields(verb)[0]] {
				continue
			}
			if _, status := l.RunTool(append(strings.Fields(verb), "--help")...); status != 0 && status >= 0 {
				out = append(out, lint.Errorf("WALK-101",
					"`%s %s` is documented and the binary does not have it", tool, verb).
					At(named[verb]))
			}
		}
		return out
	},
}, {
	id: "WALK-102", severity: lint.Error,
	title: "Every command the tool carries is documented",
	why: "A verb no document names is a verb nobody finds. The manual is the reference by " +
		"the project's own doc map, so a gap there is a gap everywhere.",
	check: func(l *Linter) []lint.Problem {
		if l.Binary() == "" {
			return []lint.Problem{lint.Skipf("WALK-102", "no %s binary to ask", l.Tool())}
		}
		prose := l.AllText()
		var missing []string
		for _, verb := range lint.SortedKeys(l.Verbs()) {
			if verb == "help" {
				continue
			}
			parts := strings.Fields(verb)
			last := parts[len(parts)-1]
			if wholeWord(last).MatchString(prose) {
				continue
			}
			missing = append(missing, verb)
		}
		if len(missing) == 0 {
			return nil
		}
		return []lint.Problem{lint.Errorf("WALK-102",
			"the binary carries %d command(s) no document names: %s",
			len(missing), strings.Join(missing, ", "))}
	},
}, {
	id: "WALK-103", severity: lint.Warn,
	title: "Every flag the tool carries is documented",
	why: "An option nobody documents is an option nobody uses, and the manual claims to " +
		"list them all.",
	check: func(l *Linter) []lint.Problem {
		if l.Binary() == "" {
			return []lint.Problem{lint.Skipf("WALK-103", "no %s binary to ask", l.Tool())}
		}
		prose := l.AllText()
		var missing []string
		for _, f := range lint.SortedKeys(l.Flags()) {
			if f != "--help" && !strings.Contains(prose, f) {
				missing = append(missing, f)
			}
		}
		if len(missing) == 0 {
			return nil
		}
		return []lint.Problem{lint.Warnf("WALK-103",
			"the binary carries %d flag(s) no document names: %s",
			len(missing), strings.Join(missing, ", "))}
	},
}, {
	id: "WALK-104", severity: lint.Error,
	title: "Every flag a document attributes to the tool exists",
	why: "A flag that was renamed leaves a document telling readers to pass one the parser " +
		"rejects, and the error names the flag rather than the page.",
	check: func(l *Linter) []lint.Problem {
		if l.Binary() == "" {
			return []lint.Problem{lint.Skipf("WALK-104", "no %s binary to ask", l.Tool())}
		}
		tool := l.Tool()
		carried := l.Flags()
		seen := map[string]bool{}
		var out []lint.Problem
		for _, block := range l.Blocks() {
			for _, command := range block.Commands {
				words := strings.Fields(command)
				if len(words) == 0 || filepath.Base(words[0]) != tool {
					continue
				}
				for _, word := range words[1:] {
					flag, _, _ := strings.Cut(word, "=")
					if !flagWord.MatchString(flag) || carried[flag] || seen[flag] || flag == "--help" {
						continue
					}
					seen[flag] = true
					out = append(out, lint.Errorf("WALK-104",
						"`%s` is passed to %s in a document and the binary has no such flag",
						flag, tool).At(block.Where()))
				}
			}
		}
		return out
	},
}, {
	id: "WALK-201", severity: lint.Error,
	title: "Every environment variable the code reads is documented",
	why: "A setting only the source names is a setting only its author knows, and one of " +
		"them usually moves a boundary the spec states as a requirement.",
	check: func(l *Linter) []lint.Problem {
		prefix := l.EnvPrefix()
		read := map[string]string{}
		l.Source(func(path, body string) {
			for name := range envReads(prefix, body) {
				if _, seen := read[name]; !seen {
					read[name] = path
				}
			}
		})
		if len(read) == 0 {
			return []lint.Problem{lint.Skipf("WALK-201", "no %s* variable is read in the source", prefix)}
		}
		prose := l.AllText()
		var out []lint.Problem
		for _, name := range lint.SortedKeys(read) {
			if strings.Contains(prose, name) {
				continue
			}
			if _, internal := l.cfg.EnvInternal[name]; internal {
				continue
			}
			out = append(out, lint.Errorf("WALK-201",
				"%s is read by the code and named in no document", name).At(read[name]))
		}
		return out
	},
}, {
	id: "WALK-202", severity: lint.Warn,
	title: "Every environment variable a document names is read",
	why: "A variable the code stopped reading still reads as a setting, and a reader who " +
		"sets it is debugging a document rather than the software.",
	check: func(l *Linter) []lint.Problem {
		prefix := l.EnvPrefix()
		read := map[string]bool{}
		l.Source(func(_, body string) {
			for name := range envReads(prefix, body) {
				read[name] = true
			}
		})
		named := map[string]bool{}
		re := regexp.MustCompile(`\b(` + regexp.QuoteMeta(prefix) + `[A-Z0-9_]+)\b`)
		for _, doc := range l.Docs() {
			for _, m := range re.FindAllStringSubmatch(l.text[doc], -1) {
				named[m[1]] = true
			}
		}
		if len(named) == 0 {
			return []lint.Problem{lint.Skipf("WALK-202", "no %s* variable is named in the documents", prefix)}
		}
		if len(read) == 0 {
			return []lint.Problem{lint.Skipf("WALK-202", "no %s* variable is read in the source", prefix)}
		}
		var out []lint.Problem
		for _, name := range lint.SortedKeys(named) {
			if !read[name] {
				out = append(out, lint.Warnf("WALK-202",
					"%s is documented and the code does not read it", name))
			}
		}
		return out
	},
}, {
	id: "WALK-301", severity: lint.Warn,
	title: "A block a reader copies names nothing they lack",
	why: "A walkthrough introduced as runnable end to end, opening on a repo the reader was " +
		"never given, fails on its first line. Say the path is theirs to supply, or give them one.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		seen := map[string]bool{}
		for _, block := range l.Blocks() {
			if !isShell(block.Lang) {
				continue
			}
			for _, command := range block.Commands {
				var hits []string
				for _, p := range placeholders {
					hits = append(hits, p.FindAllString(command, -1)...)
				}
				for _, hit := range hits {
					allowed := false
					for _, ok := range l.cfg.PlaceholderOK {
						if strings.Contains(hit, ok) {
							allowed = true
							break
						}
					}
					if allowed {
						continue
					}
					// A shorter pattern matching inside a longer hit is the
					// same placeholder seen twice: /my-service inside
					// ~/my-service.
					nested := false
					for _, other := range hits {
						if other != hit && strings.Contains(other, hit) {
							nested = true
							break
						}
					}
					if nested {
						continue
					}
					msg := "a command names " + hit + ", which the reader has to supply"
					// One report per document is enough to send someone to the
					// section.
					key := block.Where() + "\x00" + msg
					if seen[key] {
						continue
					}
					seen[key] = true
					out = append(out, lint.Warnf("WALK-301", "%s", msg).At(block.Where()))
				}
			}
		}
		return out
	},
}, {
	id: "WALK-302", severity: lint.Error,
	title: "Every repository path a document names exists",
	why: "A file that moves leaves the documents pointing at where it was, and nothing " +
		"fails. The reference is then wrong until somebody happens to read that line. " +
		"Every tracked Markdown file is read, not only the document set: a nested " +
		"README makes the same claim, and markdownSkip says which trees make none.",
	check: func(l *Linter) []lint.Problem {
		roots := map[string]bool{}
		for _, p := range l.repo.Tracked() {
			roots[strings.SplitN(p, "/", 2)[0]] = true
		}
		tracked := map[string]bool{}
		for _, p := range l.repo.Tracked() {
			tracked[p] = true
		}
		var out []lint.Problem
		seen := map[string]bool{}
		for _, name := range l.Markdown() {
			body := l.text[name]
			for _, m := range pathToken.FindAllStringSubmatch(body, -1) {
				token := strings.TrimRight(m[1], ".,;:")
				head := strings.SplitN(token, "/", 2)[0]
				if !roots[head] || strings.HasPrefix(token, "http") {
					continue
				}
				if l.repo.Exists(token) || tracked[token] {
					continue
				}
				// A path under a generated or ignored tree is not a claim
				// about a tracked file, and a trailing wildcard names a family.
				if strings.Contains(token, "*") || strings.HasSuffix(token, "/") {
					continue
				}
				// package/file.Symbol is a citation of code, not of a path. An
				// extension is short and lower case; a camelCase tail is a symbol.
				parts := strings.Split(token, "/")
				tail := parts[len(parts)-1]
				if i := strings.LastIndex(tail, "."); i >= 0 {
					if !shortExt.MatchString(tail[i+1:]) {
						continue
					}
				}
				key := name + "\x00" + token
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, lint.Errorf("WALK-302",
					"%s is named here and does not exist", token).At(name))
			}
		}
		return out
	},
}, {
	id: "WALK-303", severity: lint.Error,
	title: "Every section citation resolves",
	why: "A citation like SPEC.md §7.2 is useful only while §7.2 says what it said. " +
		"Renumbering breaks every one at once, and a stale citation sends a reader to a " +
		"rule that now means something else.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		seen := map[string]bool{}
		for _, name := range l.Docs() {
			body := l.text[name]
			for _, m := range citation.FindAllStringSubmatchIndex(body, -1) {
				target := body[m[2]:m[3]]
				section := strings.TrimRight(body[m[4]:m[5]], ".")
				text, ok := l.text[target]
				if !ok {
					continue
				}
				if sectionHeading(section).MatchString(text) {
					continue
				}
				key := name + "\x00" + target + "\x00" + section
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, lint.Errorf("WALK-303", "%s has no section %s", target, section).
					At(name+":"+strconv.Itoa(lint.Line(body, m[0]))))
			}
		}
		return out
	},
}, {
	id: "WALK-304", severity: lint.Error,
	title: "Every section citation in the source resolves",
	why: "A comment reading `SPEC.md §7.2` is useful only while §7.2 says what it " +
		"said. Renumbering the spec breaks every one at once, silently, and a stale " +
		"citation sends its next reader to a rule that now means something else. A " +
		"bare § is read against the spec, which is the document that numbers its " +
		"sections.",
	check: func(l *Linter) []lint.Problem {
		fallback := l.citedByDefault()
		var out []lint.Problem
		seen := map[string]bool{}
		l.AllSource(func(path, body string) {
			if l.skipCitations(path) {
				return
			}
			for _, m := range sourceCitation.FindAllStringSubmatchIndex(body, -1) {
				target := fallback
				if m[2] >= 0 {
					target = body[m[2]:m[3]]
				}
				if target == "" {
					continue
				}
				section := strings.TrimRight(body[m[4]:m[5]], ".")
				text, ok := l.text[target]
				if !ok {
					continue
				}
				if sectionHeading(section).MatchString(text) {
					continue
				}
				key := path + "\x00" + target + "\x00" + section
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, lint.Errorf("WALK-304", "%s has no section %s", target, section).
					At(path+":"+strconv.Itoa(lint.Line(body, m[0]))))
			}
		})
		return out
	},
}, {
	id: "WALK-401", severity: lint.Error,
	title: "A sample output is what the command prints today",
	why: "Sample output is the half of a document a reader compares their own screen " +
		"against. Wrong in small ways, it destroys trust in the rest.",
	check: func(l *Linter) []lint.Problem {
		if l.Binary() == "" {
			return []lint.Problem{lint.Skipf("WALK-401", "no %s binary to run", l.Tool())}
		}
		if len(l.cfg.SafeVerbs) == 0 {
			return []lint.Problem{lint.Skipf("WALK-401",
				"no safeVerbs configured, so no sample is re-run")}
		}
		safe := map[string]bool{}
		for _, v := range l.cfg.SafeVerbs {
			safe[v] = true
		}
		tool := l.Tool()
		var out []lint.Problem
		ran := 0
		for _, block := range l.Blocks() {
			if block.Lang != "console" {
				continue
			}
			for i, command := range block.Commands {
				if _, skip := l.cfg.SampleSkip[command]; skip {
					continue
				}
				// A sample is often two commands joined by &&, and the recorded
				// output covers both. Every part has to be safe before any runs.
				var parts [][]string
				ours := true
				for p := range strings.SplitSeq(command, "&&") {
					words := strings.Fields(p)
					if len(words) == 0 || filepath.Base(words[0]) != tool {
						ours = false
						break
					}
					parts = append(parts, words)
				}
				if !ours {
					continue
				}
				allSafe := true
				for _, p := range parts {
					if !safe[verbOf(p[1:])] {
						allSafe = false
						break
					}
				}
				if !allSafe {
					continue
				}
				text, status := "", 0
				var textSb441 strings.Builder
				for _, part := range parts {
					chunk, st := l.RunTool(part[1:]...)
					textSb441.WriteString(chunk)
					status = st
					if st != 0 {
						break
					}
				}
				text += textSb441.String()
				if status < 0 {
					continue
				}
				ran++
				recorded := nonEmpty(block.Output[i])
				actual := nonEmpty(strings.Split(text, "\n"))
				for j, line := range recorded {
					got := ""
					if j < len(actual) {
						got = actual[j]
					}
					if matches(line, got) {
						continue
					}
					anywhere := false
					for _, a := range actual {
						if matches(line, a) {
							anywhere = true
							break
						}
					}
					if anywhere {
						continue
					}
					first := "(nothing)"
					if len(actual) > 0 {
						first = actual[0]
					}
					out = append(out, lint.Errorf("WALK-401",
						"`%s` no longer prints %q; it prints %q. Fix the sample, or name the "+
							"command in sampleSkip with the reason it cannot reproduce here",
						command, strings.TrimSpace(line), strings.TrimSpace(first)).
						At(block.Where()))
					break
				}
			}
		}
		if ran == 0 {
			return []lint.Problem{lint.Skipf("WALK-401", "no sample named a verb in safeVerbs")}
		}
		return out
	},
}, {
	id: "WALK-402", severity: lint.Warn,
	title: "A version a document quotes is the version it ships",
	why: "A sample naming a version that has moved is the cheapest kind of wrong, and a " +
		"reader comparing their own output cannot tell which of you is stale.",
	check: func(l *Linter) []lint.Problem {
		if l.Binary() == "" {
			return []lint.Problem{lint.Skipf("WALK-402", "no %s binary to ask", l.Tool())}
		}
		text, status := l.RunTool("version")
		if status != 0 {
			return []lint.Problem{lint.Skipf("WALK-402", "`%s version` did not run", l.Tool())}
		}
		current := map[string]bool{}
		for _, v := range semver.FindAllString(text, -1) {
			current[v] = true
		}
		if len(current) == 0 {
			return []lint.Problem{lint.Skipf("WALK-402", "the version output carries no x.y.z number")}
		}
		re := regexp.MustCompile(`(?m)^\s*(?:\$\s*)?(?:\S*/)?` + regexp.QuoteMeta(l.Tool()) + `\s+version\s*$`)
		var out []lint.Problem
		for _, name := range l.Docs() {
			body := l.text[name]
			for _, m := range re.FindAllStringIndex(body, -1) {
				end := min(m[1]+400, len(body))
				tail := strings.SplitN(body[m[1]:end], "```", 2)[0]
				var stale []string
				for _, q := range semver.FindAllString(tail, -1) {
					if !current[q] {
						stale = append(stale, q)
					}
				}
				if len(stale) > 0 {
					out = append(out, lint.Warnf("WALK-402",
						"a `%s version` sample quotes %s; the binary prints %s",
						l.Tool(), strings.Join(stale, ", "), strings.Join(lint.SortedKeys(current), ", ")).
						At(name+":"+strconv.Itoa(lint.Line(body, m[0]))))
				}
			}
		}
		return out
	},
}, {
	id: "WALK-403", severity: lint.Error,
	title: "The manual the binary carries is the manual in the tree",
	why: "A tool that prints its own manual is the copy a machine with no checkout " +
		"reads. When the binary is stale the two disagree, and the reader who cannot " +
		"see the tree is the one getting the wrong answer.",
	check: func(l *Linter) []lint.Problem {
		const manual = "MANUAL.md"
		body, ok := l.text[manual]
		if !ok {
			return []lint.Problem{lint.Skipf("WALK-403", "no %s to compare against", manual)}
		}
		if l.Binary() == "" {
			return []lint.Problem{lint.Skipf("WALK-403", "no %s binary to ask", l.Tool())}
		}
		if !l.Verbs()["manual"] {
			return []lint.Problem{lint.Skipf("WALK-403",
				"`%s manual` is not a verb this tool carries", l.Tool())}
		}
		printed, status := l.RunTool("manual")
		if status != 0 {
			return []lint.Problem{lint.Skipf("WALK-403", "`%s manual` did not run", l.Tool())}
		}
		// A terminal writer may end the stream with a newline the file does
		// not carry, and that is not drift anybody can act on.
		if strings.TrimRight(printed, "\n") == strings.TrimRight(body, "\n") {
			return nil
		}
		return []lint.Problem{lint.Errorf("WALK-403",
			"`%s manual` prints %s, and the binary is stale; rebuild it",
			l.Tool(), describeDrift(printed, body)).At(manual)}
	},
}, {
	id: "WALK-501", severity: lint.Error,
	title: "Every tool the build needs is named in a document",
	why: "A contributor told to run one command, on a machine missing a tool nothing named, " +
		"meets the gate as a failure rather than as a setup step. The answer lives in an " +
		"error message only a failed run prints.",
	check: func(l *Linter) []lint.Problem {
		prose := l.AllText()
		ok := map[string]bool{l.Tool(): true}
		for _, t := range l.cfg.PrereqOK {
			ok[t] = true
		}
		needed := map[string]string{}
		l.BuildFiles(func(path, body string) {
			for _, m := range commandDash.FindAllStringSubmatch(body, -1) {
				name := m[1]
				if ok[name] || strings.HasPrefix(name, "$") {
					continue
				}
				if _, seen := needed[name]; !seen {
					needed[name] = path
				}
			}
		})
		if len(needed) == 0 {
			return []lint.Problem{lint.Skipf("WALK-501", "the build shells out to nothing it checks for")}
		}
		var out []lint.Problem
		for _, t := range lint.SortedKeys(needed) {
			if namesTool(t).MatchString(prose) {
				continue
			}
			out = append(out, lint.Errorf("WALK-501",
				"%s has to be installed for the build and no document names it", t).At(needed[t]))
		}
		return out
	},
}, {
	id: "WALK-601", severity: lint.Warn,
	title: "The manual answers the automated caller",
	why: "An agent driving the tool needs what a human infers: which commands are " +
		"non-interactive, which output is machine-readable, and what touches the network or " +
		"the filesystem.",
	check: func(l *Linter) []lint.Problem {
		manual, ok := l.text["MANUAL.md"]
		if !ok {
			return []lint.Problem{lint.Skipf("WALK-601", "no MANUAL.md in the document set")}
		}
		if strings.Contains(strings.ToLower(manual), strings.ToLower(l.cfg.AgentSection)) {
			return nil
		}
		return []lint.Problem{lint.Warnf("WALK-601", "MANUAL.md has no %q section",
			l.cfg.AgentSection).At("MANUAL.md")}
	},
}, {
	id: "WALK-602", severity: lint.Error,
	title: "The router names every document in the set",
	why: "An agent reads the AGENTS.md nearest the file it is editing and goes no further. " +
		"A document the router omits is a document it never opens.",
	check: func(l *Linter) []lint.Problem {
		router, ok := l.text["AGENTS.md"]
		if !ok {
			return []lint.Problem{lint.Skipf("WALK-602", "no AGENTS.md at the root")}
		}
		var out []lint.Problem
		for _, n := range l.Docs() {
			if n != "AGENTS.md" && !strings.Contains(router, n) {
				out = append(out, lint.Errorf("WALK-602", "AGENTS.md does not name %s", n).At("AGENTS.md"))
			}
		}
		return out
	},
}}

func nonEmpty(lines []string) []string {
	var out []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimRight(l, " \t\r"))
		}
	}
	return out
}

// The three patterns below are built from a verb, a tool name or a section
// number, so none can be a package-level constant. They are compiled where
// they are used: a run builds a few dozen at most, and a cache for that would
// buy nothing measurable while adding shared mutable state to a package whose
// rules are otherwise free of it.

// wholeWord matches a word the documents have to name somewhere.
func wholeWord(w string) *regexp.Regexp {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(w) + `\b`)
}

// sectionHeading matches the numbered section a citation points at.
//
// A subsection is as often a bold lead-in as a heading: a spec that reads
// "**3.1 Object key order.**" numbers its rules without adding a level to the
// table of contents. Matching only headings reports every citation of one as
// stale, which is a linter calling a correct document wrong.
func sectionHeading(section string) *regexp.Regexp {
	q := regexp.QuoteMeta(section)
	return regexp.MustCompile(`(?m)^(?:#{1,6}\s+` + q + `[.\s]|\*\*` + q + `[.\s])`)
}

// namesTool matches a tool named in prose, bounded so a short name does not
// match inside a longer one.
func namesTool(t string) *regexp.Regexp {
	return regexp.MustCompile("[`\\s/]" + regexp.QuoteMeta(t) + "[`\\s.,@]")
}

// describeDrift says how the printed document differs from the file, so the
// finding is actionable without running a diff. The first differing line is
// where a reader would start looking, and where a rebuild would show its work.
func describeDrift(printed, file string) string {
	p, f := splitLines(printed), splitLines(file)
	for i := 0; i < len(p) && i < len(f); i++ {
		if p[i] != f[i] {
			return fmt.Sprintf("a different line %d: %q, where the file has %q",
				i+1, lint.Truncate(p[i], 60), lint.Truncate(f[i], 60))
		}
	}
	return fmt.Sprintf("%d lines, where the file has %d", len(p), len(f))
}
