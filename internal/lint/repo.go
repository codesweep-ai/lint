package lint

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Repo is the repository under check, and the cache the rules read it through.
//
// Sixty-five rules over a few hundred tracked files would otherwise stat and
// read the same file sixty-five times. Everything a rule asks for is read once
// and kept, so a run costs one pass over the tree however many rules want it.
type Repo struct {
	Root string

	once    sync.Once
	tracked []string
	files   map[string]string
	missing map[string]bool
}

// NewRepo returns the repository rooted at the path given.
func NewRepo(root string) *Repo {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return &Repo{
		Root:    abs,
		files:   make(map[string]string),
		missing: make(map[string]bool),
	}
}

// Path joins a repository-relative path onto the root.
func (r *Repo) Path(rel string) string { return filepath.Join(r.Root, rel) }

// Read returns the contents of a repository-relative file, and whether it
// could be read. A file that cannot be read is remembered as missing, so a
// rule asking twice pays for one failed open.
func (r *Repo) Read(rel string) (string, bool) {
	if body, ok := r.files[rel]; ok {
		return body, true
	}
	if r.missing[rel] {
		return "", false
	}
	b, err := os.ReadFile(r.Path(rel))
	if err != nil {
		r.missing[rel] = true
		return "", false
	}
	r.files[rel] = string(b)
	return string(b), true
}

// Exists reports whether a repository-relative path is there.
func (r *Repo) Exists(rel string) bool {
	_, err := os.Stat(r.Path(rel))
	return err == nil
}

// Tracked returns every file git tracks, repository-relative and sorted.
//
// The scans read this rather than walking the tree, because a leak in an
// untracked file is not published and an untracked build directory would
// otherwise dominate every scan.
func (r *Repo) Tracked() []string {
	r.once.Do(func() {
		out, err := r.Git("ls-files", "-z")
		if err != nil {
			return
		}
		for f := range strings.SplitSeq(out, "\x00") {
			if f != "" {
				r.tracked = append(r.tracked, f)
			}
		}
	})
	return r.tracked
}

// Git runs a git command in the repository and returns its standard output.
func (r *Repo) Git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Root
	out, err := cmd.Output()
	return string(out), err
}

// Run executes a command in the repository and returns its combined output
// and whether it exited zero. A linter shells out to ask another tool a
// question, and a non-zero exit is usually the answer rather than a failure.
func (r *Repo) Run(name string, args ...string) (string, bool) {
	cmd := exec.Command(name, args...)
	cmd.Dir = r.Root
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

// Have reports whether a tool is on the PATH.
func Have(tool string) bool {
	_, err := exec.LookPath(tool)
	return err == nil
}
