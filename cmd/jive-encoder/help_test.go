package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/linuxmatters/jive-encoder/internal/cli"
)

// TestStyledHelpOutput renders --help against the real CLI struct through
// StyledHelpPrinter. It checks the default annotations and colour handling.
// The parser mirrors run(): same kong.Name, kong.Description, and kong.Help.
// Writers go to a buffer and exit is stubbed so help does not end the test.
func TestStyledHelpOutput(t *testing.T) {
	// CLI is package-level mutable state shared with other tests. Parsing
	// applies defaults to it, so snapshot and restore it around the parse.
	saved := CLI
	defer func() { CLI = saved }()

	var buf bytes.Buffer
	parser, err := kong.New(&CLI,
		kong.Name("jive-encoder"),
		kong.Description("Drop the mix, ship the show—metadata, cover art, and all."),
		kong.Help(cli.StyledHelpPrinter),
		kong.Writers(&buf, &buf),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}

	// --help triggers the help printer. The stubbed exit means Parse returns
	// normally afterwards; any residual parse error does not matter here.
	_, _ = parser.Parse([]string{"--help"})

	out := buf.String()
	if out == "" {
		t.Fatal("expected help output, got empty buffer")
	}

	// Kong type names must never leak into the rendered defaults.
	for _, leaked := range []string{"(default: STRING)", "(default: BOOL)"} {
		if strings.Contains(out, leaked) {
			t.Errorf("help output contains leaked type default %q", leaked)
		}
	}

	// The buffer is not a TTY, so colorprofile must degrade to plain text.
	if strings.Contains(out, "\x1b[") {
		t.Error("help output contains ANSI escape sequences on a non-TTY writer")
	}

	stereoLine := findFlagLine(t, out, "--stereo")
	// --stereo has no Kong default. Its single "(default: mono)" comes from
	// the flag's help text, so a second occurrence means a rendered default.
	if got := strings.Count(stereoLine, "(default:"); got != 1 {
		t.Errorf("--stereo line has %d \"(default:\" occurrences, want 1: %q", got, stereoLine)
	}

	formatLine := findFlagLine(t, out, "--format")
	if !strings.Contains(formatLine, "(default: mp3)") {
		t.Errorf("--format line missing \"(default: mp3)\": %q", formatLine)
	}
}

// findFlagLine returns the help output line containing the given flag. It
// fails the test if the flag is absent.
func findFlagLine(t *testing.T, out, flag string) string {
	t.Helper()
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, flag) {
			return line
		}
	}
	t.Fatalf("help output has no line containing %q", flag)
	return ""
}
