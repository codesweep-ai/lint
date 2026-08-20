// Package walk checks that the documentation still describes the software it
// ships with.
//
// The prose linter checks how the documents are written. The readiness linter
// checks what a published repository owes a reader. This one checks the
// claims: that every command a document names exists, that every command the
// tool carries is named, that the settings the code reads are the settings the
// documents list, that a sample output is still what the command prints, and
// that a tool the build needs is named somewhere a reader will find it.
//
// Every check compares a document against something that cannot lie: the
// tool's own help tree, the source that reads an environment variable, the
// build file that shells out to a binary, or the command re-run right now.
// Nothing here guesses what a document ought to say.
package walk

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

// rule is one claims check.
type rule struct {
	id       string
	severity lint.Severity
	title    string
	why      string
	check    func(*Linter) []lint.Problem
}

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

// splitLines cuts a body into lines, treating a trailing newline as ending the
// last line rather than starting an empty one. A block's body always ends with
// one, and an empty final line would otherwise read as an output line the
// command never printed.
func splitLines(body string) []string {
	body = strings.TrimSuffix(body, "\n")
	if body == "" {
		return nil
	}
	return strings.Split(body, "\n")
}

func newBlock(doc string, line int, lang, body string) Block {
	b := Block{Doc: doc, Line: line, Lang: lang, Body: body}
	if lang == "console" {
		for _, raw := range splitLines(body) {
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
	for _, raw := range splitLines(body) {
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

// Linter checks one repository's claims.
type Linter struct {
	cfg  config.Walkthrough
	repo *lint.Repo

	text    map[string]string
	order   []string
	blocks  []Block
	help    map[string]string // verb path, space-joined -> help text
	gotHelp bool
	binary  string
	gotBin  bool
}

// New returns a claims linter for the repository given.
func New(cfg config.Walkthrough, repo *lint.Repo) *Linter {
	l := &Linter{cfg: cfg, repo: repo, text: map[string]string{}}
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
		l.text[path] = string(b)
		l.order = append(l.order, path)
	}
	sort.Strings(l.order)
	return l
}

// Docs returns the document set that is present, in the order a reader meets it.
func (l *Linter) Docs() []string {
	var out []string
	for _, n := range append(append([]string(nil), l.cfg.Docs...), l.cfg.ExtraDocs...) {
		if _, ok := l.text[n]; ok {
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
func (l *Linter) Markdown() []string {
	inSet := map[string]bool{}
	for _, n := range l.Docs() {
		inSet[n] = true
	}
	out := append([]string(nil), l.Docs()...)
	for _, name := range l.order {
		if inSet[name] || !strings.HasSuffix(name, ".md") {
			continue
		}
		if l.skipMarkdown(name) {
			continue
		}
		out = append(out, name)
	}
	return out
}

// skipMarkdown reports whether a declared prefix covers this file.
func (l *Linter) skipMarkdown(name string) bool {
	for prefix := range l.cfg.MarkdownSkip {
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
func (l *Linter) AllText() string {
	var b strings.Builder
	for _, n := range l.Docs() {
		b.WriteString(l.text[n])
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
func (l *Linter) Blocks() []Block {
	if l.blocks != nil {
		return l.blocks
	}
	l.blocks = []Block{}
	for _, name := range l.Docs() {
		body := l.text[name]
		for _, m := range fenced.FindAllStringSubmatchIndex(body, -1) {
			l.blocks = append(l.blocks, newBlock(name, lint.Line(body, m[0]),
				strings.ToLower(body[m[2]:m[3]]), body[m[4]:m[5]]))
		}
	}
	return l.blocks
}

var (
	makeBin     = regexp.MustCompile(`(?m)^BIN\s*:?=\s*\S*?([\w.-]+)\s*$`)
	releaseName = regexp.MustCompile(`(?m)^project_name:\s*(\S+)`)
	notUpper    = regexp.MustCompile(`[^A-Z0-9]`)
)

// Tool is the command name, guessed from the build file when not configured.
func (l *Linter) Tool() string {
	if l.cfg.Tool != "" {
		return l.cfg.Tool
	}
	mk := l.text["Makefile"] + l.text["makefile"]
	if m := makeBin.FindStringSubmatch(mk); m != nil {
		return m[1]
	}
	gor := l.text[".goreleaser.yaml"] + l.text[".goreleaser.yml"]
	if m := releaseName.FindStringSubmatch(gor); m != nil {
		return m[1]
	}
	return filepath.Base(l.repo.Root)
}

// Binary is a binary to ask for help, preferring the one this checkout built
// over whatever the developer installed last.
func (l *Linter) Binary() string {
	if l.gotBin {
		return l.binary
	}
	l.gotBin = true
	var candidates []string
	if l.cfg.ToolPath != "" {
		candidates = append(candidates, l.repo.Path(l.cfg.ToolPath))
	}
	candidates = append(candidates, l.repo.Path(filepath.Join("bin", l.Tool())))
	for _, path := range candidates {
		if st, err := os.Stat(path); err == nil && st.Mode().IsRegular() &&
			st.Mode().Perm()&0o111 != 0 {
			l.binary = path
			return l.binary
		}
	}
	if found, err := exec.LookPath(l.Tool()); err == nil {
		l.binary = found
	}
	return l.binary
}

// RunTool runs the tool and returns its combined output and its exit status.
// A status of -1 means the tool could not be run at all, which a check reports
// as a skip rather than a pass.
func (l *Linter) RunTool(args ...string) (out string, status int) {
	binary := l.Binary()
	if binary == "" {
		return "", -1
	}
	cmd := exec.Command(binary, args...)
	cmd.Dir = l.repo.Root
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
func (l *Linter) HelpTree() map[string]string {
	if l.gotHelp {
		return l.help
	}
	l.gotHelp = true
	l.help = map[string]string{}
	if l.Binary() == "" {
		return l.help
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
		text, status := l.RunTool(append(append([]string{}, path...), "--help")...)
		if status < 0 {
			continue
		}
		l.help[key] = text
		// A tool with no per-verb help answers `<tool> <verb> --help` with the
		// page its parent gave, listing the same verbs again. Reading those as
		// children multiplies the tree by itself at every level, so an
		// identical page means this verb has no subcommands.
		if len(path) > 0 {
			if parent, ok := l.help[strings.Join(path[:len(path)-1], " ")]; ok && parent == text {
				continue
			}
		}
		for _, child := range verbsIn(text) {
			pending = append(pending, append(append([]string{}, path...), child))
		}
	}
	return l.help
}

// Verbs is every verb path the tool carries, space-joined.
func (l *Linter) Verbs() map[string]bool {
	out := map[string]bool{}
	for path := range l.HelpTree() {
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
func (l *Linter) Flags() map[string]bool {
	out := map[string]bool{}
	for path, text := range l.HelpTree() {
		if path == "completion" || strings.HasPrefix(path, "completion ") {
			continue
		}
		for _, m := range longFlag.FindAllStringSubmatch(text, -1) {
			out[m[1]] = true
		}
	}
	return out
}

var sourceExts = []string{".go", ".rs", ".py", ".ts", ".js", ".java", ".rb",
	".c", ".h", ".cpp", ".sh"}

// Source yields tracked source files, which is where a setting is really read.
//
// Tests are left out. A variable only the suite reads is instrumentation
// rather than a setting, and documenting it would tell a user to reach for
// something built for the harness.
func (l *Linter) Source(visit func(path, body string)) {
	for _, path := range l.order {
		if strings.HasPrefix(path, "vendor/") || isTest(path) {
			continue
		}
		skip := false
		for prefix := range l.cfg.SourceSkip {
			if strings.HasPrefix(path, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		for _, ext := range sourceExts {
			if strings.HasSuffix(path, ext) {
				visit(path, l.text[path])
				break
			}
		}
	}
}

// EnvPrefix is the variable prefix this tool reads.
func (l *Linter) EnvPrefix() string {
	if l.cfg.EnvPrefix != "" {
		return l.cfg.EnvPrefix
	}
	return notUpper.ReplaceAllString(strings.ToUpper(l.Tool()), "_") + "_"
}

// BuildFiles yields the files that describe how the project is built.
func (l *Linter) BuildFiles(visit func(path, body string)) {
	for _, path := range l.order {
		if path == "Makefile" || path == "makefile" ||
			strings.HasPrefix(path, "scripts/") || strings.HasPrefix(path, "Taskfile") {
			visit(path, l.text[path])
		}
	}
}

// reads are the shapes a getenv-style call takes across the languages.
var reads = []string{"getenv", "lookupenv", "envor", "environ", "process.env",
	"env[", "env::var", "env.var", "env_var", "env("}

// envReads returns the prefixed variable names a file reads, with the write
// sites left out.
//
// A tool that hands a child process `-e NAME=value` is not reading NAME, and a
// list of those is the largest class of false positive an environment scan has.
// A read is the name inside a getenv-shaped call, or spelled $NAME.
func envReads(prefix, body string) map[string]bool {
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

// isTest reports whether a path is test code, by the conventions the languages
// use.
func isTest(path string) bool {
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

var elision = regexp.MustCompile(`…|\.\.\.`)

// matches reports whether a recorded line still matches what the command
// printed.
//
// An elision stands for whatever the command prints there, which is how a
// document keeps a sample true across a number that moves: a byte count, a
// duration, a path under a temporary directory. Everything either side of it
// still has to match, so the elision buys drift in one place rather than
// turning the whole line off.
func matches(recorded, actual string) bool {
	if !strings.Contains(recorded, "…") && !strings.Contains(recorded, "...") {
		return recorded == actual
	}
	parts := elision.Split(recorded, -1)
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		quoted = append(quoted, regexp.QuoteMeta(p))
	}
	re, err := regexp.Compile(`(?s)\A` + strings.Join(quoted, ".*") + `\z`)
	return err == nil && re.MatchString(actual)
}

// verbOf returns the verb path in an argv, with flags and the values they take
// removed.
//
// A flag's value is a word like any other, so `--cassette build` would read as
// a verb called build. Anything after a flag that carries no `=` is that
// flag's value unless it is another flag.
func verbOf(words []string) string {
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

// verbsIn returns the subcommand names a help page lists.
func verbsIn(help string) []string {
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

// Run applies every rule and returns what they found, with any waiver applied.
func (l *Linter) Run() []lint.Problem {
	var out []lint.Problem
	for _, r := range rules {
		out = append(out, l.runOne(r)...)
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
