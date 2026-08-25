// Package docset is the repository as the documentation rules read it: the
// document set, the fenced blocks in it, the source, the build files, and the
// binary the checkout builds.
//
// Two linters read the same repository. `cs-lint surface` asks the binary what
// it carries and compares that against the documents. `cs-lint refs` resolves
// what the documents point at. Both need the same document set, the same
// blocks and the same tracked text, and a second copy of that machinery would
// be a second chance for the two to disagree about what this repository says.
//
// Nothing here decides what is wrong. The rule packages do.
package docset

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/lint"
)

// Block is one fenced block, with where it came from and what it holds.
//
// Commands are the lines a reader would type. In a console block those are the
// lines after a `$` prompt, and everything else is the output the document
// claims they print.
type Block struct {
	Doc      string
	Line     int
	Lang     string
	Body     string
	Commands []string
	Output   [][]string
}

// Where is the address a finding in this block carries.
func (b Block) Where() string { return b.Doc + ":" + strconv.Itoa(b.Line) }

// SplitLines cuts a body into lines, treating a trailing newline as ending the
// last line rather than starting an empty one. A block's body always ends with
// one, and an empty final line would otherwise read as an output line the
// command never printed.
func SplitLines(body string) []string {
	body = strings.TrimSuffix(body, "\n")
	if body == "" {
		return nil
	}
	return strings.Split(body, "\n")
}

// NewBlock reads one fenced block, splitting a console sample into the
// commands and the output they claim to print.
func NewBlock(doc string, line int, lang, body string) Block {
	b := Block{Doc: doc, Line: line, Lang: lang, Body: body}
	if lang == "console" {
		for _, raw := range SplitLines(body) {
			switch {
			case strings.HasPrefix(raw, "$ "):
				b.Commands = append(b.Commands, raw[2:])
				b.Output = append(b.Output, nil)
			case len(b.Output) > 0:
				b.Output[len(b.Output)-1] = append(b.Output[len(b.Output)-1], raw)
			}
		}
		return b
	}
	var continued string
	for _, raw := range SplitLines(body) {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if continued != "" {
			line = continued + " " + line
			continued = ""
		}
		if before, ok := strings.CutSuffix(line, `\`); ok {
			continued = strings.TrimSpace(before)
			continue
		}
		b.Commands = append(b.Commands, line)
	}
	return b
}

// IsShell reports whether a fenced block's language is a shell a reader types
// into.
func IsShell(lang string) bool {
	switch lang {
	case "bash", "sh", "shell", "console":
		return true
	}
	return false
}

// Placeholders are the shapes a path takes when it is the reader's to supply.
//
// The reference rules report one a walkthrough leaves undeclared, and the
// inventory annotates the line it appears on, so both read this list.
var Placeholders = []*regexp.Regexp{
	regexp.MustCompile(`~/projects/\S+`),
	regexp.MustCompile(`/path/to/\S+`),
	regexp.MustCompile(`<your[-\w]*>`),
	regexp.MustCompile(`~/my-\S+`),
	regexp.MustCompile(`/my-\S+`),
	regexp.MustCompile(`<PATH>`),
	regexp.MustCompile(`<name-of-\S+>`),
}

// Set is one repository's documents and everything the rules compare them
// against.
type Set struct {
	cfg  config.Docs
	repo *lint.Repo

	text    map[string]string
	order   []string
	blocks  []Block
	help    map[string]string // verb path, space-joined -> help text
	gotHelp bool
	binary  string
	gotBin  bool
}

// New reads the repository given. Every tracked file is read once here, so a
// run costs one pass over the tree however many rules want it.
func New(cfg config.Docs, repo *lint.Repo) *Set {
	s := &Set{cfg: cfg, repo: repo, text: map[string]string{}}
	const tooBig = 8 << 20
	for _, path := range repo.Tracked() {
		full := repo.Path(path)
		st, err := os.Stat(full)
		if err != nil || !st.Mode().IsRegular() || st.Size() > tooBig {
			continue
		}
		b, err := os.ReadFile(full)
		if err != nil || !utf8.Valid(b) {
			continue
		}
		s.text[path] = string(b)
		s.order = append(s.order, path)
	}
	sort.Strings(s.order)
	return s
}

// Repo is the repository under check.
func (s *Set) Repo() *lint.Repo { return s.repo }

// Text returns a tracked file's contents, and whether it is there.
func (s *Set) Text(name string) (string, bool) {
	body, ok := s.text[name]
	return body, ok
}

// Docs returns the document set that is present, in the order a reader meets it.
func (s *Set) Docs() []string {
	var out []string
	for _, n := range append(append([]string(nil), s.cfg.Documents...), s.cfg.ExtraDocs...) {
		if _, ok := s.text[n]; ok {
			out = append(out, n)
		}
	}
	return out
}

// Markdown is every tracked Markdown file whose paths are this repository's
// claim, in the order a scan meets it.
//
// The document set is always in it. Everything else is included unless a
// markdownSkip prefix declares why it is not: payload this repository ships
// somewhere else, a corpus, a template materialized into a consumer repo, or a
// page another tool generates. A nested README makes the same claims the doc
// set does, and a path it names that has moved is wrong in the same way.
func (s *Set) Markdown() []string {
	inSet := map[string]bool{}
	for _, n := range s.Docs() {
		inSet[n] = true
	}
	out := append([]string(nil), s.Docs()...)
	for _, name := range s.order {
		if inSet[name] || !strings.HasSuffix(name, ".md") {
			continue
		}
		if covers(s.cfg.Refs.MarkdownSkip, name) {
			continue
		}
		out = append(out, name)
	}
	return out
}

// covers reports whether a declared prefix covers this file.
func covers(skip map[string]string, name string) bool {
	for prefix := range skip {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// AllText is every document joined, for the checks that ask only whether a
// surface is named anywhere.
//
// Not named Prose: mdtext.Prose strips what is not prose from one document,
// and a reader meeting both would reasonably expect them to be related.
func (s *Set) AllText() string {
	var b strings.Builder
	for _, n := range s.Docs() {
		b.WriteString(s.text[n])
		b.WriteString("\n")
	}
	return b.String()
}

// The trailing ^ is a line anchor under (?m): a closing fence has to start a
// line, or an indented ``` inside a block would end it early.
//
//nolint:gocritic // badRegexp reads the anchor as redundant; (?m) makes it load-bearing
var fenced = regexp.MustCompile("(?sm)^```([a-zA-Z]*)\n(.*?)^```")

// Blocks returns every fenced block in the document set.
func (s *Set) Blocks() []Block {
	if s.blocks != nil {
		return s.blocks
	}
	s.blocks = []Block{}
	for _, name := range s.Docs() {
		body := s.text[name]
		for _, m := range fenced.FindAllStringSubmatchIndex(body, -1) {
			s.blocks = append(s.blocks, NewBlock(name, lint.Line(body, m[0]),
				strings.ToLower(body[m[2]:m[3]]), body[m[4]:m[5]]))
		}
	}
	return s.blocks
}

var (
	makeBin     = regexp.MustCompile(`(?m)^BIN\s*:?=\s*\S*?([\w.-]+)\s*$`)
	releaseName = regexp.MustCompile(`(?m)^project_name:\s*(\S+)`)
	notUpper    = regexp.MustCompile(`[^A-Z0-9]`)
)

// Tool is the command name, guessed from the build file when not configured.
func (s *Set) Tool() string {
	if s.cfg.Surface.Tool != "" {
		return s.cfg.Surface.Tool
	}
	mk := s.text["Makefile"] + s.text["makefile"]
	if m := makeBin.FindStringSubmatch(mk); m != nil {
		return m[1]
	}
	gor := s.text[".goreleaser.yaml"] + s.text[".goreleaser.yml"]
	if m := releaseName.FindStringSubmatch(gor); m != nil {
		return m[1]
	}
	return filepath.Base(s.repo.Root)
}

// Binary is a binary to ask for help, preferring the one this checkout built
// over whatever the developer installed last.
func (s *Set) Binary() string {
	if s.gotBin {
		return s.binary
	}
	s.gotBin = true
	var candidates []string
	if s.cfg.Surface.ToolPath != "" {
		candidates = append(candidates, s.repo.Path(s.cfg.Surface.ToolPath))
	}
	candidates = append(candidates, s.repo.Path(filepath.Join("bin", s.Tool())))
	for _, path := range candidates {
		if st, err := os.Stat(path); err == nil && st.Mode().IsRegular() &&
			st.Mode().Perm()&0o111 != 0 {
			s.binary = path
			return s.binary
		}
	}
	if found, err := exec.LookPath(s.Tool()); err == nil {
		s.binary = found
	}
	return s.binary
}

// RunTool runs the tool and returns its combined output and its exit status.
// A status of -1 means the tool could not be run at all, which a check reports
// as a skip rather than a pass.
func (s *Set) RunTool(args ...string) (out string, status int) {
	binary := s.Binary()
	if binary == "" {
		return "", -1
	}
	cmd := exec.Command(binary, args...)
	cmd.Dir = s.repo.Root
	done := make(chan struct{})
	var b []byte
	var err error
	go func() {
		b, err = cmd.CombinedOutput()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		return "", -1
	}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return string(b), ee.ExitCode()
		}
		return "", -1
	}
	return string(b), 0
}

// HelpTree maps every verb path the tool carries to its help text.
//
// Walked rather than assumed: a subcommand's own subcommands are where a
// surface goes undocumented, because nothing at the top level names them.
func (s *Set) HelpTree() map[string]string {
	if s.gotHelp {
		return s.help
	}
	s.gotHelp = true
	s.help = map[string]string{}
	if s.Binary() == "" {
		return s.help
	}
	pending := [][]string{{}}
	seen := map[string]bool{}
	for len(pending) > 0 {
		path := pending[0]
		pending = pending[1:]
		key := strings.Join(path, " ")
		if seen[key] || len(path) > 3 {
			continue
		}
		seen[key] = true
		text, status := s.RunTool(append(append([]string{}, path...), "--help")...)
		if status < 0 {
			continue
		}
		s.help[key] = text
		// A tool with no per-verb help answers `<tool> <verb> --help` with the
		// page its parent gave, listing the same verbs again. Reading those as
		// children multiplies the tree by itself at every level, so an
		// identical page means this verb has no subcommands.
		if len(path) > 0 {
			if parent, ok := s.help[strings.Join(path[:len(path)-1], " ")]; ok && parent == text {
				continue
			}
		}
		for _, child := range VerbsIn(text) {
			pending = append(pending, append(append([]string{}, path...), child))
		}
	}
	return s.help
}

// Verbs is every verb path the tool carries, space-joined.
func (s *Set) Verbs() map[string]bool {
	out := map[string]bool{}
	for path := range s.HelpTree() {
		if path != "" {
			out[path] = true
		}
	}
	return out
}

var longFlag = regexp.MustCompile(`(--[a-z][a-z0-9-]+)`)

// Flags is every long flag the tool's help tree mentions.
//
// The completion subtree is left out: its flags come from the shell completion
// framework rather than from this project, and a document that named them
// would be documenting somebody else's surface.
func (s *Set) Flags() map[string]bool {
	out := map[string]bool{}
	for path, text := range s.HelpTree() {
		if path == "completion" || strings.HasPrefix(path, "completion ") {
			continue
		}
		for _, m := range longFlag.FindAllStringSubmatch(text, -1) {
			out[m[1]] = true
		}
	}
	return out
}

var sourceExts = []string{".go", ".rs", ".py", ".ts", ".tsx", ".js", ".jsx",
	".mjs", ".cjs", ".java", ".rb", ".c", ".h", ".cpp", ".sh"}

// Source yields tracked source files, which is where a setting is really read.
//
// Tests are left out. A variable only the suite reads is instrumentation
// rather than a setting, and documenting it would tell a user to reach for
// something built for the harness. A tree the tuning file declares under
// `surface.sourceSkip` is left out too: what it says is another program's.
func (s *Set) Source(visit func(path, body string)) {
	for _, path := range s.order {
		if strings.HasPrefix(path, "vendor/") || IsTest(path) {
			continue
		}
		if covers(s.cfg.Surface.SourceSkip, path) {
			continue
		}
		for _, ext := range sourceExts {
			if strings.HasSuffix(path, ext) {
				visit(path, s.text[path])
				break
			}
		}
	}
}

// CitedByDefault is the document a bare § is read against, or empty where
// there is no unambiguous answer.
//
// The spec is it wherever there is one, because that is the document a comment
// citing a section number means. Failing that, a repository with exactly one
// document that numbers its sections has only one candidate. Anything else is
// ambiguous, and a rule that guesses which document was meant reports a
// finding nobody can act on.
func (s *Set) CitedByDefault() string {
	var numbered []string
	for _, name := range s.Docs() {
		if !numberedSection.MatchString(s.text[name]) {
			continue
		}
		if name == "SPEC.md" {
			return name
		}
		numbered = append(numbered, name)
	}
	if len(numbered) == 1 {
		return numbered[0]
	}
	return ""
}

var numberedSection = regexp.MustCompile(`(?m)^#{2,6}\s+\d+(?:\.\d+)*[.\s]`)

// AllSource yields every tracked source file, the suite included, and honours
// no sourceSkip.
//
// Both differences are deliberate, and both are about citations rather than
// settings. A comment in a test pointing at a renumbered section misleads its
// next reader exactly as one in production code does. And a tree excluded
// because the settings it reads are not this tool's still cites this
// repository's own spec: the shell scripts a sandbox image ships are the case
// this was written for, and they carry no file extension either, so a file
// opening with a shebang counts as source here.
func (s *Set) AllSource(visit func(path, body string)) {
	for _, path := range s.order {
		if strings.HasPrefix(path, "vendor/") {
			continue
		}
		body := s.text[path]
		named := false
		for _, ext := range sourceExts {
			if strings.HasSuffix(path, ext) {
				named = true
				break
			}
		}
		if named || (filepath.Ext(path) == "" && strings.HasPrefix(body, "#!")) {
			visit(path, body)
		}
	}
}

// EnvPrefix is the variable prefix this tool reads.
func (s *Set) EnvPrefix() string {
	if s.cfg.Surface.EnvPrefix != "" {
		return s.cfg.Surface.EnvPrefix
	}
	return notUpper.ReplaceAllString(strings.ToUpper(s.Tool()), "_") + "_"
}

// BuildFiles yields the files that describe how the project is built.
func (s *Set) BuildFiles(visit func(path, body string)) {
	for _, path := range s.order {
		if path == "Makefile" || path == "makefile" ||
			strings.HasPrefix(path, "scripts/") || strings.HasPrefix(path, "Taskfile") {
			visit(path, s.text[path])
		}
	}
}

// reads are the shapes a getenv-style call takes across the languages.
var reads = []string{"getenv", "lookupenv", "envor", "environ", "process.env",
	"env[", "env::var", "env.var", "env_var", "env("}

// EnvReads returns the prefixed variable names a file reads, with the write
// sites left out.
//
// A tool that hands a child process `-e NAME=value` is not reading NAME, and a
// list of those is the largest class of false positive an environment scan has.
// A read is the name inside a getenv-shaped call, or spelled $NAME.
func EnvReads(prefix, body string) map[string]bool {
	re := regexp.MustCompile(`(\$\{?)?\b(` + regexp.QuoteMeta(prefix) + `[A-Z0-9_]+)\b`)
	names := map[string]bool{}
	for _, m := range re.FindAllStringSubmatchIndex(body, -1) {
		name := body[m[4]:m[5]]
		if m[2] >= 0 { // spelled $NAME
			names[name] = true
			continue
		}
		start := max(m[0]-60, 0)
		window := strings.ToLower(body[start:m[0]])
		for _, token := range reads {
			if strings.Contains(window, token) {
				names[name] = true
				break
			}
		}
	}
	return names
}

// IsTest reports whether a path is test code, by the conventions the languages
// use.
func IsTest(path string) bool {
	name := filepath.Base(path)
	for part := range strings.SplitSeq(path, "/") {
		if part == "test" || part == "tests" || part == "spec" {
			return true
		}
	}
	for _, suffix := range []string{"_test.go", "_test.py", "_test.rb", "_spec.rb"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return strings.HasPrefix(name, "test_") ||
		strings.Contains(name, ".test.") || strings.Contains(name, ".spec.")
}

// VerbOf returns the verb path in an argv, with flags and the values they take
// removed.
//
// A flag's value is a word like any other, so `--cassette build` would read as
// a verb called build. Anything after a flag that carries no `=` is that
// flag's value unless it is another flag.
func VerbOf(words []string) string {
	var path []string
	skipNext := false
	for _, word := range words {
		if skipNext {
			skipNext = false
			if !strings.HasPrefix(word, "-") {
				continue
			}
		}
		if strings.HasPrefix(word, "-") {
			skipNext = !strings.Contains(word, "=")
			continue
		}
		path = append(path, word)
	}
	return strings.Join(path, " ")
}

var (
	commandsHead = regexp.MustCompile(`(?i)^\s*(available )?(commands|verbs):\s*$`)
	commandRow   = regexp.MustCompile(`^\s{1,6}([a-z][a-z0-9-]*)(?:\s+<[^>]+>|\s+\[[^\]]+\])?\s+\S`)
)

// VerbsIn returns the subcommand names a help page lists.
func VerbsIn(help string) []string {
	var verbs []string
	listing := false
	for raw := range strings.SplitSeq(help, "\n") {
		// "Commands:", "Available Commands:" and "verbs:" all head the same
		// list. A hand-rolled help page picks its own word, and a parser that
		// knows only one framework's finds nothing and reports nothing.
		if commandsHead.MatchString(raw) {
			listing = true
			continue
		}
		if !listing {
			continue
		}
		if strings.TrimSpace(raw) == "" {
			break
		}
		// A row is a name, optionally the argument it takes, then the
		// description: `normalize <path>    write the JSON tree`. Without the
		// argument the row falls through and the verb goes undiscovered.
		//
		// One space is enough between the two. A help page pads the column to
		// its widest entry, so the longest verb on any page is separated by
		// exactly one space — and requiring two silently drops it, along with
		// every flag and subcommand underneath it.
		if m := commandRow.FindStringSubmatch(raw); m != nil {
			verbs = append(verbs, m[1])
		} else if strings.HasPrefix(strings.TrimSpace(raw), "-") {
			break
		}
	}
	var ordinary, last []string
	for _, v := range verbs {
		if v == "help" || v == "completion" {
			last = append(last, v)
		} else {
			ordinary = append(ordinary, v)
		}
	}
	return append(ordinary, last...)
}
