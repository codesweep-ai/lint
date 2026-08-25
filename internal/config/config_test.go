package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, Name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestMissingFileGetsTheDefaults(t *testing.T) {
	// A repository with nothing to tune can leave the file out entirely.
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.OSS.DocSet) == 0 || len(cfg.OSS.HomeAllow) == 0 {
		t.Errorf("the defaults are not there: %+v", cfg.OSS)
	}
	if len(cfg.Docs.Documents) == 0 {
		t.Errorf("the document set has no default: %+v", cfg.Docs)
	}
}

func TestTuningOverridesTheDefaults(t *testing.T) {
	root := write(t, "docs:\n  documents: [README.md]\n  prose:\n    glossary: [cassette]\n"+
		"  surface:\n    tool: cs-thing\noss:\n  project: cs-thing\n  published: true\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OSS.Project != "cs-thing" || !cfg.OSS.Published {
		t.Errorf("the tuning was not read: %+v", cfg.OSS)
	}
	if len(cfg.Docs.Prose.Glossary) != 1 {
		t.Errorf("the glossary was not read: %v", cfg.Docs.Prose.Glossary)
	}
	if cfg.Docs.Surface.Tool != "cs-thing" {
		t.Errorf("the tool name was not read: %q", cfg.Docs.Surface.Tool)
	}
	// What the file does not mention keeps its default.
	if len(cfg.OSS.DocSet) == 0 || cfg.Docs.Refs.AgentSection == "" {
		t.Error("an untouched default was dropped")
	}
}

// The document set sits above the two linters that read it, so one list
// answers for both.
func TestTheDocumentSetIsSharedByBothLinters(t *testing.T) {
	root := write(t, "docs:\n  documents: [README.md, GUIDE.md]\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Docs.Documents) != 2 {
		t.Errorf("the set was not read: %v", cfg.Docs.Documents)
	}
}

func TestAMisspelledKeyIsAnError(t *testing.T) {
	// A knob that silently does nothing is the failure this format is most
	// prone to, so an unknown key fails the run rather than being ignored.
	if _, err := Load(write(t, "oss:\n  proejct: cs-thing\n")); err == nil {
		t.Error("a misspelled key was accepted")
	}
}

func TestBrokenYAMLIsReported(t *testing.T) {
	if _, err := Load(write(t, "oss:\n  - [\n")); err == nil {
		t.Error("a file that does not parse was accepted")
	}
}

// A file written to the schema from before the split has to say where its
// knobs went. Both shapes fail the strict decode anyway, in the parser's own
// words; naming the new location is the difference between a message that ends
// the work and one that starts it.
func TestAWalkthroughSectionNamesItsSuccessors(t *testing.T) {
	_, err := Load(write(t, "walkthrough:\n  tool: cs-thing\n"))
	if err == nil {
		t.Fatal("a tuning file from before the split was accepted")
	}
	for _, want := range []string{"docs.surface", "docs.refs"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q does not name %s", err, want)
		}
	}
}

func TestAProseKeyUnderDocsNamesItsNewHome(t *testing.T) {
	_, err := Load(write(t, "docs:\n  glossary: [cassette]\n"))
	if err == nil {
		t.Fatal("prose keys directly under docs were accepted")
	}
	if !strings.Contains(err.Error(), "docs.prose.glossary") {
		t.Errorf("%q does not name docs.prose.glossary", err)
	}
}
