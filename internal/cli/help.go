package cli

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/kong"
)

// Custom help styles using shared disco ball palette 🪩
var (
	helpModeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Italic(true).
			MarginBottom(1)

	helpSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(SecondaryColor).
				MarginTop(1)

	helpFlagStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	helpArgStyle = lipgloss.NewStyle().
			Foreground(SuccessColor).
			Bold(true)

	helpDefaultStyle = lipgloss.NewStyle().
				Foreground(MutedColor).
				Italic(true)
)

// StyledHelpPrinter creates a custom help printer with Lipgloss styling
func StyledHelpPrinter(options kong.HelpOptions) kong.HelpPrinter {
	return kong.HelpPrinter(func(options kong.HelpOptions, ctx *kong.Context) error {
		var sb strings.Builder

		// Title and description
		sb.WriteString(TitleStyle.Render("Jivedrop 🪩"))
		sb.WriteString("\n")
		sb.WriteString(helpDescStyle.Render("Drop your podcast .wav into a shiny MP3 with metadata, cover art, and all."))
		sb.WriteString("\n")

		// Usage
		sb.WriteString(helpSectionStyle.Render("Usage:"))
		sb.WriteString("\n  ")
		sb.WriteString(helpModeStyle.Render("Hugo mode:"))
		sb.WriteString("\n    ")
		fmt.Fprintf(&sb, "%s <audio-file> <episode-md> [flags]", ctx.Model.Name)
		sb.WriteString("\n  ")
		sb.WriteString(helpModeStyle.Render("Standalone mode:"))
		sb.WriteString("\n    ")
		fmt.Fprintf(&sb, "%s <audio-file> --title TEXT --num NUMBER --cover PATH [flags]", ctx.Model.Name)
		sb.WriteString("\n")

		// Arguments section
		args := getArguments(ctx)
		if len(args) > 0 {
			sb.WriteString("\n")
			sb.WriteString(helpSectionStyle.Render("Arguments:"))
			sb.WriteString("\n")
			for _, arg := range args {
				sb.WriteString("  ")
				sb.WriteString(helpArgStyle.Render(arg.name))
				if arg.help != "" {
					sb.WriteString("  ")
					sb.WriteString(arg.help)
				}
				sb.WriteString("\n")
			}
		}

		// Flags section
		flags := getFlags(ctx)
		if len(flags) > 0 {
			sb.WriteString("\n")
			sb.WriteString(helpSectionStyle.Render("Flags:"))
			sb.WriteString("\n")
			for _, flag := range flags {
				sb.WriteString("  ")
				sb.WriteString(helpFlagStyle.Render(flag.flags))
				if flag.help != "" {
					sb.WriteString("  ")
					sb.WriteString(flag.help)
				}
				if flag.defaultVal != "" {
					sb.WriteString(" ")
					sb.WriteString(helpDefaultStyle.Render("(default: " + flag.defaultVal + ")"))
				}
				sb.WriteString("\n")
			}
		}

		sb.WriteString("\n")
		fmt.Fprint(ctx.Stdout, sb.String())
		return nil
	})
}

// Argument represents a CLI argument
type argument struct {
	name string
	help string
}

// Flag represents a CLI flag
type flag struct {
	flags      string
	help       string
	defaultVal string
}

// getArguments extracts arguments from Kong context
func getArguments(ctx *kong.Context) []argument {
	var args []argument

	for _, arg := range ctx.Model.Positional {
		name := arg.Summary()
		help := arg.Help
		args = append(args, argument{name: name, help: help})
	}

	return args
}

// getFlags extracts flags from Kong context
func getFlags(ctx *kong.Context) []flag {
	var flags []flag

	// Always include help flag
	flags = append(flags, flag{
		flags: "-h, --help",
		help:  "Show context-sensitive help.",
	})

	for _, f := range ctx.Model.Flags {
		if f.Name == "help" {
			continue // Already added above.
		}

		var flagStr string
		if f.Short != 0 {
			flagStr = fmt.Sprintf("-%c, --%s", f.Short, f.Name)
		} else {
			flagStr = fmt.Sprintf("--%s", f.Name)
		}

		if !f.IsBool() && f.PlaceHolder != "" {
			flagStr += "=" + strings.ToUpper(f.PlaceHolder)
		}

		flags = append(flags, flag{
			flags:      flagStr,
			help:       f.Help,
			defaultVal: f.FormatPlaceHolder(),
		})
	}

	return flags
}
