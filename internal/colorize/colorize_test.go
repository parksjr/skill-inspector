package colorize

import (
	"strings"
	"testing"
)

// =============================================================================
// ColorizeLines integration tests (exercises colorizeLine through the public API)
// =============================================================================

func TestColorizeLines_NoColor(t *testing.T) {
	// When NoColor is true, the input must be returned unchanged.
	origNoColor := NoColor
	NoColor = true
	defer func() { NoColor = origNoColor }()

	input := []string{"# Heading", "plain text", "```go", "code", "```"}
	got := ColorizeLines(input)

	if len(got) != len(input) {
		t.Fatalf("expected %d lines, got %d", len(input), len(got))
	}
	for i := range input {
		if got[i] != input[i] {
			t.Errorf("line %d: expected %q, got %q", i, input[i], got[i])
		}
	}
}

func TestColorizeLines_ResetTermination(t *testing.T) {
	// Every colorized line must end with Reset to prevent bleeding.
	lines := []string{
		"---",
		"name: test",
		"---",
		"# Heading",
		"```go",
		"code here",
		"```",
		"<!-- comment -->",
		"**bold**",
		"plain",
	}
	got := ColorizeLines(lines)

	for i, line := range got {
		if line == "" {
			continue
		}
		// Plain lines are returned as-is; colorized lines must end with Reset.
		if strings.Contains(line, "\033[") && !strings.HasSuffix(line, Reset) {
			t.Errorf("line %d: colorized line does not end with Reset: %q", i, line)
		}
	}
}

func TestColorizeLines_PlainTextPassthrough(t *testing.T) {
	tests := []string{
		"Just a normal line of text.",
		"",
		"  leading spaces",
		"trailing spaces  ",
		"- bullet list",
		"1. numbered list",
		"> blockquote",
		"[link](https://example.com)",
		"`inline code` should pass through",
	}
	for _, input := range tests {
		lines := []string{input}
		got := ColorizeLines(lines)
		if got[0] != input {
			t.Errorf("plain line should pass through unchanged: input=%q, got=%q", input, got[0])
		}
	}
}

// =============================================================================
// Headings
// =============================================================================

func TestColorizeLines_Headings(t *testing.T) {
	tests := []struct {
		input string
		name  string
	}{
		{"# H1", "ATX h1"},
		{"## H2", "ATX h2"},
		{"### H3", "ATX h3"},
		{"#### H4", "ATX h4"},
		{"##### H5", "ATX h5"},
		{"###### H6", "ATX h6"},
		{"  # indented heading", "indented h1"},
		{"  ## indented h2", "indented h2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ColorizeLines([]string{tc.input})
			if !strings.Contains(got[0], BoldCyan) {
				t.Errorf("heading should contain BoldCyan: got %q", got[0])
			}
			if !strings.HasSuffix(got[0], Reset) {
				t.Errorf("heading should end with Reset: got %q", got[0])
			}
		})
	}
}

func TestColorizeLines_HeadingPrecedence(t *testing.T) {
	// A heading containing bold markers should still be colored as a heading
	// (heading check runs before bold/italic check).
	got := ColorizeLines([]string{"# **Bold Heading**"})
	if !strings.Contains(got[0], BoldCyan) {
		t.Errorf("heading with bold markers should use BoldCyan: got %q", got[0])
	}
}

// =============================================================================
// Code fences and code blocks
// =============================================================================

func TestColorizeLines_CodeFence(t *testing.T) {
	lines := []string{"```go", "func main() {}", "```"}
	got := ColorizeLines(lines)

	for i := range got {
		if !strings.Contains(got[i], Dim) {
			t.Errorf("line %d should contain Dim: %q", i, got[i])
		}
		if !strings.HasSuffix(got[i], Reset) {
			t.Errorf("line %d should end with Reset: %q", i, got[i])
		}
	}
}

func TestColorizeLines_CodeBlockNested(t *testing.T) {
	// Nested code fences: ``` outer, ``` inner — each toggles.
	// First ``` enters code block, second ``` (inside) exits it,
	// third ``` re-enters.
	lines := []string{
		"```",
		"outer code",
		"```",
		"not code",
		"```",
		"more code",
		"```",
	}
	got := ColorizeLines(lines)

	// Line 0: enters code block → Dim
	if !strings.Contains(got[0], Dim) {
		t.Errorf("line 0 (opening fence): expected Dim, got %q", got[0])
	}
	// Line 1: inside code block → Dim
	if !strings.Contains(got[1], Dim) {
		t.Errorf("line 1 (code content): expected Dim, got %q", got[1])
	}
	// Line 2: exits code block → Dim (the fence itself is Dim)
	if !strings.Contains(got[2], Dim) {
		t.Errorf("line 2 (closing fence): expected Dim, got %q", got[2])
	}
	// Line 3: outside code block → plain
	if got[3] != "not code" {
		t.Errorf("line 3 (outside block): expected unchanged, got %q", got[3])
	}
	// Line 4: re-enters code block → Dim
	if !strings.Contains(got[4], Dim) {
		t.Errorf("line 4 (re-opening fence): expected Dim, got %q", got[4])
	}
	// Line 5: inside again → Dim
	if !strings.Contains(got[5], Dim) {
		t.Errorf("line 5 (code again): expected Dim, got %q", got[5])
	}
	// Line 6: exits again → Dim
	if !strings.Contains(got[6], Dim) {
		t.Errorf("line 6 (closing fence again): expected Dim, got %q", got[6])
	}
}

func TestColorizeLines_CodeFenceBacktickCount(t *testing.T) {
	// Only lines STARTING with ``` are code fences.
	lines := []string{"not a ``` fence", "plain text"}
	got := ColorizeLines(lines)
	if got[0] != "not a ``` fence" {
		t.Errorf("inline backticks should not trigger code fence: got %q", got[0])
	}
	if got[1] != "plain text" {
		t.Errorf("line after inline backticks should be plain: got %q", got[1])
	}
}

// =============================================================================
// Frontmatter
// =============================================================================

func TestColorizeLines_Frontmatter(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
	}{
		{"standard frontmatter", []string{"---", "name: test", "version: 1.0", "---", "plain text"}},
		{"minimal frontmatter", []string{"---", "---", "plain text"}},
		{"empty line in frontmatter", []string{"---", "", "name: test", "---", "after"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ColorizeLines(tc.lines)
			// Opening delimiter (line 0)
			if !strings.Contains(got[0], Magenta) {
				t.Errorf("opening --- should use Magenta: %q", got[0])
			}
			// Content lines (lines 1 through len-3) — all inside frontmatter
			for i := 1; i < len(tc.lines)-2; i++ {
				if !strings.Contains(got[i], Magenta) {
					t.Errorf("frontmatter content line %d should use Magenta: %q", i, got[i])
				}
			}
			// Closing delimiter
			closeIdx := len(tc.lines) - 2
			if !strings.Contains(got[closeIdx], Magenta) {
				t.Errorf("closing --- should use Magenta: %q", got[closeIdx])
			}
			// Line after frontmatter should be plain
			afterIdx := len(tc.lines) - 1
			if got[afterIdx] != tc.lines[afterIdx] {
				t.Errorf("line after frontmatter should be plain: got %q", got[afterIdx])
			}
		})
	}
}

func TestColorizeLines_FrontmatterNotAtStart(t *testing.T) {
	// --- only opens frontmatter when it's the first line (lineIdx == 0)
	lines := []string{"plain first line", "---", "should be plain", "---"}
	got := ColorizeLines(lines)

	if got[0] != "plain first line" {
		t.Errorf("line 0 should be plain: got %q", got[0])
	}
	// --- at line 1 (not line 0) → treated as thematic break, not frontmatter
	if got[1] != "---" {
		t.Errorf("--- not at position 0 should be plain: got %q", got[1])
	}
	if got[2] != "should be plain" {
		t.Errorf("content between non-frontmatter --- should be plain: got %q", got[2])
	}
	if got[3] != "---" {
		t.Errorf("trailing --- should be plain: got %q", got[3])
	}
}

func TestColorizeLines_FrontmatterUnclosed(t *testing.T) {
	// Opening --- without closing ---: everything after is frontmatter.
	lines := []string{"---", "name: test", "# not a heading in frontmatter", "```not a code fence```"}
	got := ColorizeLines(lines)

	for i := 0; i < len(lines); i++ {
		if !strings.Contains(got[i], Magenta) {
			t.Errorf("line %d in unclosed frontmatter should use Magenta: %q", i, got[i])
		}
	}
}

func TestColorizeLines_FrontmatterLaterSeparator(t *testing.T) {
	// After frontmatter is closed, --- is a thematic break (plain).
	lines := []string{"---", "name: test", "---", "text", "---"}
	got := ColorizeLines(lines)

	if !strings.Contains(got[0], Magenta) {
		t.Errorf("opening ---: expected Magenta, got %q", got[0])
	}
	if !strings.Contains(got[1], Magenta) {
		t.Errorf("frontmatter content: expected Magenta, got %q", got[1])
	}
	if !strings.Contains(got[2], Magenta) {
		t.Errorf("closing ---: expected Magenta, got %q", got[2])
	}
	// Line 4: --- after frontmatter closed → thematic break, plain
	if got[4] != "---" {
		t.Errorf("--- after frontmatter should be plain: got %q", got[4])
	}
}

// =============================================================================
// HTML Comments
// =============================================================================

func TestColorizeLines_HTMLComment(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"single-line comment", "<!-- this is a comment -->"},
		{"multi-line start", "<!-- start of multi-line comment"},
		{"comment with content after", "text <!-- hidden --> more text"},
		{"empty comment", "<!---->"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ColorizeLines([]string{tc.input})
			if !strings.Contains(got[0], Yellow) {
				t.Errorf("HTML comment line should contain Yellow: got %q", got[0])
			}
			if !strings.HasSuffix(got[0], Reset) {
				t.Errorf("HTML comment line should end with Reset: got %q", got[0])
			}
		})
	}
}

func TestColorizeLines_HTMLCommentNoColoringWithoutMarker(t *testing.T) {
	// Lines without <!-- should not be yellow even if they have -->
	got := ColorizeLines([]string{"just some --> text"})
	if strings.Contains(got[0], Yellow) {
		t.Errorf("line with --> but no <!-- should not be Yellow: got %q", got[0])
	}
}

// =============================================================================
// Bold markers (** and __)
// =============================================================================

func TestColorizeLines_Bold(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"double asterisk", "this is **bold** text"},
		{"double underscore", "this is __bold__ text"},
		{"bold at start", "**bold** at the beginning"},
		{"bold at end", "end with **bold**"},
		{"only bold", "**only bold**"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ColorizeLines([]string{tc.input})
			if !strings.Contains(got[0], Dim) {
				t.Errorf("bold line should contain Dim: got %q", got[0])
			}
		})
	}
}

func TestColorizeLines_BoldFalsePositives(t *testing.T) {
	// ** not adjacent to text content should not trigger bold.
	// But the current logic is simply strings.Contains(line, "**") — so
	// it WILL trigger for any **. This test documents actual behavior.
	t.Skip("current implementation flags any line with ** as bold — known limitation (LAUNCH-READINESS M8)")
}

// =============================================================================
// Italic markers (* and _) — only when adjacent to non-space text
// =============================================================================

func TestColorizeLines_Italic(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"single asterisk emphasis", "this is *emphasized* text"},
		{"single underscore emphasis", "this is _emphasized_ text"},
		{"asterisk at start", "*emphasized* at start"},
		{"asterisk at end", "end with *emphasized*"},
		{"underscore at start", "_partial emphasis_"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ColorizeLines([]string{tc.input})
			if !strings.Contains(got[0], Dim) {
				t.Errorf("italic line should contain Dim: got %q", got[0])
			}
		})
	}
}

func TestColorizeLines_ItalicFalsePositives(t *testing.T) {
	// These patterns should NOT trigger emphasis because the * / _ are
	// not adjacent to non-space text on at least one side.
	tests := []struct {
		name  string
		input string
	}{
		{"bullet list asterisk", "* bullet item"},
		{"bullet list underscore", "_ bullet item"},
		{"multiplication math", "a * b = c"},
		{"space before asterisk", "not * emphasized"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ColorizeLines([]string{tc.input})
			if got[0] != tc.input {
				t.Errorf("line should pass through unchanged: expected %q, got %q", tc.input, got[0])
			}
		})
	}
}

func TestColorizeLines_ItalicKnownFalsePositives(t *testing.T) {
	// These patterns trigger emphasis but arguably shouldn't:
	// "not* emphasized" — * is adjacent to 't' on the left, so emphasis fires.
	// "this_is_snake_case" — _ is adjacent to text on both sides.
	// This documents actual behavior (known limitation LAUNCH-READINESS M8).
	tests := []string{
		"not* emphasized",
		"this_is_snake_case",
	}
	for _, input := range tests {
		got := ColorizeLines([]string{input})
		if !strings.Contains(got[0], Dim) {
			t.Errorf("known false positive: %q should be Dim, got %q", input, got[0])
		}
	}
}

func TestColorizeLines_AsteriskAtWordBoundary(t *testing.T) {
	// Single * with text on one side only — should still be emphasis.
	got := ColorizeLines([]string{"partial *emphasis"})
	if !strings.Contains(got[0], Dim) {
		t.Errorf("partial *emphasis should be Dim: got %q", got[0])
	}
}

func TestColorizeLines_UnderscoreInURL(t *testing.T) {
	// URL with underscores — each _ has non-space on both sides,
	// so it's treated as emphasis. This is a known limitation but
	// we test actual behavior.
	// Actually: in "https://example_com/path" each _ is adjacent to
	// text, so the current implementation treats it as emphasis.
	// This test documents that behavior.
	got := ColorizeLines([]string{"https://example_com/path"})
	if !strings.Contains(got[0], Dim) {
		t.Errorf("underscore in URL currently triggers emphasis: got %q", got[0])
	}
}

// =============================================================================
// Bold takes precedence over italic when both are present
// =============================================================================

func TestColorizeLines_BoldPrecedenceOverItalic(t *testing.T) {
	// When ** is present, italic check is skipped entirely.
	// A line with both ** and non-italic _ should still be Dim (from bold).
	got := ColorizeLines([]string{"**bold** and _snake_case"})
	if !strings.Contains(got[0], Dim) {
		t.Errorf("bold line should contain Dim: got %q", got[0])
	}
}

// =============================================================================
// Frontmatter takes precedence over everything
// =============================================================================

func TestColorizeLines_FrontmatterPrecedence(t *testing.T) {
	// Inside frontmatter, nothing else matters — no headings, code fences,
	// HTML comments, or bold/italic detection.
	lines := []string{
		"---",
		"# not a heading in frontmatter",
		"```not a code fence```",
		"<!-- not a comment in frontmatter -->",
		"**not bold** in frontmatter",
		"---",
	}
	got := ColorizeLines(lines)

	for i := 0; i < len(lines); i++ {
		if !strings.Contains(got[i], Magenta) {
			t.Errorf("line %d in frontmatter should be Magenta: got %q", i, got[i])
		}
	}
}

// =============================================================================
// Code block takes precedence over headings and HTML comments
// =============================================================================

func TestColorizeLines_CodeBlockPrecedence(t *testing.T) {
	lines := []string{
		"```",
		"# not a heading in code block",
		"<!-- not a comment in code block -->",
		"**not bold** in code block",
		"```",
	}
	got := ColorizeLines(lines)

	for i := 0; i < len(lines); i++ {
		if !strings.Contains(got[i], Dim) {
			t.Errorf("line %d in code block should be Dim: got %q", i, got[i])
		}
	}
}

// =============================================================================
// Multi-line document integration
// =============================================================================

func TestColorizeLines_FullDocument(t *testing.T) {
	lines := []string{
		"---",
		"name: my-skill",
		"description: A test skill",
		"---",
		"",
		"# My Skill",
		"",
		"This is **important** and *emphasized* text.",
		"",
		"## Usage",
		"",
		"```go",
		"func main() {",
		`	fmt.Println("hello")`,
		"}",
		"```",
		"",
		"<!-- Hidden note: this is safe -->",
		"",
		"Just plain text.",
		"---",
	}

	got := ColorizeLines(lines)

	// Frontmatter (lines 0-3): Magenta
	for i := 0; i <= 3; i++ {
		if !strings.Contains(got[i], Magenta) {
			t.Errorf("line %d (frontmatter): expected Magenta, got %q", i, got[i])
		}
	}

	// Line 4: empty line → plain
	if got[4] != "" {
		t.Errorf("line 4 (empty): expected empty, got %q", got[4])
	}

	// Line 5: # My Skill → BoldCyan
	if !strings.Contains(got[5], BoldCyan) {
		t.Errorf("line 5 (heading): expected BoldCyan, got %q", got[5])
	}

	// Line 7: bold + italic → Dim
	if !strings.Contains(got[7], Dim) {
		t.Errorf("line 7 (bold+italic): expected Dim, got %q", got[7])
	}

	// Line 9: ## Usage → BoldCyan
	if !strings.Contains(got[9], BoldCyan) {
		t.Errorf("line 9 (heading): expected BoldCyan, got %q", got[9])
	}

	// Lines 11-15: code block → Dim
	for i := 11; i <= 15; i++ {
		if !strings.Contains(got[i], Dim) {
			t.Errorf("line %d (code block): expected Dim, got %q", i, got[i])
		}
	}

	// Line 17: HTML comment → Yellow
	if !strings.Contains(got[17], Yellow) {
		t.Errorf("line 17 (HTML comment): expected Yellow, got %q", got[17])
	}

	// Line 19: plain text → unchanged
	if got[19] != "Just plain text." {
		t.Errorf("line 19 (plain): expected unchanged, got %q", got[19])
	}

	// Line 20: --- after frontmatter closed → plain (thematic break)
	if got[20] != "---" {
		t.Errorf("line 20 (thematic break): expected unchanged ---, got %q", got[20])
	}
}

// =============================================================================
// Edge cases
// =============================================================================

func TestColorizeLines_EmptyInput(t *testing.T) {
	got := ColorizeLines(nil)
	// make([]string, 0) returns an empty (non-nil) slice.
	if len(got) != 0 {
		t.Errorf("nil input should return empty slice, got len=%d", len(got))
	}

	got = ColorizeLines([]string{})
	if len(got) != 0 {
		t.Errorf("empty input should return empty slice, got %v", got)
	}
}

func TestColorizeLines_SingleLine(t *testing.T) {
	got := ColorizeLines([]string{"hello"})
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("single plain line: expected [hello], got %v", got)
	}
}

func TestColorizeLines_ConsecutiveFences(t *testing.T) {
	// Back-to-back code fences: toggles on first, toggles off on second.
	lines := []string{"```a", "```b", "plain"}
	got := ColorizeLines(lines)

	if !strings.Contains(got[0], Dim) {
		t.Errorf("first fence should be Dim: got %q", got[0])
	}
	if !strings.Contains(got[1], Dim) {
		t.Errorf("second fence should be Dim: got %q", got[1])
	}
	if got[2] != "plain" {
		t.Errorf("after two fences (toggled twice): expected plain, got %q", got[2])
	}
}

func TestColorizeLines_HeadingWithHashOnly(t *testing.T) {
	// A line that is just "#" — trimmed starts with # so it's a heading.
	got := ColorizeLines([]string{"#"})
	if !strings.Contains(got[0], BoldCyan) {
		t.Errorf("bare # should be BoldCyan: got %q", got[0])
	}
}

func TestColorizeLines_TildeFencesNotCodeBlock(t *testing.T) {
	// ~~~ is not a code fence per our detection (only ```).
	lines := []string{"~~~", "content", "~~~"}
	got := ColorizeLines(lines)
	if got[0] != "~~~" {
		t.Errorf("~~~ should not trigger code fence: got %q", got[0])
	}
	if got[1] != "content" {
		t.Errorf("content after ~~~ should be plain: got %q", got[1])
	}
}

func TestColorizeLines_ItalicEdgePositions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		dim   bool
	}{
		// * at index 0: leftText=false (i==0), rightText=true → emphasis true
		{"asterisk position 0 adjacent right", "*foo*", true},
		// * at last position: leftText=true, rightText=false (i+1 == len) → emphasis true
		{"asterisk at end adjacent left", "foo*", true},
		// * surrounded by spaces: leftText=false, rightText=false → emphasis false
		{"asterisk with spaces both sides", "a * b", false},
		// _ surrounded by spaces
		{"underscore with spaces both sides", "a _ b", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ColorizeLines([]string{tc.input})
			hasDim := strings.Contains(got[0], Dim)
			if hasDim != tc.dim {
				t.Errorf("expected hasDim=%v for %q, got %q", tc.dim, tc.input, got[0])
			}
		})
	}
}
