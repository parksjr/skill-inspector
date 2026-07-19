package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/parksjr/skill-inspector/internal/colorize"
	"github.com/parksjr/skill-inspector/internal/installer"
	"github.com/parksjr/skill-inspector/internal/loader"
	"github.com/parksjr/skill-inspector/internal/parser"
	"github.com/parksjr/skill-inspector/internal/tui"
)

// version is set at build time via ldflags.
var version = "dev"

// installFlag is set by the --install flag.
var installFlag bool

// baselinePath is set by --baseline <file>.
var baselinePath string

// outputBaselinePath is set by --output-baseline <file>.
var outputBaselinePath string

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

	var comparison *parser.Comparison
	if baselinePath != "" {
		bl, err := parser.LoadBaseline(baselinePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading baseline %q: %v\n", baselinePath, err)
			os.Exit(1)
		}
		comparison = bl.Compare(result)
	}

	if outputBaselinePath != "" {
		bl := parser.NewBaseline(result)
		if err := bl.Save(outputBaselinePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving baseline %q: %v\n", outputBaselinePath, err)
			os.Exit(1)
		}
		fmt.Printf("Baseline saved to %s (%d findings)\n", outputBaselinePath, len(bl.IDs))
	}

	if installFlag {
		runCLIInstall(sf)
		return
	}

	if err := tui.Run(sf, result, comparison); err != nil {
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
		case "--install":
			installFlag = true
		case "--check-deps":
			parser.EnableDeps = true
		case "--baseline":
			if i+1 < len(os.Args) {
				i++
				baselinePath = os.Args[i]
			}
		case "--output-baseline":
			if i+1 < len(os.Args) {
				i++
				outputBaselinePath = os.Args[i]
			}
		default:
			if strings.HasPrefix(os.Args[i], "--no-symlink-") {
				agent := strings.TrimPrefix(os.Args[i], "--no-symlink-")
				if agent != "" {
					installer.ExcludeAgent(agent)
				}
			} else {
				pathArgs = append(pathArgs, os.Args[i])
			}
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
  skill-inspector [flags] <url-or-file-path>

Flags:
  -h, --help              Show this help message
  -v, --version           Print version string
  --no-color              Disable ANSI color output (also NO_COLOR env var)
  --install               Install the skill after showing the plan preview
  --no-symlink-<agent>    Skip symlink creation for a specific agent
                          (e.g. --no-symlink-goose, --no-symlink-pi)
  --baseline <file>       Compare findings against a previous baseline
  --output-baseline <file> Save current findings as a baseline for future comparison
  --check-deps            Scan for package install references (advisory only)

Examples:
  skill-inspector ./my-skill/SKILL.md
  skill-inspector ./my-skill/
  skill-inspector --install ./my-skill/
  skill-inspector https://raw.githubusercontent.com/user/repo/main/skills/my-skill/SKILL.md
`)
}

// runCLIInstall shows the install plan, asks for confirmation, and runs the install.
// Used when the --install flag is passed.
func runCLIInstall(sf *loader.SkillFile) {
	preview, err := installer.PlanInstall(sf.SkillName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error planning install: %v\n", err)
		os.Exit(1)
	}

	_, missingName := parser.FrontmatterValue(sf.Parsed.Frontmatter, "name")

	fmt.Printf("Install %q?\n\n", preview.SkillName)
	if missingName {
		fmt.Printf("⚠ Warning: frontmatter is missing \"name\".\n")
		fmt.Printf("  Using fallback folder name: %s\n\n", preview.SkillName)
	}

	fmt.Printf("Files:\n  %s\n\n", preview.InstallPath)
	fmt.Printf("Planned symlinks:\n")
	for _, link := range preview.Links {
		if link.Available {
			fmt.Printf("  %s -> %s (%s)\n", link.Source, link.Destination, link.Agent)
		} else {
			fmt.Printf("  %s -> %s (%s missing: skipped)\n", link.Source, link.Destination, link.Agent)
		}
	}
	fmt.Print("\nConfirm: y = install, n/Enter = cancel\n")

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)
	if response != "y" && response != "Y" {
		fmt.Println("Install cancelled.")
		return
	}

	result, installErr := installer.Install(sf.SkillName, sf.SourcePath, sf.Content, sf.IsURL)
	if installErr != nil {
		fmt.Fprintf(os.Stderr, "✗ Install failed: %v\n", installErr)
		os.Exit(1)
	}

	fmt.Printf("✓ Installed to %s\n", result.InstallPath)
	for _, lr := range result.Links {
		switch {
		case lr.Err != nil:
			fmt.Printf("  ✗ Error  %-10s %v\n", lr.Agent, lr.Err)
		case lr.Skipped:
			fmt.Printf("  — Skipped %-10s (directory not found)\n", lr.Agent)
		case lr.Linked:
			fmt.Printf("  ✓ Linked  %-10s %s\n", lr.Agent, lr.Path)
		}
	}
}
