package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/parksjr/skill-inspector/internal/loader"
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

	lines := strings.Split(sf.Content, "\n")
	fmt.Printf("Skill: %s\n", sf.SkillName)
	fmt.Printf("--- first 5 lines ---\n")
	for i, line := range lines {
		if i >= 5 {
			break
		}
		fmt.Println(line)
	}
}
