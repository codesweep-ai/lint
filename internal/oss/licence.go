package oss

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	lintdoc "github.com/codesweep-ai/lint"
	"github.com/codesweep-ai/lint/internal/lint"
)

var licenceNames = []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "LICENCE"}

func (l *Linter) licenceText() (string, bool) {
	for _, name := range licenceNames {
		if body, ok := l.read(name); ok {
			return body, true
		}
	}
	return "", false
}

// licenceName is the licence file this repository carries, for a report that
// names the file a reader has rather than the one the check looked for first.
func (l *Linter) licenceName() string {
	for _, name := range licenceNames {
		if _, ok := l.read(name); ok {
			return name
		}
	}
	return "LICENSE"
}

var (
	licenceSection = regexp.MustCompile(`(?im)^##+\s*Licen[cs]e\b`)
	vendorLicence  = regexp.MustCompile(`(?i)/(LICENSE|LICENCE|NOTICE|COPYING)`)
	rootLicence    = regexp.MustCompile(`(?i)^(LICENSE|LICENCE|COPYING)(\.\w+)?$`)
)

// apacheMarkers are the lines that identify a file as the Apache 2.0 text at
// all, as opposed to a summary, a link, or a different licence entirely. OSS-102
// reports what is missing; OSS-107 then holds the text to the byte.
var apacheMarkers = []string{
	"Apache License",
	"Version 2.0, January 2004",
	"TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION",
}

// noticeProjectPrefix is what a NOTICE names itself. The family shares one
// word so that a NOTICE copied from a sibling and never edited is visible as
// what it is, while the rest of the line stays the project's own.
const noticeProjectPrefix = "Codesweep"

// referenceLine returns the one-based line of an embedded reference text.
func referenceLine(text string, n int) string {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if n < 1 || n > len(lines) {
		return ""
	}
	return lines[n-1]
}

// firstDifference returns the one-based line where two texts diverge, with the
// line each carries there. A byte-identical check is only worth failing on if
// it says where to look; "the file differs" sends a reader to diff it by hand.
//
// Line 0 means the walk found no differing line, which happens when the texts
// differ only in how the last one ends. That case is reported on its own,
// because an invisible difference described as a line number reads as a bug in
// the check.
func firstDifference(got, want string) (line int, gotLine, wantLine string) {
	g := strings.Split(got, "\n")
	w := strings.Split(want, "\n")
	for i := range max(len(g), len(w)) {
		var gl, wl string
		if i < len(g) {
			gl = g[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if gl != wl {
			return i + 1, gl, wl
		}
	}
	return 0, "", ""
}

// verbatim reports how a file differs from the text the tool carries, or
// nothing where it does not differ at all.
func verbatim(id, name, got, want string) []lint.Problem {
	if got == want {
		return nil
	}
	line, gotLine, wantLine := firstDifference(got, want)
	if line == 0 {
		return []lint.Problem{lint.Errorf(id,
			"%s matches the text the tool carries except in how the file ends", name)}
	}
	return []lint.Problem{lint.Errorf(id,
		"%s differs from the text the tool carries, first at line %d", name, line).
		At(fmt.Sprintf("%s:%d", name, line)).
		Quoting(fmt.Sprintf("has %q, want %q",
			lint.Truncate(gotLine, 64), lint.Truncate(wantLine, 64)))}
}

var licenceRules = []rule{{
	id: "OSS-101", severity: lint.Error,
	title: "A licence file sits at the repository root",
	why: "Without one the code is All Rights Reserved by default, whatever the " +
		"README says, and nobody may legally use it.",
	check: func(l *Linter) []lint.Problem {
		if slices.ContainsFunc(licenceNames, l.has) {
			return nil
		}
		return []lint.Problem{lint.Errorf("OSS-101", "no LICENSE at the repository root")}
	},
}, {
	id: "OSS-102", severity: lint.Error,
	title: "The licence is the full Apache 2.0 text",
	why: "A summary, a link or an SPDX line is not a grant. GitHub also detects a " +
		"licence by matching the full text, and shows nothing for a file it cannot match.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.licenceText()
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-102", "no LICENSE to read")}
		}
		for _, want := range apacheMarkers {
			if !strings.Contains(body, want) {
				return []lint.Problem{lint.Errorf("OSS-102",
					"LICENSE does not read as the Apache 2.0 text (missing %q)", want)}
			}
		}
		return nil
	},
	// OSS-103 named a copyright holder in the licence appendix. It is retired
	// rather than renumbered: the appendix is boilerplate the canonical text
	// ships with placeholders in, so filling it in is the modification OSS-107
	// now reports. The copyright holder moved to NOTICE, where OSS-108 checks
	// it. A retired id stays retired, because reusing one would silently change
	// what an existing waiver means.
}, {
	id: "OSS-104", severity: lint.Error,
	title: "The README says what the licence is",
	why: "A reader deciding whether they may use this looks for one line. A repository " +
		"that makes them open a 200-line legal file to find out loses them.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read("README.md")
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-104", "no README.md")}
		}
		m := licenceSection.FindStringIndex(body)
		if m == nil {
			return []lint.Problem{lint.Errorf("OSS-104", "README.md has no License section")}
		}
		end := min(m[0]+600, len(body))
		if !strings.Contains(body[m[0]:end], "LICENSE") {
			return []lint.Problem{lint.Errorf("OSS-104",
				"the README's License section does not link LICENSE")}
		}
		return nil
	},
}, {
	id: "OSS-105", severity: lint.Warn, needsTree: true,
	title: "Borrowed code carries the licence it came under",
	why: "Vendoring someone else's work strips its licence unless you carry it. " +
		"The obligation survives the copy.",
	check: func(l *Linter) []lint.Problem {
		roots := map[string]bool{}
		for _, path := range l.repo.Tracked() {
			parts := strings.Split(path, "/")
			for i, part := range parts[:max(len(parts)-1, 0)] {
				switch part {
				case "vendor", "third_party", "thirdparty":
					roots[strings.Join(parts[:i+2], "/")] = true
				}
			}
		}
		var out []lint.Problem
		for _, tree := range lint.SortedKeys(roots) {
			carried := false
			for _, p := range l.repo.Tracked() {
				if strings.HasPrefix(p, tree) && vendorLicence.MatchString("/"+p) {
					carried = true
					break
				}
			}
			if !carried {
				out = append(out, lint.Warnf("OSS-105", "%s/ carries no LICENSE or NOTICE", tree))
			}
		}
		return out
	},
}, {
	id: "OSS-106", severity: lint.Warn, needsTree: true,
	title: "One licence, stated once",
	why: "Two licence files at the root leave a reader guessing which one governs, " +
		"and GitHub picks whichever it matches first.",
	check: func(l *Linter) []lint.Problem {
		var found []string
		for _, p := range l.repo.Tracked() {
			if rootLicence.MatchString(p) {
				found = append(found, p)
			}
		}
		if len(found) > 1 {
			return []lint.Problem{lint.Warnf("OSS-106",
				"more than one root licence file: %s", strings.Join(found, ", "))}
		}
		return nil
	},
}, {
	id: "OSS-107", severity: lint.Error,
	title: "The licence is the canonical text, unmodified",
	why: "An edited licence is a licence nobody has reviewed. A word changed in the " +
		"grant changes the terms, and GitHub matches the canonical text to name the " +
		"licence at all, so it shows nothing for a file it cannot match. The appendix " +
		"placeholders are part of that text: they are instructions for applying the " +
		"licence to a source file, not a blank the project fills in. The copyright " +
		"holder goes in NOTICE, which is what OSS-108 holds.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.licenceText()
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-107", "no LICENSE to read")}
		}
		// A file that is not the Apache text at all is OSS-102's finding. Two
		// errors for one broken file leaves a reader fixing it twice.
		for _, marker := range apacheMarkers {
			if !strings.Contains(body, marker) {
				return []lint.Problem{lint.Skipf("OSS-107",
					"LICENSE is not the Apache 2.0 text; OSS-102 reports that")}
			}
		}
		return verbatim("OSS-107", l.licenceName(), body, lintdoc.LicenceText)
	},
}, {
	id: "OSS-108", severity: lint.Error,
	title: "NOTICE names the project and the copyright holder, and nothing else",
	why: "Apache 2.0 section 4(d) obliges every redistributor to carry NOTICE onward, " +
		"so whatever it says is copied forever by people who cannot edit it. That makes " +
		"it the wrong place for anything not legally required: a tagline propagates as " +
		"far as the grant does. Two lines is the whole file. The second is shared across " +
		"the family so the holder and the year are bumped in one place; the first names " +
		"this project, so a NOTICE copied from a sibling and never edited is visible.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read("NOTICE")
		if !ok {
			return []lint.Problem{lint.Errorf("OSS-108", "no NOTICE at the repository root")}
		}
		want := referenceLine(lintdoc.NoticeText, 2)
		if body == "" {
			return []lint.Problem{lint.Errorf("OSS-108", "NOTICE is empty")}
		}
		if !strings.HasSuffix(body, "\n") {
			return []lint.Problem{lint.Errorf("OSS-108", "NOTICE does not end with a newline")}
		}
		lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
		if len(lines) != 2 {
			return []lint.Problem{lint.Errorf("OSS-108",
				"NOTICE is %d lines; it is the project name and %q, and nothing else",
				len(lines), want)}
		}
		var out []lint.Problem
		if !strings.HasPrefix(lines[0], noticeProjectPrefix) {
			out = append(out, lint.Errorf("OSS-108",
				"NOTICE opens with %q; the first line names the project and starts with %q",
				lint.Truncate(lines[0], 64), noticeProjectPrefix).At("NOTICE:1"))
		}
		if lines[1] != want {
			out = append(out, lint.Errorf("OSS-108",
				"NOTICE line 2 is %q, and the family's is %q",
				lint.Truncate(lines[1], 64), want).At("NOTICE:2"))
		}
		return out
	},
}, {
	id: "OSS-109", severity: lint.Error,
	title: "The code of conduct is the canonical text, unmodified",
	why: "Contributor Covenant is published under a Creative Commons licence that " +
		"requires attribution, which a paraphrase with the attribution block dropped " +
		"does not satisfy. Shortening it is worse than copying it: what gets cut is the " +
		"enforcement ladder and the reporting address, and a code of conduct that names " +
		"no consequence and no channel is a document nobody can act on. Carrying the " +
		"text whole also means a reader recognises it without reading it.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.read("CODE_OF_CONDUCT.md")
		if !ok {
			return []lint.Problem{lint.Errorf("OSS-109",
				"no CODE_OF_CONDUCT.md at the repository root")}
		}
		return verbatim("OSS-109", "CODE_OF_CONDUCT.md", body, lintdoc.CodeOfConductMD)
	},
}}
