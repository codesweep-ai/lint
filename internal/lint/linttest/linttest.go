// Package linttest builds the scratch repositories the rule tests read.
//
// Every scan reads what git tracks rather than what the tree happens to hold,
// so a test fixture has to be a real repository with a real commit. Two rule
// packages needed the same twenty lines to make one, which is how the helper
// ended up here rather than beside either of them.
package linttest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/codesweep-ai/lint/internal/lint"
)

// Tree writes the files given into a temporary directory and returns a repo
// rooted there. Nothing is committed, so git tracks nothing: this is the state
// a scan has to report a skip for.
func Tree(t *testing.T, files map[string]string) *lint.Repo {
	t.Helper()
	return lint.NewRepo(write(t, files))
}

// Repo writes the files given, commits them, and returns a repo rooted there.
func Repo(t *testing.T, files map[string]string) *lint.Repo {
	t.Helper()
	root := write(t, files)
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "ada@example.com"},
		{"config", "user.name", "Ada"},
		{"add", "-A"},
		{"commit", "-qm", "Add the tree"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return lint.NewRepo(root)
}

func write(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
