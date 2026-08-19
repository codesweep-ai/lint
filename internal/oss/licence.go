package oss

import (
	"regexp"
	"slices"
	"strings"

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

var (
	apachePlaceholder = regexp.MustCompile(`\[yyyy\]|\[name of copyright owner\]`)
	copyrightLine     = regexp.MustCompile(`Copyright\s+(?:\(c\)\s*)?\d{4}\s+\S`)
	licenceSection    = regexp.MustCompile(`(?im)^##+\s*Licen[cs]e\b`)
	vendorLicence     = regexp.MustCompile(`(?i)/(LICENSE|LICENCE|NOTICE|COPYING)`)
	rootLicence       = regexp.MustCompile(`(?i)^(LICENSE|LICENCE|COPYING)(\.\w+)?$`)
)

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
		for _, want := range []string{"Apache License", "Version 2.0, January 2004",
			"TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION"} {
			if !strings.Contains(body, want) {
				return []lint.Problem{lint.Errorf("OSS-102",
					"LICENSE does not read as the Apache 2.0 text (missing %q)", want)}
			}
		}
		return nil
	},
}, {
	id: "OSS-103", severity: lint.Error,
	title: "The licence appendix names a copyright holder",
	why: "Apache 2.0 ships with [yyyy] and [name of copyright owner] placeholders. " +
		"Leaving them in publishes a licence that grants rights on behalf of nobody.",
	check: func(l *Linter) []lint.Problem {
		body, ok := l.licenceText()
		if !ok {
			return []lint.Problem{lint.Skipf("OSS-103", "no LICENSE to read")}
		}
		if apachePlaceholder.MatchString(body) {
			return []lint.Problem{lint.Errorf("OSS-103",
				"LICENSE still carries the Apache placeholders")}
		}
		if !copyrightLine.MatchString(body) {
			return []lint.Problem{lint.Errorf("OSS-103",
				"LICENSE has no filled-in copyright line")}
		}
		return nil
	},
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
}}
