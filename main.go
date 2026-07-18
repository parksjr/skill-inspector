package main

import (
	"fmt"
	"os"

	"github.com/parksjr/skill-inspector/internal/colorize"
	"github.com/parksjr/skill-inspector/internal/loader"
	"github.com/parksjr/skill-inspector/internal/tui"
)

// version is set at build time via ldflags.
var version = "dev"

func main() {
	pathArgs := parseFlags()

	if len(pathArgs) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: skill-inspector <url-or-file-path>\n")
		os.Exit(1)
	}

	input := pathArgs[0]

	sf, err := loader.Load(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading skill: %v\n", err)
		os.Exit(1)
	}

	result := sf.Parsed

	if err := tui.Run(sf, result); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// parseFlags scans os.Args for flags, sets globals, and returns
// the non-flag positional arguments.
func parseFlags() []string {
	var pathArgs []string
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--help", "-h":
			printHelp()
			os.Exit(0)
		case "--version", "-v":
			fmt.Printf("skill-inspector version %s\n", version)
			os.Exit(0)
		case "--no-color":
			colorize.NoColor = true
		default:
			pathArgs = append(pathArgs, os.Args[i])
		}
	}

	// NO_COLOR env var (no-color.org spec): any value disables ANSI color.
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		colorize.NoColor = true
	}

	return pathArgs
}

func printHelp() {
	fmt.Print(`skill-inspector — audit agent skill files before installation

A minimal-dependency CLI/TUI tool for inspecting agent skill files.
Surfaces hidden content (HTML comments, YAML frontmatter, suspicious
Unicode characters) so you can audit what you're actually running.

Usage:
  skill-inspector <url-or-file-path>
  skill-inspector [flags]

Flags:
  -h, --help       Show this help message
  -v, --version    Print version string
  --no-color       Disable ANSI color output (also NO_COLOR env var)

Examples:
  skill-inspector ./my-skill/SKILL.md
  skill-inspector ./my-skill/
  skill-inspector https://raw.githubusercontent.com/user/repo/main/skills/my-skill/SKILL.md
`)
}
