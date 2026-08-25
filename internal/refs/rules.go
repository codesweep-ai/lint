package refs

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/codesweep-ai/lint/internal/docset"
	"github.com/codesweep-ai/lint/internal/lint"
)

var (
	pathToken = regexp.MustCompile("[`\\[(]([\\w.@-]+/[\\w./@-]+)[`\\])]")
	citation  = regexp.MustCompile(`(\w+\.md)\s*(?:§|section\s+)([\d.]+)`)
	// The same citation as it appears in source, where the document is often
	// left out because there is only one that numbers anything.
	sourceCitation = regexp.MustCompile(`(?:(\w+\.md)\s*)?§\s?([\d.]+)`)
	commandDash    = regexp.MustCompile(`command -v ([\w.-]+)`)
	shortExt       = regexp.MustCompile(`^[a-z0-9]{1,6}$`)
)

var rules = []rule{{
	id: "REF-101", severity: lint.Error,
	title: "Every repository path a document names exists",
	why: "A file that moves leaves the documents pointing at where it was, and nothing " +
		"fails. The reference is then wrong until somebody happens to read that line. " +
		"Every tracked Markdown file is read, not only the document set: a nested " +
		"README makes the same claim, and markdownSkip says which trees make none.",
	check: func(l *Linter) []lint.Problem {
		set := l.set
		repo := set.Repo()
		roots := map[string]bool{}
		for _, p := range repo.Tracked() {
			roots[strings.SplitN(p, "/", 2)[0]] = true
		}
		tracked := map[string]bool{}
		for _, p := range repo.Tracked() {
			tracked[p] = true
		}
		var out []lint.Problem
		seen := map[string]bool{}
		for _, name := range set.Markdown() {
			body, _ := set.Text(name)
			for _, m := range pathToken.FindAllStringSubmatch(body, -1) {
				token := strings.TrimRight(m[1], ".,;:")
				head, _, _ := strings.Cut(token, "/")
				if !roots[head] || strings.HasPrefix(token, "http") {
					continue
				}
				if repo.Exists(token) || tracked[token] {
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
				out = append(out, lint.Errorf("REF-101",
					"%s is named here and does not exist", token).At(name))
			}
		}
		return out
	},
}, {
	id: "REF-102", severity: lint.Error,
	title: "Every section citation resolves",
	why: "A citation like SPEC.md §7.2 is useful only while §7.2 says what it said. " +
		"Renumbering breaks every one at once, and a stale citation sends a reader to a " +
		"rule that now means something else.",
	check: func(l *Linter) []lint.Problem {
		set := l.set
		var out []lint.Problem
		seen := map[string]bool{}
		for _, name := range set.Docs() {
			body, _ := set.Text(name)
			for _, m := range citation.FindAllStringSubmatchIndex(body, -1) {
				target := body[m[2]:m[3]]
				section := strings.TrimRight(body[m[4]:m[5]], ".")
				text, ok := set.Text(target)
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
				out = append(out, lint.Errorf("REF-102", "%s has no section %s", target, section).
					At(name+":"+strconv.Itoa(lint.Line(body, m[0]))))
			}
		}
		return out
	},
}, {
	id: "REF-103", severity: lint.Error,
	title: "Every section citation in the source resolves",
	why: "A comment reading `SPEC.md §7.2` is useful only while §7.2 says what it " +
		"said. Renumbering the spec breaks every one at once, silently, and a stale " +
		"citation sends its next reader to a rule that now means something else. A " +
		"bare § is read against the spec, which is the document that numbers its " +
		"sections.",
	check: func(l *Linter) []lint.Problem {
		set := l.set
		fallback := set.CitedByDefault()
		var out []lint.Problem
		seen := map[string]bool{}
		set.AllSource(func(path, body string) {
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
				text, ok := set.Text(target)
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
				out = append(out, lint.Errorf("REF-103", "%s has no section %s", target, section).
					At(path+":"+strconv.Itoa(lint.Line(body, m[0]))))
			}
		})
		return out
	},
}, {
	id: "REF-201", severity: lint.Warn,
	title: "A block a reader copies names nothing they lack",
	why: "A sequence introduced as runnable end to end, opening on a repo the reader was " +
		"never given, fails on its first line. Say the path is theirs to supply, or give them one.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		seen := map[string]bool{}
		for _, block := range l.set.Blocks() {
			if !docset.IsShell(block.Lang) {
				continue
			}
			for _, command := range block.Commands {
				var hits []string
				for _, p := range docset.Placeholders {
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
					out = append(out, lint.Warnf("REF-201", "%s", msg).At(block.Where()))
				}
			}
		}
		return out
	},
}, {
	id: "REF-202", severity: lint.Error,
	title: "Every tool the build needs is named in a document",
	why: "A contributor told to run one command, on a machine missing a tool nothing named, " +
		"meets the gate as a failure rather than as a setup step. The answer lives in an " +
		"error message only a failed run prints.",
	check: func(l *Linter) []lint.Problem {
		set := l.set
		prose := set.AllText()
		ok := map[string]bool{set.Tool(): true}
		for _, t := range l.cfg.PrereqOK {
			ok[t] = true
		}
		needed := map[string]string{}
		set.BuildFiles(func(path, body string) {
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
			return []lint.Problem{lint.Skipf("REF-202", "the build shells out to nothing it checks for")}
		}
		var out []lint.Problem
		for _, t := range lint.SortedKeys(needed) {
			if namesTool(t).MatchString(prose) {
				continue
			}
			out = append(out, lint.Errorf("REF-202",
				"%s has to be installed for the build and no document names it", t).At(needed[t]))
		}
		return out
	},
}, {
	id: "REF-301", severity: lint.Warn,
	title: "The manual answers the automated caller",
	why: "An agent driving the tool needs what a human infers: which commands are " +
		"non-interactive, which output is machine-readable, and what touches the network or " +
		"the filesystem.",
	check: func(l *Linter) []lint.Problem {
		manual, ok := l.set.Text("MANUAL.md")
		if !ok {
			return []lint.Problem{lint.Skipf("REF-301", "no MANUAL.md in the document set")}
		}
		if strings.Contains(strings.ToLower(manual), strings.ToLower(l.cfg.AgentSection)) {
			return nil
		}
		return []lint.Problem{lint.Warnf("REF-301", "MANUAL.md has no %q section",
			l.cfg.AgentSection).At("MANUAL.md")}
	},
}, {
	id: "REF-302", severity: lint.Error,
	title: "The router names every document in the set",
	why: "An agent reads the AGENTS.md nearest the file it is editing and goes no further. " +
		"A document the router omits is a document it never opens.",
	check: func(l *Linter) []lint.Problem {
		router, ok := l.set.Text("AGENTS.md")
		if !ok {
			return []lint.Problem{lint.Skipf("REF-302", "no AGENTS.md at the root")}
		}
		var out []lint.Problem
		for _, n := range l.set.Docs() {
			if n != "AGENTS.md" && !strings.Contains(router, n) {
				out = append(out, lint.Errorf("REF-302", "AGENTS.md does not name %s", n).At("AGENTS.md"))
			}
		}
		return out
	},
}, {
	id: "REF-303", severity: lint.Error,
	title: "Every issue this repository cites has a record",
	why: "An identifier in a comment or a help string is a promise that somewhere there is " +
		"an account of why the code is shaped that way. A citation whose record is gone " +
		"sends its reader looking for something nobody can find, and nothing else notices: " +
		"a ledger reports on itself, and the citations live outside it.",
	check: func(l *Linter) []lint.Problem {
		set := l.set
		prefix, ok := ledgerPrefix(l)
		if !ok {
			return []lint.Problem{lint.Skipf("REF-303", "no ledger/ledger.json at the root")}
		}
		have := map[string]bool{}
		for _, path := range set.Repo().Tracked() {
			if m := ledgerRecord.FindStringSubmatch(path); m != nil {
				have[m[1]] = true
			}
		}
		if len(have) == 0 {
			return []lint.Problem{lint.Skipf("REF-303", "the ledger holds no records to cite")}
		}
		cite := regexp.MustCompile(`\b(` + regexp.QuoteMeta(prefix) + `-\d+)\b`)
		// Where each missing id was first seen, so the finding names a file to
		// open rather than an identifier to hunt for.
		at := map[string]string{}
		note := func(path, body string) {
			if strings.HasPrefix(path, "ledger/") {
				return
			}
			for _, m := range cite.FindAllStringSubmatch(body, -1) {
				if !have[m[1]] {
					if _, seen := at[m[1]]; !seen {
						at[m[1]] = path
					}
				}
			}
		}
		set.Source(note)
		for _, doc := range set.Docs() {
			body, _ := set.Text(doc)
			note(doc, body)
		}
		var out []lint.Problem
		for _, id := range lint.SortedKeys(at) {
			out = append(out, lint.Errorf("REF-303",
				"%s is cited here and the ledger holds no such record", id).At(at[id]))
		}
		return out
	},
}}

// ledgerRecord matches one record's file, whose name is the identifier.
var ledgerRecord = regexp.MustCompile(`^ledger/issues/([A-Z]+-\d+)\.json$`)

// ledgerPrefix reads the identifier prefix the ledger mints, so a citation is
// recognised by the shape this repository actually uses.
func ledgerPrefix(l *Linter) (string, bool) {
	body, ok := l.set.Repo().Read("ledger/ledger.json")
	if !ok {
		return "", false
	}
	var meta struct {
		IDPrefix string `json:"idPrefix"`
	}
	if json.Unmarshal([]byte(body), &meta) != nil || meta.IDPrefix == "" {
		return "", false
	}
	return meta.IDPrefix, true
}

// The two patterns below are built from a section number or a tool name, so
// neither can be a package-level constant. They are compiled where they are
// used: a run builds a few dozen at most, and a cache for that would buy
// nothing measurable while adding shared mutable state to a package whose
// rules are otherwise free of it.

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
