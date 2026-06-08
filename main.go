package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/parksjr/skill-inspector/internal/colorize"
	"github.com/parksjr/skill-inspector/internal/loader"
	"github.com/parksjr/skill-inspector/internal/parser"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: skill-inspector <url-or-file-path>\n")
		os.Exit(1)
	}

	input := os.Args[1]

	sf, err := loader.Load(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Skill: %s\n\n", sf.SkillName)

	// --- Source view (colorized) ---
	lines := strings.Split(sf.Content, "\n")
	colorized := colorize.ColorizeLines(lines)
	fmt.Println("=== Source ===")
	for _, line := range colorized {
		fmt.Println(line)
	}

	// --- Hidden content view ---
	result := parser.Parse(sf.Content)

	fmt.Println("\n=== Frontmatter ===")
	if result.Frontmatter == nil {
		fmt.Println("✓ None found")
	} else {
		fm := result.Frontmatter
		fmt.Printf("Lines %d–%d:\n", fm.StartLine, fm.EndLine)
		for i, l := range fm.Lines {
			fmt.Printf("  %d: %s\n", fm.StartLine+i, l)
		}
	}

	fmt.Println("\n=== HTML Comments ===")
	if len(result.HTMLComments) == 0 {
		fmt.Println("✓ None found")
	} else {
		for i, c := range result.HTMLComments {
			fmt.Printf("[%d] Lines %d–%d:\n%s\n", i+1, c.StartLine, c.EndLine, c.Raw)
		}
	}

	fmt.Println("\n=== Suspicious Characters ===")
	if len(result.SuspiciousChars) == 0 {
		fmt.Println("✓ None found")
	} else {
		for _, s := range result.SuspiciousChars {
			fmt.Println(s.Format())
		}
	}
}
