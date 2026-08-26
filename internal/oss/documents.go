package oss

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/codesweep-ai/lint/internal/lint"
)

var (
	mdLink       = regexp.MustCompile(`\[[^\]]*\]\(<?([^)\s>]+)>?(?:\s+"[^"]*")?\)`)
	anchorLink   = regexp.MustCompile(`\[[^\]]*\]\(<?([^)\s>]*#[^)\s>]+)>?\)`)
	headingLine  = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*$`)
	codeSpan     = regexp.MustCompile("`([^`]*)`")
	linkInHead   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	emphasisMark = regexp.MustCompile(`[*_]`)
	notSlugChar  = regexp.MustCompile(`[^\w\- ]`)
	ciBadge      = regexp.MustCompile(`https://github\.com/([\w.-]+/[\w.-]+)/actions/workflows/[\w.-]+`)
	boldClaim    = regexp.MustCompile(`^> \*\*`)

	// Language that invites a security report, and the shapes an answer takes.
	reportsSecurity = regexp.MustCompile(`(?i)security[- ](?:issue|sensitive|bug|report|vulnerabilit)|vulnerabilit`)
	namesAChannel   = regexp.MustCompile(`(?i)https?://|mailto:|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}` +
		`|private vulnerability reporting|security advisor`)

	// The conventions CONTRIBUTING.md has to state, and the headings that
	// carry them. Constant, so they are compiled once rather than per run.
	contribHeading = map[string]*regexp.Regexp{
		"Commits": regexp.MustCompile(`(?im)^##+\s*.*\bCommits\b`),
		"Writing": regexp.MustCompile(`(?im)^##+\s*.*\bWriting\b`),
	}
	// The gate a contributor is told to run. `ci` counts as well as `check`:
	// a project whose wider target mirrors the workflow points its readers at
	// that one, and naming it is naming the gate.
	namesTheGate = regexp.MustCompile(`\bmake (?:check|ci)\b|\bnpm (run )?(?:check|ci)\b|scripts/check`)
	namesTrailer = regexp.MustCompile(`(?i)trailer`)

	// How a change gets in. A document that never says leaves a contributor
	// to guess, and the guess is usually to do nothing.
	submitsAChange = regexp.MustCompile(`(?i)pull request|merge request|patch(?:es)? to|send.{0,12}patch`)
	namesAForkStep = regexp.MustCompile(`(?i)\bfork\b|\bbranch (?:from|off)\b|create a branch`)
	namesATracker  = regexp.MustCompile(`(?i)(?:github|gitlab) issue|issue tracker|open an issue|` +
		`file (?:a|an) (?:bug|issue)|/issues\b|report (?:a )?bug`)
	namesTheTerms = regexp.MustCompile(`(?i)under the .{0,24}licen[cs]e|licen[cs]e this project|` +
		`inbound|developer certificate of origin|\bDCO\b|sign-?off|\bCLA\b|contributor licen`)
	namesAIPolicy = regexp.MustCompile(`(?im)^##+\s*.*\b(?:AI|agent|model|LLM)[- ]?(?:assisted|generated|written|use)`)

	// The Writing section delegates rather than restating the checks. A
	// second copy of a rule set drifts, and a document that states a
	// threshold the linter does not hold is worse than one that states none.
	citesTheLinter = regexp.MustCompile(`cs-lint prose --explain`)

	// A table's header row, used to spot the same table in two documents.
	tableHeader = regexp.MustCompile(`^\|(.+)\|\s*$`)

	// A repository-relative path with an extension. Anything with a scheme, a
	// glob, a variable or a leading slash is not one. The boundaries the
	// original expressed as lookarounds are checked beside the match.
	pathish = regexp.MustCompile(`[\w.-]+/[\w./-]*[\w-]\.` +
		`(?:json|yaml|html|mjs|toml|cast|svg|md|py|sh|js|ts|go|yml|css|txt|gif|png)`)
)

// slugs returns the anchors GitHub mints for a document's headings.
func slugs(body string) map[string]bool {
	out := map[string]bool{}
	for line := range strings.SplitSeq(body, "\n") {
		m := headingLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		text := codeSpan.ReplaceAllString(m[2], "$1")
		text = linkInHead.ReplaceAllString(text, "$1")
		text = emphasisMark.ReplaceAllString(text, "")
		text = notSlugChar.ReplaceAllString(strings.ToLower(text), "")
		out[strings.ReplaceAll(strings.TrimSpace(text), " ", "-")] = true
	}
	return out
}

func isExternal(target string) bool {
	for _, scheme := range []string{"http://", "https://", "mailto:", "#", "tel:", "data:"} {
		if strings.HasPrefix(target, scheme) {
			return true
		}
	}
	return false
}

var documentRules = []rule{{
	id: "OSS-201", severity: lint.Error,
	title: "The document set is complete",
	why: "Each document answers one question a reader arrives with. A missing one means " +
		"that question is answered nowhere, or answered inside a document whose readers " +
		"did not ask it.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		for _, d := range l.allDocs() {
			if !l.has(d) {
				out = append(out, lint.Errorf("OSS-201", "%s is missing", d))
			}
		}
		return out
	},
}, {
	id: "OSS-202", severity: lint.Error,
	title: "AGENTS.md routes to every document",
	why: "An agent harness discovers AGENTS.md by filename, so it is read whether or not " +
		"anyone points at it. A router that omits a document is why that document goes unread.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read("AGENTS.md")
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-202", "no AGENTS.md")}
		}
		var missing []string
		for _, d := range l.allDocs() {
			if d != "AGENTS.md" && !strings.Contains(body, d) {
				missing = append(missing, d)
			}
		}
		var out []lint.Problem
		if len(missing) > 0 {
			out = append(out, lint.Errorf("OSS-202", "AGENTS.md does not route to %s",
				strings.Join(missing, ", ")))
		}
		if l.keepsLedger() && !strings.Contains(body, "ledger/AGENTS.md") {
			out = append(out, lint.Errorf("OSS-202", "AGENTS.md does not route to ledger/AGENTS.md"))
		}
		return out
	},
}, {
	id: "OSS-203", severity: lint.Warn,
	title: "AGENTS.md routes and holds no knowledge of its own",
	why: "A second copy of a fact goes stale. The file exists to point at the documents, " +
		"so anything it explains is something a document should.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read("AGENTS.md")
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-203", "no AGENTS.md")}
		}
		var out []lint.Problem
		if n := len(strings.Split(strings.TrimRight(body, "\n"), "\n")); n > 40 {
			out = append(out, lint.Warnf("OSS-203", "AGENTS.md is %d lines; a router is under 40", n))
		}
		if !strings.Contains(body, "routes") {
			out = append(out, lint.Warnf("OSS-203",
				"AGENTS.md does not say that it routes, so a reader treats it as a source"))
		}
		return out
	},
}, {
	id: "OSS-204", severity: lint.Warn,
	title: "The README opens the way the family's do",
	why: "A stranger reaching the repository from a search result decides in under a " +
		"minute. The name, one bold sentence and the badge block are what they read.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read("README.md")
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-204", "no README.md")}
		}
		lines := strings.Split(body, "\n")
		if len(lines) > 16 {
			lines = lines[:16]
		}
		var out []lint.Problem
		if len(lines) == 0 || !strings.HasPrefix(lines[0], "# ") {
			out = append(out, lint.Warnf("OSS-204", "README.md does not open with an H1"))
		} else {
			name := strings.TrimSpace(lines[0][2:])
			slug := l.slug()
			if i := strings.LastIndex(slug, "/"); i >= 0 {
				slug = slug[i+1:]
			}
			if slug != "" && !strings.EqualFold(name, slug) && !strings.EqualFold(name, l.project()) {
				out = append(out, lint.Warnf("OSS-204",
					"the README's H1 is %q, and the repository is named %q", name, slug))
			}
		}
		claim := slices.ContainsFunc(lines, boldClaim.MatchString)
		if !claim {
			out = append(out, lint.Warnf("OSS-204", "no bold one-sentence claim in the opening lines"))
		}
		if !strings.Contains(strings.ToLower(body), "img.shields.io/badge/license") {
			out = append(out, lint.Warnf("OSS-204", "no licence badge"))
		}
		return out
	},
}, {
	id: "OSS-205", severity: lint.Error,
	title: "The README's CI badge points at this repository",
	why: "A badge copied from a sibling reports that sibling's build. It stays green " +
		"while this one is broken, which is worse than having no badge.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read("README.md")
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-205", "no README.md")}
		}
		found := ciBadge.FindAllStringSubmatch(body, -1)
		if len(found) == 0 {
			return []lint.Problem{lint.Errorf("OSS-205", "the README carries no CI badge")}
		}
		slug := l.slug()
		if slug == "" {
			return nil
		}
		wrong := map[string]bool{}
		for _, m := range found {
			if !strings.EqualFold(m[1], slug) {
				wrong[m[1]] = true
			}
		}
		if len(wrong) > 0 {
			return []lint.Problem{lint.Errorf("OSS-205", "the CI badge names %s, not %s",
				lint.SortedKeys(wrong)[0], slug)}
		}
		return nil
	},
}, {
	id: "OSS-206", severity: lint.Error, needsTree: true,
	title: "Every relative link resolves",
	why: "A link to a file that is not there is the classic residue of splitting a " +
		"document, and it is the first thing a new reader clicks.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		l.scannable(func(path, body string) {
			if !strings.HasSuffix(path, ".md") {
				return
			}
			base := filepath.Dir(l.repo.Path(path))
			for _, m := range mdLink.FindAllStringSubmatchIndex(body, -1) {
				target := body[m[2]:m[3]]
				if isExternal(target) {
					continue
				}
				filePart := target
				if i := strings.Index(filePart, "#"); i >= 0 {
					filePart = filePart[:i]
				}
				if filePart == "" {
					continue
				}
				if _, err := os.Stat(filepath.Join(base, filePart)); err != nil {
					out = append(out, lint.Errorf("OSS-206", "link to %s does not resolve", target).
						At(lint.At(path, body, m[0])))
				}
			}
		})
		return out
	},
}, {
	id: "OSS-207", severity: lint.Error, needsTree: true,
	title: "Every anchor resolves to a heading",
	why: "Renumbering a section silently breaks every link into it. Nothing about the " +
		"link looks wrong, so only this check finds it.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		cache := map[string]map[string]bool{}
		l.scannable(func(path, body string) {
			if !strings.HasSuffix(path, ".md") {
				return
			}
			base := filepath.Dir(l.repo.Path(path))
			for _, m := range anchorLink.FindAllStringSubmatchIndex(body, -1) {
				target := body[m[2]:m[3]]
				if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
					continue
				}
				filePart, anchor, _ := strings.Cut(target, "#")
				doc := l.repo.Path(path)
				if filePart != "" {
					doc = filepath.Join(base, filePart)
				}
				known, seen := cache[doc]
				if !seen {
					if b, err := os.ReadFile(doc); err == nil {
						known = slugs(string(b))
					}
					cache[doc] = known
				}
				if known == nil || known[strings.ToLower(anchor)] {
					continue
				}
				where := filePart
				if where == "" {
					where = path
				}
				out = append(out, lint.Errorf("OSS-207",
					"anchor #%s matches no heading in %s", anchor, where).At(lint.At(path, body, m[0])))
			}
		})
		return out
	},
}, {
	id: "OSS-208", severity: lint.Error,
	title: "CONTRIBUTING states the conventions it expects",
	why: "A contributor who cannot find the commit rule invents one, and a reviewer then " +
		"argues for a convention nobody wrote down.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read("CONTRIBUTING.md")
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-208", "no CONTRIBUTING.md")}
		}
		var out []lint.Problem
		for _, heading := range []string{"Commits", "Writing"} {
			if !contribHeading[heading].MatchString(body) {
				out = append(out, lint.Errorf("OSS-208", "CONTRIBUTING.md has no %s section", heading))
			}
		}
		if !namesTheGate.MatchString(body) {
			out = append(out, lint.Errorf("OSS-208",
				"CONTRIBUTING.md never names the one command to run before pushing"))
		}
		if !namesTrailer.MatchString(body) {
			out = append(out, lint.Errorf("OSS-208",
				"CONTRIBUTING.md states no rule about git trailers, so an agent-written "+
					"commit keeps whatever its harness appends"))
		}
		return out
	},
}, {
	id: "OSS-209", severity: lint.Warn,
	title: "The release archive ships the documents",
	why: "Someone who downloads the release and never visits the repository has only " +
		"what the archive carries.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.goreleaser()
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-209", "no goreleaser manifest")}
		}
		var missing []string
		for _, d := range append(l.allDocs(), "LICENSE") {
			if d != "AGENTS.md" && !strings.Contains(body, d) {
				missing = append(missing, d)
			}
		}
		if len(missing) > 0 {
			return []lint.Problem{lint.Warnf("OSS-209", "the release archive omits %s",
				strings.Join(missing, ", "))}
		}
		return nil
	},
}, {
	id: "OSS-210", severity: lint.Warn, needsTree: true,
	title: "No stray document at the root",
	why: "A root file outside the set is a document with no stated job, and the reader " +
		"has no way to know which question it answers.",
	check: func(l *Linter) []lint.Problem {
		known := map[string]bool{"CHANGELOG.md": true, "NOTICE.md": true,
			"CODE_OF_CONDUCT.md": true, "SECURITY.md": true}
		for _, d := range l.allDocs() {
			known[d] = true
		}
		var stray []string
		for _, p := range l.repo.Tracked() {
			if strings.HasSuffix(p, ".md") && !strings.Contains(p, "/") && !known[p] {
				stray = append(stray, p)
			}
		}
		if len(stray) > 0 {
			return []lint.Problem{lint.Warnf("OSS-210", "root documents outside the set: %s",
				strings.Join(stray, ", "))}
		}
		return nil
	},
}, {
	id: "OSS-211", severity: lint.Warn,
	title: "Every path a manifest names exists",
	why: "A file copied from a sibling repository passes every other check and still " +
		"describes a project that is not this one. The paths it names are what gives it " +
		"away. Prose is left to OSS-206, because a document names illustrative paths on purpose.",
	check: func(l *Linter) []lint.Problem {
		var out []lint.Problem
		watched := []string{".gitattributes", ".goreleaser.yaml", ".goreleaser.yml",
			".github/workflows/release.yml"}
		for _, path := range watched {
			body, ok := l.read(path)
			if !ok {
				continue
			}
			seen := map[string]bool{}
			for _, m := range pathish.FindAllStringIndex(body, -1) {
				// The original bounded the match with lookarounds: a path does
				// not continue a word, a URL, or a longer path.
				if m[0] > 0 && strings.ContainsRune(`abcdefghijklmnopqrstuvwxyz`+
					`ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_/@:.-`, rune(body[m[0]-1])) {
					continue
				}
				if m[1] < len(body) && (lint.IsWordByte(body[m[1]]) || body[m[1]] == '-') {
					continue
				}
				target := body[m[0]:m[1]]
				if seen[target] || strings.Contains(target, "://") || strings.Contains(target, "*") {
					continue
				}
				seen[target] = true
				if l.repo.Exists(target) {
					continue
				}
				out = append(out, lint.Warnf("OSS-211", "names %s, which is not here", target).
					At(lint.At(path, body, m[0])))
			}
		}
		if len(out) > 12 {
			out = out[:12]
		}
		return out
	},
}, {
	id: "OSS-212", severity: lint.Error,
	title: "A security report has somewhere to go",
	why: "\"Ask for a private contact\" sends a reporter to the public tracker to request " +
		"the private channel they were trying to use. A promise with nothing behind it is " +
		"worse than saying nothing, because it reads as an answer.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read("CONTRIBUTING.md")
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-212", "no CONTRIBUTING.md")}
		}
		if l.has("SECURITY.md") {
			return nil
		}
		// Flattened first: a wrapped paragraph puts a newline inside the very
		// phrase that names the channel.
		var invites []string
		for p := range strings.SplitSeq(body, "\n\n") {
			flat := strings.Join(strings.Fields(p), " ")
			if reportsSecurity.MatchString(flat) {
				invites = append(invites, flat)
			}
		}
		if len(invites) == 0 {
			return []lint.Problem{lint.Warnf("OSS-212",
				"nothing says where a security report goes, so it will arrive in the public tracker")}
		}
		if slices.ContainsFunc(invites, namesAChannel.MatchString) {
			return nil
		}
		quote := invites[0]
		if len(quote) > 110 {
			quote = quote[:110]
		}
		return []lint.Problem{lint.Errorf("OSS-212",
			"a security report is invited but no channel is named: %s", quote)}
	},
}, {
	id: "OSS-213", severity: lint.Warn,
	title: "CONTRIBUTING says how a change gets in",
	why: "A stranger who wants to fix a typo has to be able to learn what to do next. A " +
		"document that states every convention and never states the process reads as " +
		"published rather than open, and the contribution that does not arrive is invisible.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read("CONTRIBUTING.md")
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-213", "no CONTRIBUTING.md")}
		}
		var missing []string
		if !submitsAChange.MatchString(body) {
			missing = append(missing, "how a change is submitted")
		}
		if !namesAForkStep.MatchString(body) {
			missing = append(missing, "where the work starts, as a fork or a branch")
		}
		if len(missing) > 0 {
			return []lint.Problem{lint.Warnf("OSS-213",
				"CONTRIBUTING.md never says %s", strings.Join(missing, ", nor "))}
		}
		return nil
	},
}, {
	id: "OSS-214", severity: lint.Warn,
	title: "An ordinary bug report has somewhere to go",
	why: "A ledger committed in the tree is the maintainers' own record, not a place a " +
		"stranger can file. OSS-212 covers the private channel for a vulnerability. This " +
		"covers the public one for everything else, which is the commoner report by far.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read("CONTRIBUTING.md")
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-214", "no CONTRIBUTING.md")}
		}
		readme, _ := l.read("README.md")
		if namesATracker.MatchString(body) || namesATracker.MatchString(readme) {
			return nil
		}
		return []lint.Problem{lint.Warnf("OSS-214",
			"neither CONTRIBUTING.md nor README.md names where an ordinary bug report goes")}
	},
}, {
	id: "OSS-215", severity: lint.Warn,
	title: "CONTRIBUTING states the terms a contribution is accepted under",
	why: "A contributor is granting a licence whether or not anybody says so. Stating it " +
		"costs one sentence and removes the question a careful contributor stops to ask.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read("CONTRIBUTING.md")
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-215", "no CONTRIBUTING.md")}
		}
		// Flattened first: the sentence that states the terms is usually
		// wrapped, and the licence name lands on the next line.
		if namesTheTerms.MatchString(strings.Join(strings.Fields(body), " ")) {
			return nil
		}
		return []lint.Problem{lint.Warnf("OSS-215",
			"CONTRIBUTING.md never says what terms a contribution is accepted under")}
	},
}, {
	id: "OSS-216", severity: lint.Warn,
	title: "CONTRIBUTING states a policy for AI-assisted contributions",
	why: "A repository whose history carries Co-Authored-By trailers owes its readers the " +
		"policy behind them. Saying nothing leaves a maintainer arguing it case by case, " +
		"and leaves an agent working here with no instruction it can follow.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read("CONTRIBUTING.md")
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-216", "no CONTRIBUTING.md")}
		}
		if namesAIPolicy.MatchString(body) {
			return nil
		}
		return []lint.Problem{lint.Warnf("OSS-216",
			"CONTRIBUTING.md has no section stating a policy for AI-assisted contributions")}
	},
}, {
	id: "OSS-217", severity: lint.Warn,
	title: "The writing rules are cited rather than copied",
	why: "A restated rule set is a second copy that nothing keeps in sync. Six sibling " +
		"projects each restating this one produced six different lists, and two of them " +
		"stated a threshold the linter they shipped did not hold. Cite the linter instead.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read("CONTRIBUTING.md")
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-217", "no CONTRIBUTING.md")}
		}
		start, length := sectionAt(body, contribHeading["Writing"])
		if start < 0 {
			return []lint.Problem{lint.Skipf("OSS-217", "CONTRIBUTING.md has no Writing section")}
		}
		section := body[start : start+length]
		var out []lint.Problem
		if !citesTheLinter.MatchString(section) {
			out = append(out, lint.Warnf("OSS-217",
				"the Writing section never points at `cs-lint prose --explain`, "+
					"so a reader cannot tell which rules are enforced"))
		}
		if n := strings.Count(section, "\n"); n > maxWritingLines {
			out = append(out, lint.Warnf("OSS-217",
				"the Writing section is %d lines (max %d): it is restating the linter "+
					"rather than citing it", n, maxWritingLines))
		}
		return out
	},
}, {
	id: "OSS-218", severity: lint.Warn,
	title: "CONTRIBUTING does not restate a table SPEC already carries",
	why: "A fact lives in one document and the others link to it. Two copies of one table " +
		"drift, and the copy a contributor reads is rarely the copy that was updated.",
	check: func(l *Linter) []lint.Problem {
		contrib, ok := l.read("CONTRIBUTING.md")
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-218", "no CONTRIBUTING.md")}
		}
		spec, ok := l.read("SPEC.md")
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-218", "no SPEC.md")}
		}
		specHeads := tableHeaders(spec)
		var out []lint.Problem
		for head := range tableHeaders(contrib) {
			if specHeads[head] {
				out = append(out, lint.Warnf("OSS-218",
					"the table `%s` is in SPEC.md too; keep one and link to it", head))
			}
		}
		return out
	},
}}

// maxWritingLines is where a Writing section stops citing the linter and
// starts being a second copy of it. Long enough for the principles that carry
// the voice and the pointer, short enough that a restated rule set trips it.
const maxWritingLines = 60

// sectionAt returns the offset and length of the section a heading opens,
// running to the next heading of the same level or to the end.
func sectionAt(body string, heading *regexp.Regexp) (int, int) {
	loc := heading.FindStringIndex(body)
	if loc == nil {
		return -1, 0
	}
	rest := body[loc[1]:]
	if next := regexp.MustCompile(`(?m)^##\s`).FindStringIndex(rest); next != nil {
		return loc[0], loc[1] - loc[0] + next[0]
	}
	return loc[0], len(body) - loc[0]
}

// tableHeaders returns the header row of every Markdown table in a document,
// normalised, which is what identifies one table as a copy of another.
func tableHeaders(body string) map[string]bool {
	out := map[string]bool{}
	lines := strings.Split(body, "\n")
	for i := 0; i+1 < len(lines); i++ {
		m := tableHeader.FindStringSubmatch(strings.TrimSpace(lines[i]))
		if m == nil || !strings.Contains(lines[i+1], "---") {
			continue
		}
		var cells []string
		for c := range strings.SplitSeq(m[1], "|") {
			if c = strings.ToLower(strings.TrimSpace(c)); c != "" {
				cells = append(cells, c)
			}
		}
		if len(cells) > 1 {
			out[strings.Join(cells, " | ")] = true
		}
	}
	return out
}
