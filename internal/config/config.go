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

	"gopkg.in/yaml.v3"
)

// Name is the file the knobs are read from, at the repository root.
const Name = ".cs-lint.yaml"

// Config is the whole file: one section per linter.
type Config struct {
	Docs        Docs        `yaml:"docs"`
	OSS         OSS         `yaml:"oss"`
	Walkthrough Walkthrough `yaml:"walkthrough"`
}

// Docs tunes the prose linter.
type Docs struct {
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
	// RequiredTargets are the task-runner targets the project must carry.
	RequiredTargets []string `yaml:"requiredTargets"`
	// ExpectedTargets are the rest of the family's vocabulary. A missing one
	// is a warning: somebody who has worked on a sibling will reach for it.
	ExpectedTargets []string `yaml:"expectedTargets"`
}

// Walkthrough tunes the claims linter.
type Walkthrough struct {
	// Tool is the command name, guessed from the build file when empty.
	Tool string `yaml:"tool"`
	// ToolPath points at the binary this checkout builds, so a check reads
	// this tree rather than whatever the developer installed last.
	ToolPath string `yaml:"toolPath"`
	// Docs is the document set the claims are read from.
	Docs []string `yaml:"docs"`
	// ExtraDocs are the standalone pages the set adds.
	ExtraDocs []string `yaml:"extraDocs"`
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
	// PlaceholderOK are placeholder paths a block may name on purpose.
	PlaceholderOK []string `yaml:"placeholderOK"`
	// PrereqOK are tools the build needs that no document has to name.
	PrereqOK []string `yaml:"prereqOK"`
	// SourceSkip maps a path prefix to why its settings are not this tool's.
	SourceSkip map[string]string `yaml:"sourceSkip"`
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

// Default returns the configuration a project gets before it tunes anything.
func Default() *Config {
	return &Config{
		Docs: Docs{
			Terms:      map[string]string{},
			TermsProse: map[string]string{},
		},
		OSS: OSS{
			DocSet: []string{"README.md", "INSTALL.md", "MANUAL.md", "SPEC.md",
				"CONTRIBUTING.md", "AGENTS.md"},
			// The four placeholder names in wide use. A project adds the
			// account an image of its own ships under, and nothing else.
			HomeAllow: []string{"user", "you", "name", "runner"},
			SkipPaths: map[string]string{},
			Allow:     map[string]string{},
			BinaryOK: []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico",
				".icns", ".woff", ".woff2", ".ttf", ".otf", ".pdf", ".zip",
				".gz", ".tar", ".mp4", ".mov", ".wasm"},
			RequiredTargets: []string{"build", "test", "check", "docs", "oss", "clean"},
			ExpectedTargets: []string{"help", "install", "uninstall", "fmt",
				"fmt-check", "vet", "lint"},
		},
		Walkthrough: Walkthrough{
			Docs: []string{"README.md", "INSTALL.md", "MANUAL.md", "SPEC.md",
				"CONTRIBUTING.md"},
			EnvInternal:  map[string]string{},
			SampleSkip:   map[string]string{},
			SourceSkip:   map[string]string{},
			MarkdownSkip: map[string]string{},
			CitationSkip: map[string]string{},
			AgentSection: "Notes for agents",
			Allow:        map[string]string{},
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
	// KnownFields makes a misspelled key an error rather than a knob that
	// silently does nothing, which is the failure this format is most prone to.
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", Name, err)
	}
	return cfg, nil
}
