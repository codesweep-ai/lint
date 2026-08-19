package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMissingFileGetsTheDefaults(t *testing.T) {
	// A repository with nothing to tune can leave the file out entirely.
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.OSS.DocSet) == 0 || len(cfg.OSS.HomeAllow) == 0 {
		t.Errorf("the defaults are not there: %+v", cfg.OSS)
	}
}

func TestTuningOverridesTheDefaults(t *testing.T) {
	root := t.TempDir()
	body := "docs:\n  glossary: [cassette]\noss:\n  project: cs-thing\n  published: true\n"
	if err := os.WriteFile(filepath.Join(root, Name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OSS.Project != "cs-thing" || !cfg.OSS.Published {
		t.Errorf("the tuning was not read: %+v", cfg.OSS)
	}
	if len(cfg.Docs.Glossary) != 1 {
		t.Errorf("the glossary was not read: %v", cfg.Docs.Glossary)
	}
	// What the file does not mention keeps its default.
	if len(cfg.OSS.DocSet) == 0 {
		t.Error("an untouched default was dropped")
	}
}

func TestAMisspelledKeyIsAnError(t *testing.T) {
	// A knob that silently does nothing is the failure this format is most
	// prone to, so an unknown key fails the run rather than being ignored.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, Name),
		[]byte("oss:\n  proejct: cs-thing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Error("a misspelled key was accepted")
	}
}

func TestBrokenYAMLIsReported(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, Name), []byte("oss:\n  - [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Error("a file that does not parse was accepted")
	}
}
