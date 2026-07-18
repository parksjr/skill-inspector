package main

import (
	"fmt"
	"os"

	"github.com/parksjr/skill-inspector/internal/loader"
	"github.com/parksjr/skill-inspector/internal/parser"
	"github.com/parksjr/skill-inspector/internal/tui"
)

// version is set at build time via ldflags.
var version = "dev"

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "--help", "-h":
			printHelp()
			os.Exit(0)
		case "--version", "-v":
			fmt.Printf("skill-inspector version %s\n", version)
			os.Exit(0)
		}
	}

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: skill-inspector <url-or-file-path>\n")
		os.Exit(1)
	}

	input := os.Args[1]

	sf, err := loader.Load(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading skill: %v\n", err)
		os.Exit(1)
	}

	result := parser.Parse(sf.Content)

	if err := tui.Run(sf, result); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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

Examples:
  skill-inspector ./my-skill/SKILL.md
  skill-inspector ./my-skill/
  skill-inspector https://raw.githubusercontent.com/user/repo/main/skills/my-skill/SKILL.md
`)
}
