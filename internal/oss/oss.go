// Package oss checks that a repository is in a shape it can be published in.
//
// The rules are what a published project owes a reader: a licence, a document
// set, a build a stranger's clone can run, a release they can verify, and
// nothing in the tree or in any past commit that was never meant to leave the
// machine it was written on.
//
// Every pattern matches a class rather than a name, so nothing private is
// written down here. A username is the segment after /home/ that is not a
// placeholder the project ships, and the name of whoever runs the check comes
// from the environment. A term no pattern can infer goes in .leakterms at the
// root, which is gitignored.
package oss

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	lintdoc "github.com/codesweep-ai/lint"
	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/lint"
)

// reservedDomains are reserved by RFC 2606 and RFC 6761 for documentation and
// testing, plus the noreply identities a bot commits under. Every project gets
// these; the configuration adds only what is its own.
var reservedDomains = []string{"example.com", "example.org", "example.net",
	"noreply.github.com"}

// reservedTLDs are the top-level domains reserved for testing.
var reservedTLDs = []string{"test", "example", "invalid", "localhost", "local"}

// rule is one readiness check.
type rule struct {
	id       string
	severity lint.Severity
	title    string
	why      string
	check    func(*Linter) []lint.Problem
	// online marks a rule that asks the forge about the repository itself,
	// which a run does only when asked.
	online bool
	// needsTree marks a rule whose verdict comes from reading the tracked
	// files. Where there are none to read, such a rule reports a skip rather
	// than the silence that reads as a pass.
	needsTree bool
}

// Linter checks one repository.
type Linter struct {
	cfg    config.OSS
	repo   *lint.Repo
	Online bool

	text       map[string]string // tracked files that read as text
	order      []string          // their paths, sorted, for stable reporting
	unreadable []string          // tracked, not a known asset, not decodable
	leaks      []lint.Problem
	scanned    bool

	loose    []string // full SHAs no remote in this clone carries
	askedGit bool     // whether the question above has been put to git
	canTell  bool     // whether git could answer it
}

// New returns a readiness linter for the repository given.
func New(cfg config.OSS, repo *lint.Repo) *Linter {
	l := &Linter{cfg: cfg, repo: repo, text: map[string]string{}}
	l.readTracked()
	return l
}

// readTracked reads every tracked file once.
//
// A file that cannot be read is recorded rather than skipped. A file nobody
// can inspect must never be reported as clean, which is how a committed editor
// swap file once carried a username past a scan of this kind.
func (l *Linter) readTracked() {
	const tooBig = 40 << 20
	for _, path := range l.repo.Tracked() {
		if l.isBinaryAsset(path) {
			continue
		}
		full := l.repo.Path(path)
		st, err := os.Stat(full)
		if err != nil || !st.Mode().IsRegular() || st.Size() > tooBig {
			continue
		}
		b, err := os.ReadFile(full)
		if err != nil || !utf8Valid(b) {
			l.unreadable = append(l.unreadable, path)
			continue
		}
		l.text[path] = string(b)
		l.order = append(l.order, path)
	}
	sort.Strings(l.order)
}

func (l *Linter) isBinaryAsset(path string) bool {
	lower := strings.ToLower(path)
	for _, ext := range l.cfg.BinaryOK {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func utf8Valid(b []byte) bool { return utf8.Valid(b) }

// scannable yields the tracked text files the leak scans read.
//
// Every tracked file, not a chosen subset. Leaks have turned up in fixtures,
// in goldens derived from them, in a committed manifest, in docs, and in a
// script with a hard-coded path. Narrowing the scope is how the second round
// of a scrub survives the first.
func (l *Linter) scannable(visit func(path, body string)) {
	for _, path := range l.order {
		if l.skipped(path) {
			continue
		}
		visit(path, l.text[path])
	}
}

// nothingToScan reports whether the text scans had anything to read.
//
// A directory git cannot answer for, a repository with nothing committed, and
// one tracking only binary assets all leave every text scan reading zero
// files. None of them is a pass: a run that inspected nothing must never be
// indistinguishable from a run that inspected everything and found it clean.
func (l *Linter) nothingToScan() bool { return len(l.order) == 0 }

func (l *Linter) skipped(path string) bool {
	if l.isReference(path) {
		return true
	}
	for prefix := range l.cfg.SkipPaths {
		if path == prefix || strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// isReference reports whether a path is a governance file the tool carries the
// text of, holding exactly that text.
//
// Such a file cannot leak anything the reference does not already carry, and
// the reference is reviewed once for the whole family rather than per
// repository. Without this the rule set contradicts itself: OSS-109 requires a
// code of conduct that names a reporting address, and OSS-303 then reports that
// address as a leak. The exemption is conditional on the exact text, so a file
// with anything added to it is scanned like any other.
func (l *Linter) isReference(path string) bool {
	switch path {
	case "CODE_OF_CONDUCT.md":
		return l.text[path] == lintdoc.CodeOfConductMD
	case "LICENSE":
		return l.text[path] == lintdoc.LicenceText
	}
	return false
}

func (l *Linter) read(name string) (string, bool) {
	body, ok := l.text[name]
	if ok {
		return body, true
	}
	return l.repo.Read(name)
}

// has reports whether a path is tracked or present.
func (l *Linter) has(name string) bool {
	if _, ok := l.text[name]; ok {
		return true
	}
	if slices.Contains(l.repo.Tracked(), name) {
		return true
	}
	return l.repo.Exists(name)
}

func (l *Linter) allDocs() []string {
	return append(append([]string(nil), l.cfg.DocSet...), l.cfg.ExtraDocs...)
}

func (l *Linter) workflows() map[string]string {
	out := map[string]string{}
	for _, p := range l.order {
		if strings.HasPrefix(p, ".github/workflows/") &&
			(strings.HasSuffix(p, ".yml") || strings.HasSuffix(p, ".yaml")) {
			out[p] = l.text[p]
		}
	}
	return out
}

func (l *Linter) ci() (string, bool) {
	if b, ok := l.read(".github/workflows/ci.yml"); ok {
		return b, true
	}
	return l.read(".github/workflows/ci.yaml")
}

func (l *Linter) makefile() string {
	if b, ok := l.read("Makefile"); ok {
		return b
	}
	b, _ := l.read("makefile")
	return b
}

func (l *Linter) goreleaser() (string, bool) {
	if b, ok := l.read(".goreleaser.yaml"); ok {
		return b, true
	}
	return l.read(".goreleaser.yml")
}

func (l *Linter) keepsLedger() bool {
	for _, p := range l.repo.Tracked() {
		if strings.HasPrefix(p, "ledger/") {
			return true
		}
	}
	return false
}

var (
	makeBin     = regexp.MustCompile(`(?m)^BIN\s*:?=\s*\S*?([\w.-]+)\s*$`)
	releaseName = regexp.MustCompile(`(?m)^project_name:\s*(\S+)`)
	remoteSlug  = regexp.MustCompile(`[:/]([\w.-]+)/([\w.-]+?)(?:\.git)?$`)
)

// project is the command this repository ships, as a reader types it.
func (l *Linter) project() string {
	if l.cfg.Project != "" {
		return l.cfg.Project
	}
	if m := makeBin.FindStringSubmatch(l.makefile()); m != nil {
		return m[1]
	}
	if body, ok := l.goreleaser(); ok {
		if m := releaseName.FindStringSubmatch(body); m != nil {
			return m[1]
		}
	}
	return filepath.Base(l.repo.Root)
}

// slug is owner/name, from the configuration or the origin remote.
func (l *Linter) slug() string {
	if l.cfg.GitHubRepo != "" {
		return l.cfg.GitHubRepo
	}
	url, err := l.repo.Git("remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	if m := remoteSlug.FindStringSubmatch(strings.TrimSpace(url)); m != nil {
		return m[1] + "/" + m[2]
	}
	return ""
}

// excerpt is a bounded quote. A generated page puts its whole payload on one
// line, so an unbounded one would print the page.
func excerpt(text string, index int) string {
	const before, after = 40, 60
	start := index - before
	lead := "…"
	if start <= 0 {
		start, lead = 0, ""
	}
	end := min(index+after, len(text))
	return lead + strings.Join(strings.Fields(text[start:end]), " ") + "…"
}

// Run applies every rule and returns what they found, with any waiver applied.
func (l *Linter) Run() []lint.Problem {
	var out []lint.Problem
	for _, r := range rules {
		switch {
		case r.online && !l.Online:
			out = append(out, lint.Skipf(r.id, "not asked to go online"))
		case r.needsTree && l.nothingToScan():
			out = append(out, lint.Skipf(r.id,
				"no tracked file to read; git tracks nothing here, so this rule "+
					"inspected nothing"))
		default:
			out = append(out, l.runOne(r)...)
		}
	}
	return lint.Waive(out, l.cfg.Allow)
}

func (l *Linter) runOne(r rule) []lint.Problem {
	return lint.Guard(r.id, r.severity, func() []lint.Problem { return r.check(l) })
}

// Explain returns every rule, what it wants, and why it exists.
func Explain() []lint.RuleDoc {
	out := make([]lint.RuleDoc, 0, len(rules))
	for _, r := range rules {
		out = append(out, lint.RuleDoc{
			ID: r.id, Severity: r.severity.String(), Title: r.title, Why: r.why})
	}
	return out
}

// Project is the command this repository ships, as a reader types it.
func (l *Linter) Project() string { return l.project() }

// Slug is owner/name, from the configuration or the origin remote.
func (l *Linter) Slug() string { return l.slug() }
