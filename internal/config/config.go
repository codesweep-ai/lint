// Package config carries the knobs a project tunes, read from one YAML file at
// the repository root.
//
// The linters themselves carry no project knowledge. Everything that differs
// between repositories lives here, so a fix to a check reaches every project
// without carrying one project's exceptions into the next.
//
// A waiver's reason is a required value rather than an optional one. A waiver
// with no reason is one nobody can review, and the reason is printed with the
// finding so a reviewer sees what was traded away.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

// Name is the file the knobs are read from, at the repository root.
const Name = ".cs-lint.yaml"

// Config is the whole file: the documentation block, and the readiness one.
type Config struct {
	Docs Docs `yaml:"docs"`
	OSS  OSS  `yaml:"oss"`
}

// Docs is the documentation block: the set of documents the checks read, and
// one section per documentation linter.
//
// The set sits above the sections because rules on both sides of it read the
// same documents. A second copy drifts, and the two halves would then disagree
// about which pages this repository publishes.
type Docs struct {
	// Documents is the set the claims and the references are read from, in
	// the order a reader meets it.
	Documents []string `yaml:"documents"`
	// ExtraDocs are the standalone pages the set adds.
	ExtraDocs []string `yaml:"extraDocs"`
	// Prose tunes `cs-lint prose`: how the documents are written.
	Prose Prose `yaml:"prose"`
	// Refs tunes `cs-lint refs`: whether every reference resolves.
	Refs Refs `yaml:"refs"`
	// Surface tunes `cs-lint surface`: whether the documented interface is
	// the real one.
	Surface Surface `yaml:"surface"`
}

// Prose tunes the prose linter.
type Prose struct {
	// SkipExtra names directories holding fixtures, corpora or generated
	// Markdown, and root-level files that are data rather than documentation.
	SkipExtra []string `yaml:"skipExtra"`
	// Glossary is the domain terms a reader of this project cannot infer.
	// Each must be introduced where a document first uses it. An empty list
	// disables the most valuable check in the set.
	Glossary []string `yaml:"glossary"`
	// LowercaseStarters are words that legitimately start a sentence in lower
	// case, which is nearly always the project's own command name.
	LowercaseStarters []string `yaml:"lowercaseStarters"`
	// ProjectVerbs are verbs the shared list does not carry. Only what is this
	// project's own belongs here; an ordinary English verb belongs in the
	// shared list, where every project gets it.
	ProjectVerbs []string `yaml:"projectVerbs"`
	// Countable names the things this repository counts for itself: fixtures,
	// rules, goldens. Prose that states one of those numbers goes stale the
	// moment the corpus changes, and nothing fails. Entries are pattern
	// fragments, so "fixtures?" covers both. An empty list disables the check.
	Countable []string `yaml:"countable"`
	// Terms adds this project's own declined words to the shared table.
	Terms map[string]string `yaml:"terms"`
	// TermsProse adds declined words that apply to prose but not to a spec.
	TermsProse map[string]string `yaml:"termsProse"`
}

// Refs tunes the reference linter.
type Refs struct {
	// PlaceholderOK are placeholder paths a block may name on purpose.
	PlaceholderOK []string `yaml:"placeholderOK"`
	// PrereqOK are tools the build needs that no document has to name.
	PrereqOK []string `yaml:"prereqOK"`
	// MarkdownSkip maps a path prefix to why the Markdown under it makes no
	// claim about this repository: payload shipped elsewhere, a corpus, a
	// template materialized into a consumer repo, or a page another tool
	// generates. The document set is always checked, whatever this says.
	MarkdownSkip map[string]string `yaml:"markdownSkip"`
	// CitationSkip maps a path prefix to why a section number written there is
	// not a citation: a rule that quotes the shape it searches for, or a test
	// fixture built to be stale on purpose.
	CitationSkip map[string]string `yaml:"citationSkip"`
	// AgentSection is the heading in the manual addressed to automated callers.
	AgentSection string `yaml:"agentSection"`
	// Allow waives a rule for this repository, with the reason.
	Allow map[string]string `yaml:"allow"`
}

// Surface tunes the interface linter.
type Surface struct {
	// Tool is the command name, guessed from the build file when empty. The
	// reference rules read it too, to know which name in a build file is the
	// tool itself rather than a program it needs installed first.
	Tool string `yaml:"tool"`
	// ToolPath points at the binary this checkout builds, so a check reads
	// this tree rather than whatever the developer installed last.
	ToolPath string `yaml:"toolPath"`
	// EnvPrefix is the variable prefix this tool reads, guessed from the tool
	// name when empty.
	EnvPrefix string `yaml:"envPrefix"`
	// EnvInternal maps a variable to why it is deliberately undocumented.
	EnvInternal map[string]string `yaml:"envInternal"`
	// SafeVerbs are the verbs a sample check may re-run. Every one has to be
	// read-only, offline and safe in a checkout, because they run on every
	// gate. A verb that writes belongs nowhere near this list: a checker that
	// writes can mask the staleness another gate exists to catch.
	SafeVerbs []string `yaml:"safeVerbs"`
	// SampleSkip maps a sample's first command to why it cannot re-run here.
	SampleSkip map[string]string `yaml:"sampleSkip"`
	// SourceSkip maps a path prefix to why the source under it is not this
	// tool's. The ledger citation rule reads it too, because a tree excluded
	// here is excluded from every scan of what this repository's own code says.
	SourceSkip map[string]string `yaml:"sourceSkip"`
	// Allow waives a rule for this repository, with the reason.
	Allow map[string]string `yaml:"allow"`
}

// OSS tunes the readiness linter.
type OSS struct {
	// Project is the command this repository ships, as a reader types it.
	// Left empty, the linter infers it from the module path or the build file.
	Project string `yaml:"project"`
	// GitHubRepo is the owner/name this repository is published as. Empty
	// means take it from the origin remote.
	GitHubRepo string `yaml:"githubRepo"`
	// Published is true once the repository is public. Published history
	// cannot be rewritten, so the history rules report as warnings from that
	// point on.
	Published bool `yaml:"published"`
	// DocSet is the documents every reader-facing repository carries.
	DocSet []string `yaml:"docSet"`
	// ExtraDocs are documents this project adds to the set, which the router
	// must also name.
	ExtraDocs []string `yaml:"extraDocs"`
	// HomeAllow are home-directory names that are a placeholder or a shipped
	// account rather than a person. Every name added here is one the check
	// stops catching, so a real login must never be added.
	HomeAllow []string `yaml:"homeAllow"`
	// EmailAllow are mail domains that are documentation addresses or machine
	// identities rather than a person's.
	EmailAllow []string `yaml:"emailAllow"`
	// SkipPaths are tracked paths the text scans skip, each with the reason it
	// is safe. Prefixes match a whole directory.
	SkipPaths map[string]string `yaml:"skipPaths"`
	// Allow waives a rule for this repository, with the reason.
	Allow map[string]string `yaml:"allow"`
	// BinaryOK are extensions a scan may skip because they are known binary
	// assets. Anything else that cannot be read as text is reported: a file
	// nobody can inspect must never be reported as clean.
	BinaryOK []string `yaml:"binaryOK"`
	// CISkip maps a target the CI workflow runs to why `make ci` does not.
	// A step that needs a privileged host or an hour is the case this is for.
	// Nothing is excluded silently: a skip nobody can review is a gate
	// deleted in private.
	CISkip map[string]string `yaml:"ciSkip"`
	// RequiredTargets are the task-runner targets the project must carry.
	RequiredTargets []string `yaml:"requiredTargets"`
	// ExpectedTargets are the rest of the family's vocabulary. A missing one
	// is a warning: somebody who has worked on a sibling will reach for it.
	ExpectedTargets []string `yaml:"expectedTargets"`
}

// Default returns the configuration a project gets before it tunes anything.
func Default() *Config {
	return &Config{
		Docs: Docs{
			Documents: []string{"README.md", "INSTALL.md", "MANUAL.md", "SPEC.md",
				"CONTRIBUTING.md"},
			Prose: Prose{
				Terms:      map[string]string{},
				TermsProse: map[string]string{},
			},
			Refs: Refs{
				MarkdownSkip: map[string]string{},
				CitationSkip: map[string]string{},
				AgentSection: "Notes for agents",
				Allow:        map[string]string{},
			},
			Surface: Surface{
				EnvInternal: map[string]string{},
				SampleSkip:  map[string]string{},
				SourceSkip:  map[string]string{},
				Allow:       map[string]string{},
			},
		},
		OSS: OSS{
			DocSet: []string{"README.md", "INSTALL.md", "MANUAL.md", "SPEC.md",
				"CONTRIBUTING.md", "AGENTS.md"},
			// The four placeholder names in wide use. A project adds the
			// account an image of its own ships under, and nothing else.
			HomeAllow: []string{"user", "you", "name", "runner"},
			SkipPaths: map[string]string{},
			Allow:     map[string]string{},
			CISkip:    map[string]string{},
			BinaryOK: []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico",
				".icns", ".woff", ".woff2", ".ttf", ".otf", ".pdf", ".zip",
				".gz", ".tar", ".mp4", ".mov", ".wasm"},
			RequiredTargets: []string{"help", "build", "test", "check", "lint",
				"prose", "refs", "oss", "clean"},
			ExpectedTargets: []string{"install", "uninstall", "fmt", "fmt-check", "vet"},
		},
	}
}

// Load reads the configuration from the repository root. A repository with
// nothing to tune can leave the file out entirely and gets the defaults.
func Load(root string) (*Config, error) {
	cfg := Default()
	b, err := os.ReadFile(filepath.Join(root, Name))
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := checkShape(b); err != nil {
		return nil, err
	}
	// KnownFields makes a misspelled key an error rather than a knob that
	// silently does nothing, which is the failure this format is most prone to.
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", Name, err)
	}
	return cfg, nil
}

// movedProse are the keys that sat directly under `docs:` while that section
// tuned the prose linter alone.
var movedProse = []string{"skipExtra", "glossary", "lowercaseStarters",
	"projectVerbs", "countable", "terms", "termsProse"}

// checkShape reports a file written to the schema from before the split.
//
// Both shapes are rejected by the strict decode that follows, and both would
// be rejected with the parser's own words: a field the reader has to map back
// onto a section they have not read about yet. Naming the new location is the
// difference between a message that ends the work and one that starts it.
func checkShape(b []byte) error {
	var file map[string]yaml.Node
	// A file that does not parse at all is the decoder's to report.
	if yaml.Unmarshal(b, &file) != nil {
		return nil
	}
	if _, ok := file["walkthrough"]; ok {
		return fmt.Errorf("%s: `walkthrough:` was split in two. The checks that read "+
			"the binary are now `docs.surface`, and the ones that resolve a reference "+
			"are `docs.refs`", Name)
	}
	docs, ok := file["docs"]
	if !ok || docs.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(docs.Content); i += 2 {
		if key := docs.Content[i].Value; slices.Contains(movedProse, key) {
			return fmt.Errorf("%s: `docs.%s` is now `docs.prose.%s`. The `docs:` section "+
				"holds the document set and one block per documentation linter: `prose`, "+
				"`refs` and `surface`", Name, key, key)
		}
	}
	return nil
}
