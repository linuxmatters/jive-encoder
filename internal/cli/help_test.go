package cli

import (
	"bytes"
	"testing"

	"github.com/alecthomas/kong"
)

// helpFixture is a minimal CLI covering the flag shapes getFlags must handle.
type helpFixture struct {
	Format string `help:"Output format." default:"mp3"`
	Title  string `help:"Episode title."`
	Stereo bool   `help:"Encode in stereo."`
	Number int    `short:"n" help:"Episode number."`
}

// newFixtureContext builds a kong context that neither exits nor prints.
func newFixtureContext(t *testing.T) *kong.Context {
	t.Helper()

	var fixture helpFixture
	parser, err := kong.New(&fixture,
		kong.Exit(func(int) {}),
		kong.Writers(&bytes.Buffer{}, &bytes.Buffer{}),
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

// findFlag returns the extracted flag whose rendered form matches exactly.
func findFlag(t *testing.T, flags []flag, rendered string) flag {
	t.Helper()
	for _, f := range flags {
		if f.flags == rendered {
			return f
		}
	}
	t.Fatalf("flag %q not found in %+v", rendered, flags)
	return flag{}
}

func TestGetFlags(t *testing.T) {
	flags := getFlags(newFixtureContext(t))

	tests := []struct {
		name       string
		rendered   string
		help       string
		defaultVal string
	}{
		{"help flag always present", "-h, --help", "Show context-sensitive help.", ""},
		{"flag with default tag", "--format", "Output format.", "mp3"},
		// A defaultless flag must render no default clause. This guards
		// against reintroducing the placeholder-formatting helper, which
		// printed bogus values like "(default: STRING)".
		{"flag without default", "--title", "Episode title.", ""},
		{"bool flag has no default", "--stereo", "Encode in stereo.", ""},
		{"short name captured", "-n, --number", "Episode number.", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := findFlag(t, flags, tt.rendered)
			if f.help != tt.help {
				t.Errorf("help = %q; want %q", f.help, tt.help)
			}
			if f.defaultVal != tt.defaultVal {
				t.Errorf("defaultVal = %q; want %q", f.defaultVal, tt.defaultVal)
			}
		})
	}

	// The help flag leads the list; the fixture contributes the other four.
	if flags[0].flags != "-h, --help" {
		t.Errorf("flags[0] = %q; want %q", flags[0].flags, "-h, --help")
	}
	if len(flags) != 5 {
		t.Errorf("len(flags) = %d; want 5", len(flags))
	}
}

func TestGetArguments(t *testing.T) {
	// The fixture has no positional arguments, so none are extracted.
	if args := getArguments(newFixtureContext(t)); len(args) != 0 {
		t.Errorf("getArguments() = %+v; want none", args)
	}
}
