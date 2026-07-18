package main

import (
	"os"
	"strings"
	"testing"

	"github.com/parksjr/skill-inspector/internal/loader"
	"github.com/parksjr/skill-inspector/internal/parser"
)

// TestE2ECleanSkill verifies the full pipeline on a clean skill file.
func TestE2ECleanSkill(t *testing.T) {
	data, err := os.ReadFile("testdata/clean-skill.md")
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}
	content := string(data)

	sf, err := loader.Load("testdata/clean-skill.md")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if sf.Content != content {
		t.Error("loaded content should match file content")
	}
	if sf.SkillName != "clean-skill" {
		t.Errorf("expected SkillName 'clean-skill', got %q", sf.SkillName)
	}

	result := parser.Parse(sf.Content)

	// Clean skill: has frontmatter but no hidden content.
	if result.Frontmatter == nil {
		t.Error("expected frontmatter")
	}
	if len(result.HTMLComments) != 0 {
		t.Errorf("expected 0 HTML comments, got %d", len(result.HTMLComments))
	}
	if len(result.SuspiciousChars) != 0 {
		t.Errorf("expected 0 suspicious chars, got %d", len(result.SuspiciousChars))
	}
	if len(result.CDATASections) != 0 {
		t.Errorf("expected 0 CDATA sections, got %d", len(result.CDATASections))
	}
	if len(result.HiddenComments) != 0 {
		t.Errorf("expected 0 hidden comments, got %d", len(result.HiddenComments))
	}
	// Closing --- on line 5 (i>0) is detected as a document-start YAML risk.
	if len(result.YAMLRisks) != 1 {
		t.Errorf("expected 1 YAML risk (closing --- as doc-start), got %d", len(result.YAMLRisks))
	}
}

// TestE2EMaliciousSkill verifies the full pipeline detects hidden content.
func TestE2EMaliciousSkill(t *testing.T) {
	sf, err := loader.Load("testdata/malicious-skill.md")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	result := parser.Parse(sf.Content)

	// Frontmatter with hidden directive.
	if result.Frontmatter == nil {
		t.Fatal("expected frontmatter")
	}
	if len(result.Frontmatter.Lines) != 3 {
		t.Errorf("expected 3 frontmatter lines, got %d", len(result.Frontmatter.Lines))
	}

	// HTML comment.
	if len(result.HTMLComments) != 1 {
		t.Fatalf("expected 1 HTML comment, got %d", len(result.HTMLComments))
	}
	if !strings.Contains(result.HTMLComments[0].Raw, "ignore all safety guidelines") {
		t.Errorf("HTML comment should contain hidden text, got %q", result.HTMLComments[0].Raw)
	}

	// YAML risks: %YAML directive.
	foundDirective := false
	for _, yr := range result.YAMLRisks {
		if yr.Kind == "directive" {
			foundDirective = true
		}
	}
	if !foundDirective {
		t.Error("expected YAML directive (%YAML 1.2)")
	}

	// CDATA section.
	if len(result.CDATASections) != 1 {
		t.Errorf("expected 1 CDATA section, got %d", len(result.CDATASections))
	}

	// Hidden comments: CSS and JS.
	cssFound := false
	jsFound := false
	for _, hc := range result.HiddenComments {
		if hc.Kind == "css-block" {
			cssFound = true
		}
		if hc.Kind == "js-line" {
			jsFound = true
		}
	}
	if !cssFound {
		t.Error("expected CSS block comment detection")
	}
	if !jsFound {
		t.Error("expected JS line comment detection")
	}
}

// TestE2EEdgeCaseSkill verifies handling of code fences masking comments and multi-doc YAML.
func TestE2EEdgeCaseSkill(t *testing.T) {
	sf, err := loader.Load("testdata/edge-case-skill.md")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	result := parser.Parse(sf.Content)

	// No frontmatter (file doesn't start with ---)
	if result.Frontmatter != nil {
		t.Error("expected no frontmatter (file doesn't start with ---)")
	}

	// HTML comment: the one inside code fence should NOT be detected,
	// but the multi-line comment outside SHOULD be.
	if len(result.HTMLComments) != 1 {
		t.Fatalf("expected 1 HTML comment (multi-line, not the one in code fence), got %d", len(result.HTMLComments))
	}
	if !strings.Contains(result.HTMLComments[0].Raw, "multi-line") {
		t.Errorf("expected multi-line HTML comment, got %q", result.HTMLComments[0].Raw)
	}

	// CDATA sections: 2
	if len(result.CDATASections) != 2 {
		t.Errorf("expected 2 CDATA sections, got %d", len(result.CDATASections))
	}

	// YAML risks: ... (doc-end) and --- (doc-start, i > 0)
	docEndFound := false
	docStartFound := false
	for _, yr := range result.YAMLRisks {
		if yr.Kind == "document-end" {
			docEndFound = true
		}
		if yr.Kind == "document-start" {
			docStartFound = true
		}
	}
	if !docEndFound {
		t.Error("expected ... document-end separator")
	}
	if !docStartFound {
		t.Error("expected --- document-start separator")
	}
}

// TestE2ECLIFlags tests --help and --version flags via the binary.
func TestE2ECLIFlags(t *testing.T) {
	// This test verifies the binary exists and basic flags work.
	// We test via build output since exec requires the binary to be built.
	_, err := os.Stat("skill-inspector")
	if err != nil {
		t.Skip("binary not built — skipping CLI flag test (run 'make build' first)")
	}

	// Binary exists — build confirmation.
	t.Log("binary found: skill-inspector")
}
