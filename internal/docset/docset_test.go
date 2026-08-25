package docset

import (
	"strings"
	"testing"

	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/lint/linttest"
)

func TestBlockSplitsAConsoleSample(t *testing.T) {
	// In a console block the lines after a prompt are the command, and
	// everything else is the output the document claims it prints.
	b := NewBlock("MANUAL.md", 1, "console", "$ tool version\ntool 1.2.3\n$ tool ls\na\nb\n")
	if len(b.Commands) != 2 {
		t.Fatalf("got %d commands, want 2: %v", len(b.Commands), b.Commands)
	}
	if b.Commands[0] != "tool version" || len(b.Output[0]) != 1 {
		t.Errorf("first sample is %q -> %v", b.Commands[0], b.Output[0])
	}
	if len(b.Output[1]) != 2 {
		t.Errorf("second sample's output is %v, want two lines", b.Output[1])
	}
}

func TestBlockJoinsAContinuedLine(t *testing.T) {
	b := NewBlock("README.md", 1, "bash", "tool run \\\n  --flag value\n")
	if len(b.Commands) != 1 {
		t.Fatalf("got %d commands, want 1: %v", len(b.Commands), b.Commands)
	}
	if !strings.Contains(b.Commands[0], "--flag value") {
		t.Errorf("the continuation was lost: %q", b.Commands[0])
	}
}

func TestVerbOfDropsFlagsAndTheirValues(t *testing.T) {
	// A flag's value is a word like any other, so `--cassette build` would read
	// as a verb called build.
	for _, tc := range []struct{ in, want string }{
		{"cassette ls", "cassette ls"},
		{"--cassette build verify", "verify"},
		{"--json=true report", "report"},
		// A short flag carries no "=", so the word after it is its value.
		{"-v status", ""},
	} {
		if got := VerbOf(strings.Fields(tc.in)); got != tc.want {
			t.Errorf("VerbOf(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestVerbsInReadsACommandsSection(t *testing.T) {
	// The last row is the widest name on the page, so its description is one
	// space away. Requiring two silently drops the longest verb of every help
	// page, and with it every flag and subcommand underneath.
	help := "Usage:\n  tool [command]\n\nAvailable Commands:\n" +
		"  record        record a session\n" +
		"  normalize <path>    write the JSON tree\n" +
		"  help          Help about any command\n" +
		"  surface     check the documented interface\n\nFlags:\n  -h, --help\n"
	got := VerbsIn(help)
	want := map[string]bool{"record": true, "normalize": true, "help": true,
		"surface": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, v := range got {
		if !want[v] {
			t.Errorf("found %q, which is not a verb here", v)
		}
	}
	// help and completion sort last, so an ordinary verb is discovered first.
	if got[len(got)-1] != "help" {
		t.Errorf("help is not last: %v", got)
	}
}

func TestEnvReadsIgnoresAWriteSite(t *testing.T) {
	// A tool that hands a child process -e NAME=value is not reading NAME.
	body := "cmd := exec.Command(\"x\", \"-e\", \"CS_T_CHILD=1\")\n" +
		"v := os.Getenv(\"CS_T_REAL\")\n"
	got := EnvReads("CS_T_", body)
	if !got["CS_T_REAL"] {
		t.Error("a getenv-shaped read was missed")
	}
	if got["CS_T_CHILD"] {
		t.Error("a write site was read as a setting")
	}
}

func TestEnvReadsFindsAShellVariable(t *testing.T) {
	if got := EnvReads("CS_T_", "echo $CS_T_ROOT\n"); !got["CS_T_ROOT"] {
		t.Error("a $NAME read was missed")
	}
}

func TestIsTestRecognisesTheConventions(t *testing.T) {
	for _, p := range []string{"internal/x_test.go", "test/helper.go", "tests/a.py",
		"spec/b.rb", "src/test_thing.py", "web/a.spec.ts"} {
		if !IsTest(p) {
			t.Errorf("%s is test code and was not recognised", p)
		}
	}
	for _, p := range []string{"internal/x.go", "cmd/tool/main.go", "latest/thing.go"} {
		if IsTest(p) {
			t.Errorf("%s is not test code and was recognised as it", p)
		}
	}
}

func TestBlocksAreFoundInEveryDocument(t *testing.T) {
	files := map[string]string{
		"README.md": "```bash\none\n```\n\ntext\n\n```console\n$ two\nout\n```\n",
		"MANUAL.md": "```\nthree\n```\n",
	}
	set := New(config.Docs{Documents: []string{"README.md", "MANUAL.md"}},
		linttest.Repo(t, files))
	if got := len(set.Blocks()); got != 3 {
		t.Errorf("found %d blocks, want 3", got)
	}
}

func TestHelpTreeIsEmptyWithoutABinary(t *testing.T) {
	set := New(config.Docs{Surface: config.Surface{Tool: "no-such-tool-anywhere"}},
		linttest.Repo(t, map[string]string{"a.txt": "x\n"}))
	if set.Binary() != "" {
		t.Errorf("found a binary that is not there: %q", set.Binary())
	}
	if len(set.HelpTree()) != 0 {
		t.Errorf("walked a help tree with no binary: %v", set.HelpTree())
	}
	if len(set.Verbs()) != 0 || len(set.Flags()) != 0 {
		t.Error("verbs or flags came from nowhere")
	}
}

func TestEnvPrefixIsDerivedFromTheTool(t *testing.T) {
	repo := linttest.Repo(t, map[string]string{"a.txt": "x\n"})
	set := New(config.Docs{Surface: config.Surface{Tool: "cs-thing"}}, repo)
	if got := set.EnvPrefix(); got != "CS_THING_" {
		t.Errorf("prefix derived as %q, want CS_THING_", got)
	}
	set = New(config.Docs{Surface: config.Surface{Tool: "cs-thing", EnvPrefix: "OTHER_"}}, repo)
	if got := set.EnvPrefix(); got != "OTHER_" {
		t.Errorf("the configured prefix was ignored: %q", got)
	}
}

func TestTheDocumentSetIsSharedByBothLinters(t *testing.T) {
	// The set is read from one place, so the two halves of the split cannot
	// disagree about which pages this repository publishes.
	files := map[string]string{
		"README.md": "# x\n",
		"GUIDE.md":  "# g\n",
		"OTHER.md":  "# o\n",
	}
	set := New(config.Docs{
		Documents: []string{"README.md", "MISSING.md"},
		ExtraDocs: []string{"GUIDE.md"},
	}, linttest.Repo(t, files))
	got := strings.Join(set.Docs(), " ")
	if got != "README.md GUIDE.md" {
		t.Errorf("the set is %q, want the configured documents that are present", got)
	}
}
