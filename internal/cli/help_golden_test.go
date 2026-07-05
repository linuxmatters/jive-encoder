package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

// posFixture is a CLI with positional arguments, exercising the
// Model.Positional/Summary() path in getArguments and StyledHelpPrinter.
type posFixture struct {
	Audio   string `arg:"" optional:"" help:"Audio file to encode."`
	Episode string `arg:"" optional:"" help:"Episode markdown file."`
	Format  string `help:"Output format." default:"mp3"`
}

// newPositionalContext builds a kong context whose stdout is buf, so the
// styled help output can be captured. NO_COLOR keeps the output plain.
func newPositionalContext(t *testing.T, buf *bytes.Buffer) *kong.Context {
	t.Helper()
	t.Setenv("NO_COLOR", "1")

	var fixture posFixture
	parser, err := kong.New(&fixture,
		kong.Name("jive-encoder"),
		kong.Description("Encode podcast audio."),
		kong.Exit(func(int) {}),
		kong.Writers(buf, &bytes.Buffer{}),
	)
	if err != nil {
		t.Fatalf("kong.New() error = %v", err)
	}

	ctx, err := parser.Parse([]string{})
	if err != nil {
		t.Fatalf("parser.Parse() error = %v", err)
	}
	return ctx
}

func TestGetArgumentsPositional(t *testing.T) {
	args := getArguments(newPositionalContext(t, &bytes.Buffer{}))

	want := []argument{
		{name: "[<audio>]", help: "Audio file to encode."},
		{name: "[<episode>]", help: "Episode markdown file."},
	}
	if len(args) != len(want) {
		t.Fatalf("getArguments() = %+v; want %+v", args, want)
	}
	for i, w := range want {
		if args[i] != w {
			t.Errorf("args[%d] = %+v; want %+v", i, args[i], w)
		}
	}
}

// normalise trims trailing spaces from each line and surrounding blank lines,
// stripping the Lipgloss margin padding so the golden compare is stable.
func normalise(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

func TestStyledHelpPrinterGolden(t *testing.T) {
	var out bytes.Buffer
	ctx := newPositionalContext(t, &out)

	if err := StyledHelpPrinter(kong.HelpOptions{}, ctx); err != nil {
		t.Fatalf("StyledHelpPrinter() error = %v", err)
	}

	const golden = `Jive Encoder 🪩

Encode podcast audio.


Usage:
  Hugo mode:
    jive-encoder <audio-file> <episode-md> [flags]
  Standalone mode:
    jive-encoder <audio-file> --title TEXT --num NUMBER --cover PATH [flags]


Arguments:
  [<audio>]  Audio file to encode.
  [<episode>]  Episode markdown file.


Flags:
  -h, --help  Show context-sensitive help.
  --format  Output format. (default: mp3)`

	if got := normalise(out.String()); got != golden {
		t.Errorf("StyledHelpPrinter() output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, golden)
	}
}
