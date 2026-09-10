package surface

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/codesweep-ai/lint/internal/docset"
	"github.com/codesweep-ai/lint/internal/lint"
)

var (
	verbWord = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	flagWord = regexp.MustCompile(`^--[a-z][a-z0-9-]+$`)
	semver   = regexp.MustCompile(`\b\d+\.\d+\.\d+\b`)
)

var rules = []rule{{
	id: "SURF-101", severity: lint.Error,
	title: "Every command a document names exists",
	why: "A reader who types what the page says and gets 'unknown command' stops trusting " +
		"the page, and cannot tell which of the rest is also stale.",
	check: func(l *Linter) []lint.Problem {
		set := l.set
		if set.Binary() == "" {
			return []lint.Problem{lint.Skipf("SURF-101", "no %s binary to ask", set.Tool())}
		}
		tool := set.Tool()
		carried := set.Verbs()
		named := map[string]string{}
		for _, block := range set.Blocks() {
			synopsis := statesAlternatives(block, tool, carried)
			for _, command := range block.Commands {
				words := strings.Fields(command)
				if len(words) == 0 || filepath.Base(words[0]) != tool {
					continue
				}
				for _, path := range verbPaths(words[1:], carried, synopsis) {
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
			if _, status := set.RunTool(append(strings.Fields(verb), "--help")...); status != 0 && status >= 0 {
				out = append(out, lint.Errorf("SURF-101",
					"`%s %s` is documented and the binary does not have it", tool, verb).
					At(named[verb]))
			}
		}
		return out
	},
}, {
	id: "SURF-102", severity: lint.Error,
	title: "Every command the tool carries is documented",
	why: "A verb no document names is a verb nobody finds. The manual is the reference by " +
		"the project's own doc map, so a gap there is a gap everywhere.",
	check: func(l *Linter) []lint.Problem {
		set := l.set
		if set.Binary() == "" {
			return []lint.Problem{lint.Skipf("SURF-102", "no %s binary to ask", set.Tool())}
		}
		prose := set.AllText()
		var missing []string
		for _, verb := range lint.SortedKeys(set.Verbs()) {
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
		return []lint.Problem{lint.Errorf("SURF-102",
			"the binary carries %d command(s) no document names: %s",
			len(missing), strings.Join(missing, ", "))}
	},
}, {
	id: "SURF-103", severity: lint.Warn,
	title: "Every flag the tool carries is documented",
	why: "An option nobody documents is an option nobody uses, and the manual claims to " +
		"list them all.",
	check: func(l *Linter) []lint.Problem {
		set := l.set
		if set.Binary() == "" {
			return []lint.Problem{lint.Skipf("SURF-103", "no %s binary to ask", set.Tool())}
		}
		prose := set.AllText()
		var missing []string
		for _, f := range lint.SortedKeys(set.Flags()) {
			if f != "--help" && !strings.Contains(prose, f) {
				missing = append(missing, f)
			}
		}
		if len(missing) == 0 {
			return nil
		}
		return []lint.Problem{lint.Warnf("SURF-103",
			"the binary carries %d flag(s) no document names: %s",
			len(missing), strings.Join(missing, ", "))}
	},
}, {
	id: "SURF-104", severity: lint.Error,
	title: "Every flag a document attributes to the tool exists",
	why: "A flag that was renamed leaves a document telling readers to pass one the parser " +
		"rejects, and the error names the flag rather than the page.",
	check: func(l *Linter) []lint.Problem {
		set := l.set
		if set.Binary() == "" {
			return []lint.Problem{lint.Skipf("SURF-104", "no %s binary to ask", set.Tool())}
		}
		tool := set.Tool()
		carried := set.Flags()
		seen := map[string]bool{}
		var out []lint.Problem
		for _, block := range set.Blocks() {
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
					out = append(out, lint.Errorf("SURF-104",
						"`%s` is passed to %s in a document and the binary has no such flag",
						flag, tool).At(block.Where()))
				}
			}
		}
		return out
	},
}, {
	id: "SURF-105", severity: lint.Error,
	title: "The section that states the command surface names every verb",
	why: "A section that announces itself as the command surface is where a reader goes to " +
		"learn what the tool does, so a verb missing there is missing, however thoroughly " +
		"another page documents it.",
	check: func(l *Linter) []lint.Problem {
		set := l.set
		if l.cfg.SurfaceSection == "" {
			return []lint.Problem{lint.Skipf("SURF-105", "no surfaceSection configured")}
		}
		if set.Binary() == "" {
			return []lint.Problem{lint.Skipf("SURF-105", "no %s binary to ask", set.Tool())}
		}
		doc, heading, _ := strings.Cut(l.cfg.SurfaceSection, "#")
		body, ok := set.Text(doc)
		if !ok {
			return []lint.Problem{lint.Skipf("SURF-105",
				"surfaceSection names %s, which is not in the document set", doc)}
		}
		first, last, ok := section(body, heading)
		if !ok {
			return []lint.Problem{lint.Skipf("SURF-105", "%s has no section %q", doc, heading)}
		}
		tool := set.Tool()
		carried := set.Verbs()
		named := map[string]bool{}
		for _, block := range set.Blocks() {
			if block.Doc != doc || block.Line < first || block.Line >= last {
				continue
			}
			synopsis := statesAlternatives(block, tool, carried)
			for _, command := range block.Commands {
				words := strings.Fields(command)
				if len(words) == 0 || filepath.Base(words[0]) != tool {
					continue
				}
				for _, path := range verbPaths(words[1:], carried, synopsis) {
					named[path[0]] = true
				}
			}
		}
		// A section naming nothing is a heading that has moved, rather than a
		// surface that lost every verb at once, and reporting the whole tree
		// would bury that.
		if len(named) == 0 {
			return []lint.Problem{lint.Skipf("SURF-105",
				"%s names no %s command", l.cfg.SurfaceSection, tool)}
		}
		var missing []string
		for _, verb := range lint.SortedKeys(carried) {
			// A synopsis states the verbs a reader reaches first. What hangs
			// below one of them is the manual's to document, and SURF-102
			// holds that.
			if generated[verb] || strings.Contains(verb, " ") || named[verb] {
				continue
			}
			missing = append(missing, verb)
		}
		if len(missing) == 0 {
			return nil
		}
		return []lint.Problem{lint.Errorf("SURF-105",
			"%s states the command surface and does not name %d verb(s) the binary carries: %s",
			l.cfg.SurfaceSection, len(missing), strings.Join(missing, ", ")).
			At(doc + ":" + strconv.Itoa(first))}
	},
}, {
	id: "SURF-201", severity: lint.Error,
	title: "Every environment variable the code reads is documented",
	why: "A setting only the source names is a setting only its author knows, and one of " +
		"them usually moves a boundary the spec states as a requirement.",
	check: func(l *Linter) []lint.Problem {
		set := l.set
		prefix := set.EnvPrefix()
		read := map[string]string{}
		set.Source(func(path, body string) {
			for name := range docset.EnvReads(prefix, body) {
				if _, seen := read[name]; !seen {
					read[name] = path
				}
			}
		})
		if len(read) == 0 {
			return []lint.Problem{lint.Skipf("SURF-201", "no %s* variable is read in the source", prefix)}
		}
		prose := set.AllText()
		var out []lint.Problem
		for _, name := range lint.SortedKeys(read) {
			if strings.Contains(prose, name) {
				continue
			}
			if _, internal := l.cfg.EnvInternal[name]; internal {
				continue
			}
			out = append(out, lint.Errorf("SURF-201",
				"%s is read by the code and named in no document", name).At(read[name]))
		}
		return out
	},
}, {
	id: "SURF-202", severity: lint.Warn,
	title: "Every environment variable a document names is read",
	why: "A variable the code stopped reading still reads as a setting, and a reader who " +
		"sets it is debugging a document rather than the software.",
	check: func(l *Linter) []lint.Problem {
		set := l.set
		prefix := set.EnvPrefix()
		read := map[string]bool{}
		set.Source(func(_, body string) {
			for name := range docset.EnvReads(prefix, body) {
				read[name] = true
			}
		})
		named := map[string]bool{}
		re := regexp.MustCompile(`\b(` + regexp.QuoteMeta(prefix) + `[A-Z0-9_]+)\b`)
		for _, doc := range set.Docs() {
			body, _ := set.Text(doc)
			for _, m := range re.FindAllStringSubmatch(body, -1) {
				named[m[1]] = true
			}
		}
		if len(named) == 0 {
			return []lint.Problem{lint.Skipf("SURF-202", "no %s* variable is named in the documents", prefix)}
		}
		if len(read) == 0 {
			return []lint.Problem{lint.Skipf("SURF-202", "no %s* variable is read in the source", prefix)}
		}
		var out []lint.Problem
		for _, name := range lint.SortedKeys(named) {
			if !read[name] {
				out = append(out, lint.Warnf("SURF-202",
					"%s is documented and the code does not read it", name))
			}
		}
		return out
	},
}, {
	id: "SURF-301", severity: lint.Error,
	title: "A sample output is what the command prints today",
	why: "Sample output is the half of a document a reader compares their own screen " +
		"against. Wrong in small ways, it destroys trust in the rest.",
	check: func(l *Linter) []lint.Problem {
		set := l.set
		if set.Binary() == "" {
			return []lint.Problem{lint.Skipf("SURF-301", "no %s binary to run", set.Tool())}
		}
		if len(l.cfg.SafeVerbs) == 0 {
			return []lint.Problem{lint.Skipf("SURF-301",
				"no safeVerbs configured, so no sample is re-run")}
		}
		safe := map[string]bool{}
		for _, v := range l.cfg.SafeVerbs {
			safe[v] = true
		}
		tool := set.Tool()
		var out []lint.Problem
		ran := 0
		for _, block := range set.Blocks() {
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
					if !safe[docset.VerbOf(p[1:])] {
						allSafe = false
						break
					}
				}
				if !allSafe {
					continue
				}
				var printed strings.Builder
				status := 0
				for _, part := range parts {
					chunk, st := set.RunTool(part[1:]...)
					printed.WriteString(chunk)
					status = st
					if st != 0 {
						break
					}
				}
				if status < 0 {
					continue
				}
				ran++
				recorded := nonEmpty(block.Output[i])
				actual := nonEmpty(strings.Split(printed.String(), "\n"))
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
					out = append(out, lint.Errorf("SURF-301",
						"`%s` no longer prints %q; it prints %q. Fix the sample, or name the "+
							"command in sampleSkip with the reason it cannot reproduce here",
						command, strings.TrimSpace(line), strings.TrimSpace(first)).
						At(block.Where()))
					break
				}
			}
		}
		if ran == 0 {
			return []lint.Problem{lint.Skipf("SURF-301", "no sample named a verb in safeVerbs")}
		}
		return out
	},
}, {
	id: "SURF-302", severity: lint.Warn,
	title: "A version a document quotes is the version it ships",
	why: "A sample naming a version that has moved is the cheapest kind of wrong, and a " +
		"reader comparing their own output cannot tell which of you is stale.",
	check: func(l *Linter) []lint.Problem {
		set := l.set
		if set.Binary() == "" {
			return []lint.Problem{lint.Skipf("SURF-302", "no %s binary to ask", set.Tool())}
		}
		text, status := set.RunTool("version")
		if status != 0 {
			return []lint.Problem{lint.Skipf("SURF-302", "`%s version` did not run", set.Tool())}
		}
		current := map[string]bool{}
		for _, v := range semver.FindAllString(text, -1) {
			current[v] = true
		}
		if len(current) == 0 {
			return []lint.Problem{lint.Skipf("SURF-302", "the version output carries no x.y.z number")}
		}
		re := regexp.MustCompile(`(?m)^\s*(?:\$\s*)?(?:\S*/)?` + regexp.QuoteMeta(set.Tool()) + `\s+version\s*$`)
		var out []lint.Problem
		for _, name := range set.Docs() {
			body, _ := set.Text(name)
			for _, m := range re.FindAllStringIndex(body, -1) {
				end := min(m[1]+400, len(body))
				tail, _, _ := strings.Cut(body[m[1]:end], "```")
				var stale []string
				for _, q := range semver.FindAllString(tail, -1) {
					if !current[q] {
						stale = append(stale, q)
					}
				}
				if len(stale) > 0 {
					out = append(out, lint.Warnf("SURF-302",
						"a `%s version` sample quotes %s; the binary prints %s",
						set.Tool(), strings.Join(stale, ", "), strings.Join(lint.SortedKeys(current), ", ")).
						At(name+":"+strconv.Itoa(lint.Line(body, m[0]))))
				}
			}
		}
		return out
	},
}, {
	id: "SURF-303", severity: lint.Error,
	title: "The manual the binary carries is the manual in the tree",
	why: "A tool that prints its own manual is the copy a machine with no checkout " +
		"reads. When the binary is stale the two disagree, and the reader who cannot " +
		"see the tree is the one getting the wrong answer.",
	check: func(l *Linter) []lint.Problem {
		const manual = "MANUAL.md"
		set := l.set
		body, ok := set.Text(manual)
		if !ok {
			return []lint.Problem{lint.Skipf("SURF-303", "no %s to compare against", manual)}
		}
		if set.Binary() == "" {
			return []lint.Problem{lint.Skipf("SURF-303", "no %s binary to ask", set.Tool())}
		}
		if !set.Verbs()["manual"] {
			return []lint.Problem{lint.Skipf("SURF-303",
				"`%s manual` is not a verb this tool carries", set.Tool())}
		}
		printed, status := set.RunTool("manual")
		if status != 0 {
			return []lint.Problem{lint.Skipf("SURF-303", "`%s manual` did not run", set.Tool())}
		}
		// A terminal writer may end the stream with a newline the file does
		// not carry, and that is not drift anybody can act on.
		if strings.TrimRight(printed, "\n") == strings.TrimRight(body, "\n") {
			return nil
		}
		return []lint.Problem{lint.Errorf("SURF-303",
			"`%s manual` prints %s, and the binary is stale; rebuild it",
			set.Tool(), describeDrift(printed, body)).At(manual)}
	},
}}

// verbPaths reads the verb path a command line claims about the surface. A
// synopsis states one slot as alternatives, so it claims one path per
// alternative and everything else claims a single path.
//
// A path is the longest run of verbs the tool carries, plus the word after it.
// That word is the claim: everything before it resolved, so a name the binary
// rejects there is the document naming a command that is gone.
func verbPaths(words []string, carried map[string]bool, synopsis bool) [][]string {
	var path []string
	for _, word := range words {
		if alts := alternatives(word, path, carried, synopsis); alts != nil {
			var out [][]string
			for _, alt := range alts {
				out = append(out, append(append([]string{}, path...), alt))
			}
			return out
		}
		if !verbWord.MatchString(word) {
			break
		}
		path = append(path, word)
		if carried[strings.Join(path, " ")] {
			continue
		}
		break
	}
	if len(path) == 0 {
		return nil
	}
	return [][]string{path}
}

// shellFilters are the programs a documented pipeline pours into: the readers,
// the viewers and the clipboards. A part naming one of them, in a slot this
// tool does not carry, is the far side of a pipeline rather than an
// alternative, because alternatives fill one slot and are all the same tool's
// own verbs.
//
// A verb the tool does carry is never read this way, so a tool with an `ls` or
// a `sort` of its own keeps them in its synopsis. The cost is the other
// direction: a verb that was deleted and happens to share a name here is read
// as a pipeline and goes unreported. That is the trade a list like this makes,
// and it cannot be complete, so it is worth widening whenever a real pipeline
// is reported as a stale verb.
var shellFilters = map[string]bool{
	"ag": true, "awk": true, "base64": true, "bat": true, "cat": true,
	"column": true, "comm": true, "cut": true, "fold": true, "fzf": true,
	"grep": true, "head": true, "hexdump": true, "jq": true, "join": true,
	"less": true, "more": true, "nl": true, "od": true, "paste": true,
	"pbcopy": true, "rev": true, "rg": true, "sed": true, "shuf": true,
	"sort": true, "tac": true, "tail": true, "tee": true, "tr": true,
	"uniq": true, "wc": true, "wl-copy": true, "xargs": true, "xclip": true,
	"xxd": true, "yq": true,
}

// generated are the verbs a command-line framework synthesises. Neither is
// this project's own surface, so a hand-written section stating what the tool
// does is complete without them.
var generated = map[string]bool{"help": true, "completion": true}

// section locates the section a heading opens, returning the line the heading
// is on and the line after its last, so a block between the two belongs to it.
// A section runs until the next heading at its own level or above, which is
// what makes a subsection part of it.
//
// The heading is matched on the text it starts with, so `SPEC.md#3.1` finds
// `### 3.1 The host command surface` and survives that title being reworded.
func section(body, heading string) (first, last int, ok bool) {
	lines := docset.SplitLines(body)
	level := 0
	for i, line := range lines {
		depth := len(line) - len(strings.TrimLeft(line, "#"))
		if depth == 0 || !strings.HasPrefix(line[depth:], " ") {
			continue
		}
		text := strings.TrimSpace(line[depth:])
		if first == 0 {
			if text == heading || strings.HasPrefix(text, heading+" ") {
				first, level = i+1, depth
			}
			continue
		}
		if depth <= level {
			return first, i + 1, true
		}
	}
	if first == 0 {
		return 0, 0, false
	}
	return first, len(lines) + 1, true
}

// statesAlternatives reports whether a block states this tool's surface as a
// synopsis, by holding one line whose verb slot resolves as alternatives on
// its own evidence.
//
// A surface listing is a block of parallel lines, so the one line that proves
// itself settles the shape of the rest. That is what lets a slot worn down to
// `tool doctor|gone`, or to nothing the binary still carries, be read as the
// alternatives it is. Neither can prove itself alone, and both are what a
// synopsis decays into as verbs are deleted from it one at a time, which is
// where a check that reads each line by itself goes quiet exactly as drift
// begins.
func statesAlternatives(block docset.Block, tool string, carried map[string]bool) bool {
	for _, command := range block.Commands {
		words := strings.Fields(command)
		if len(words) == 0 || filepath.Base(words[0]) != tool {
			continue
		}
		// Alternatives are the only reading that claims more than one path.
		if len(verbPaths(words[1:], carried, false)) > 1 {
			return true
		}
	}
	return false
}

// alternatives reads a verb slot written as `a|b|c`, the compact form a spec
// section or a man page states a surface in. It returns nil for anything else.
//
// A pipeline is written with spaces around the bar, so strings.Fields has
// already broken one up before this sees it. What is left is a bar with no
// spaces, which is either a synopsis or a pipeline somebody wrote tight. Two
// things separate them. A part naming a program that reads a stream is the far
// side of a pipeline, whatever else the slot holds. Otherwise two of the parts
// naming verbs this tool carries proves a synopsis by itself, and a block that
// holds such a line elsewhere proves it for the lines that cannot.
func alternatives(word string, path []string, carried map[string]bool, synopsis bool) []string {
	if !strings.Contains(word, "|") {
		return nil
	}
	prefix := strings.Join(path, " ")
	parts := strings.Split(word, "|")
	known := 0
	for _, part := range parts {
		if !verbWord.MatchString(part) {
			return nil
		}
		name := part
		if prefix != "" {
			name = prefix + " " + part
		}
		if carried[name] {
			known++
			continue
		}
		if shellFilters[part] {
			return nil
		}
	}
	if known < 2 && !synopsis {
		return nil
	}
	return parts
}

func nonEmpty(lines []string) []string {
	var out []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimRight(l, " \t\r"))
		}
	}
	return out
}

// wholeWord matches a word the documents have to name somewhere. It is built
// from a verb, so it cannot be a package-level constant; a run compiles a few
// dozen at most, and a cache for that would buy nothing measurable while
// adding shared mutable state to a package whose rules are otherwise free of it.
func wholeWord(w string) *regexp.Regexp {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(w) + `\b`)
}

var elision = regexp.MustCompile(`…|\.\.\.`)

// matches reports whether a recorded line still matches what the command
// printed.
//
// An elision stands for whatever the command prints there, which is how a
// document keeps a sample true across a number that moves: a byte count, a
// duration, a path under a temporary directory. Everything either side of it
// still has to match, so the elision buys drift in one place rather than
// turning the whole line off.
func matches(recorded, actual string) bool {
	if !strings.Contains(recorded, "…") && !strings.Contains(recorded, "...") {
		return recorded == actual
	}
	parts := elision.Split(recorded, -1)
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		quoted = append(quoted, regexp.QuoteMeta(p))
	}
	re, err := regexp.Compile(`(?s)\A` + strings.Join(quoted, ".*") + `\z`)
	return err == nil && re.MatchString(actual)
}

// describeDrift says how the printed document differs from the file, so the
// finding is actionable without running a diff. The first differing line is
// where a reader would start looking, and where a rebuild would show its work.
func describeDrift(printed, file string) string {
	p, f := docset.SplitLines(printed), docset.SplitLines(file)
	for i := 0; i < len(p) && i < len(f); i++ {
		if p[i] != f[i] {
			return fmt.Sprintf("a different line %d: %q, where the file has %q",
				i+1, lint.Truncate(p[i], 60), lint.Truncate(f[i], 60))
		}
	}
	return fmt.Sprintf("%d lines, where the file has %d", len(p), len(f))
}
