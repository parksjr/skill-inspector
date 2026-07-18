package parser

import (
	"strings"
	"testing"
)

func TestExtractHTMLCommentsSkipsFencedCodeBlocks(t *testing.T) {
	input := strings.Join([]string{
		"# My Skill",
		"",
		"<!-- This is a real comment -->",
		"",
		"```go",
		"// <!-- this is inside a code fence, not a real comment -->",
		"```",
		"",
		"Real content here.",
	}, "\n")

	result := Parse(input)
	if len(result.HTMLComments) != 1 {
		t.Fatalf("expected 1 HTML comment, got %d: %+v", len(result.HTMLComments), result.HTMLComments)
	}
	if !strings.Contains(result.HTMLComments[0].Raw, "This is a real comment") {
		t.Fatalf("expected 'This is a real comment', got %q", result.HTMLComments[0].Raw)
	}
}

func TestExtractHTMLCommentsSkipsTildeFences(t *testing.T) {
	input := strings.Join([]string{
		"# My Skill",
		"",
		"~~~bash",
		"# <!-- not a comment -->",
		"echo hello",
		"~~~",
		"",
		"<!-- real comment here -->",
	}, "\n")

	result := Parse(input)
	if len(result.HTMLComments) != 1 {
		t.Fatalf("expected 1 HTML comment, got %d: %+v", len(result.HTMLComments), result.HTMLComments)
	}
}

func TestExtractHTMLCommentsSkipsInlineCode(t *testing.T) {
	input := strings.Join([]string{
		"# My Skill",
		"",
		"Use `<!-- notacomment -->` in code.",
		"",
		"<!-- real comment -->",
	}, "\n")

	result := Parse(input)
	if len(result.HTMLComments) != 1 {
		t.Fatalf("expected 1 HTML comment, got %d: %+v", len(result.HTMLComments), result.HTMLComments)
	}
	if !strings.Contains(result.HTMLComments[0].Raw, "real comment") {
		t.Fatalf("expected 'real comment', got %q", result.HTMLComments[0].Raw)
	}
}

func TestExtractHTMLCommentsEmptyFile(t *testing.T) {
	result := Parse("")
	if len(result.HTMLComments) != 0 {
		t.Fatalf("expected 0 HTML comments, got %d", len(result.HTMLComments))
	}
}

func TestExtractHTMLCommentsNoComments(t *testing.T) {
	result := Parse("# Just a header\n\nSome content.\n")
	if len(result.HTMLComments) != 0 {
		t.Fatalf("expected 0 HTML comments, got %d", len(result.HTMLComments))
	}
}

// =============================================================================
// maskCodeBlocks
// =============================================================================

func TestMaskCodeBlocks_BacktickFence(t *testing.T) {
	input := "text\n```\n<!-- hidden -->\n```\nafter"
	got := maskCodeBlocks(input)
	if strings.Contains(got, "<!--") {
		t.Errorf("<!-- inside ``` fence should be masked, got %q", got)
	}
	if !strings.Contains(got, "after") {
		t.Errorf("content after closing fence should be preserved")
	}
}

func TestMaskCodeBlocks_TildeFence(t *testing.T) {
	input := "text\n~~~\n<!-- hidden -->\n~~~\nafter"
	got := maskCodeBlocks(input)
	if strings.Contains(got, "<!--") {
		t.Errorf("<!-- inside ~~~ fence should be masked, got %q", got)
	}
}

func TestMaskCodeBlocks_InlineCode(t *testing.T) {
	input := "Use `<!-- not a comment -->` here.\n<!-- real -->"
	got := maskCodeBlocks(input)
	// Inline code `<!-- not a comment -->` should be masked.
	// The real <!-- real --> should remain.
	if !strings.Contains(got, "<!-- real -->") {
		t.Errorf("real comment should remain, got %q", got)
	}
}

func TestMaskCodeBlocks_InlineCodeCrossesLineBreaks(t *testing.T) {
	// Inline code ending on next line: `...\n...` — should not be treated as inline.
	input := "text `backtick\nnewline` end"
	got := maskCodeBlocks(input)
	if !strings.Contains(got, "backtick") {
		t.Errorf("backtick crossing newlines should not be masked: got %q", got)
	}
}

func TestMaskCodeBlocks_BacktickCountVarying(t *testing.T) {
	// ````` (5 backticks) is a valid fence.
	input := "text\n`````\n<!-- hidden -->\n`````\nafter"
	got := maskCodeBlocks(input)
	if strings.Contains(got, "<!--") {
		t.Errorf("<!-- inside ````` fence should be masked, got %q", got)
	}
}

func TestMaskCodeBlocks_NestedFences(t *testing.T) {
	// Outer 4-backtick fence, inner 3-backtick fence.
	input := "````\nouter\n```\ninner\n```\nouter2\n````\nafter"
	got := maskCodeBlocks(input)
	// Outer content masked. "after" is outside: preserved.
	if !strings.Contains(got, "after") {
		t.Errorf("content after nested fences should be preserved, got %q", got)
	}
}

func TestMaskCodeBlocks_LessThanThreeBackticks(t *testing.T) {
	// `` (2 backticks) is NOT a code fence.
	input := "``not a fence``\n<!-- real -->"
	got := maskCodeBlocks(input)
	if !strings.Contains(got, "<!-- real -->") {
		t.Errorf("<!-- with 2 backticks should not be masked, got %q", got)
	}
}

func TestMaskCodeBlocks_TrailingWhitespaceOnFence(t *testing.T) {
	// ```   (trailing spaces) is a valid fence.
	input := "```   \n<!-- hidden -->\n```\nafter"
	got := maskCodeBlocks(input)
	if strings.Contains(got, "<!--") {
		t.Errorf("<!-- inside fence with trailing spaces should be masked, got %q", got)
	}
}

// =============================================================================
// extractCDATASections
// =============================================================================

func TestExtractCDATASections_SingleLine(t *testing.T) {
	input := "<![CDATA[hidden content]]>"
	result := Parse(input)
	if len(result.CDATASections) != 1 {
		t.Fatalf("expected 1 CDATA section, got %d", len(result.CDATASections))
	}
	if result.CDATASections[0].Raw != input {
		t.Errorf("expected raw %q, got %q", input, result.CDATASections[0].Raw)
	}
	if result.CDATASections[0].StartLine != 1 {
		t.Errorf("expected StartLine 1, got %d", result.CDATASections[0].StartLine)
	}
}

func TestExtractCDATASections_MultiLine(t *testing.T) {
	input := "line1\n<![CDATA[\nsecret\npayload\n]]>\nline5"
	result := Parse(input)
	if len(result.CDATASections) != 1 {
		t.Fatalf("expected 1 CDATA section, got %d", len(result.CDATASections))
	}
	if result.CDATASections[0].StartLine != 2 {
		t.Errorf("expected StartLine 2, got %d", result.CDATASections[0].StartLine)
	}
	if result.CDATASections[0].EndLine != 5 {
		t.Errorf("expected EndLine 5, got %d", result.CDATASections[0].EndLine)
	}
}

func TestExtractCDATASections_Unclosed(t *testing.T) {
	input := "<![CDATA[never closed"
	result := Parse(input)
	if len(result.CDATASections) != 1 {
		t.Fatalf("expected 1 CDATA section (unclosed), got %d", len(result.CDATASections))
	}
	if result.CDATASections[0].EndLine != 1 {
		t.Errorf("unclosed CDATA should have EndLine 1, got %d", result.CDATASections[0].EndLine)
	}
}

func TestExtractCDATASections_Multiple(t *testing.T) {
	input := "<![CDATA[first]]>\n<![CDATA[second]]>"
	result := Parse(input)
	if len(result.CDATASections) != 2 {
		t.Fatalf("expected 2 CDATA sections, got %d", len(result.CDATASections))
	}
}

func TestExtractCDATASections_None(t *testing.T) {
	input := "plain text with no CDATA"
	result := Parse(input)
	if len(result.CDATASections) != 0 {
		t.Errorf("expected 0 CDATA sections, got %d", len(result.CDATASections))
	}
}

// =============================================================================
// extractHiddenComments (JS // and CSS /* */)
// =============================================================================

func TestExtractHiddenComments_JSLineComment(t *testing.T) {
	input := "// this is a JS line comment"
	result := Parse(input)
	if len(result.HiddenComments) != 1 {
		t.Fatalf("expected 1 hidden comment, got %d", len(result.HiddenComments))
	}
	if result.HiddenComments[0].Kind != "js-line" {
		t.Errorf("expected kind js-line, got %q", result.HiddenComments[0].Kind)
	}
}

func TestExtractHiddenComments_JSWithURLProtocolExclusion(t *testing.T) {
	// http:// and https:// should not trigger JS line comment detection.
	input := "Visit https://example.com for more\n// real JS comment"
	result := Parse(input)
	if len(result.HiddenComments) != 1 {
		t.Fatalf("expected 1 hidden comment (URL excluded, real comment found), got %d", len(result.HiddenComments))
	}
	if !strings.Contains(result.HiddenComments[0].Raw, "real JS comment") {
		t.Errorf("expected 'real JS comment', got %q", result.HiddenComments[0].Raw)
	}
}

func TestExtractHiddenComments_JSURLProtocolEdgeCases(t *testing.T) {
	// Protocol must have : immediately before // and idx >= 5.
	tests := []struct {
		name  string
		input string
		count int // expected hidden comment count
	}{
		{"https protocol", "https://example.com", 0},
		{"http protocol", "http://example.com", 0},
		{"ftp protocol", "ftp://server/path", 1}, // idx=4 (< 5) → not excluded
		{"not a protocol", "foo//bar", 1},        // // not preceded by :
		{"short prefix detected", "a://x", 1},    // idx=1 (< 5) → not excluded
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Parse(tc.input)
			if len(result.HiddenComments) != tc.count {
				t.Errorf("expected %d hidden comments, got %d", tc.count, len(result.HiddenComments))
			}
		})
	}
}

func TestExtractHiddenComments_CSSBlockComment(t *testing.T) {
	input := "/* CSS block comment */"
	result := Parse(input)
	if len(result.HiddenComments) != 1 {
		t.Fatalf("expected 1 hidden comment, got %d", len(result.HiddenComments))
	}
	if result.HiddenComments[0].Kind != "css-block" {
		t.Errorf("expected kind css-block, got %q", result.HiddenComments[0].Kind)
	}
}

func TestExtractHiddenComments_CSSMultiLine(t *testing.T) {
	input := "line1\n/* start\nmiddle\nend */\nline5"
	result := Parse(input)
	if len(result.HiddenComments) != 1 {
		t.Fatalf("expected 1 CSS comment, got %d", len(result.HiddenComments))
	}
	if result.HiddenComments[0].StartLine != 2 {
		t.Errorf("expected StartLine 2, got %d", result.HiddenComments[0].StartLine)
	}
	if result.HiddenComments[0].EndLine != 4 {
		t.Errorf("expected EndLine 4, got %d", result.HiddenComments[0].EndLine)
	}
}

func TestExtractHiddenComments_CSSUnclosed(t *testing.T) {
	input := "/* never closed"
	result := Parse(input)
	if len(result.HiddenComments) != 1 {
		t.Fatalf("expected 1 unclosed CSS comment, got %d", len(result.HiddenComments))
	}
	if result.HiddenComments[0].EndLine != 1 {
		t.Errorf("unclosed CSS should have EndLine 1, got %d", result.HiddenComments[0].EndLine)
	}
}

func TestExtractHiddenComments_CSSMultiple(t *testing.T) {
	input := "/* first */ text /* second */"
	result := Parse(input)
	if len(result.HiddenComments) != 2 {
		t.Fatalf("expected 2 CSS comments, got %d", len(result.HiddenComments))
	}
}

// =============================================================================
// extractYAMLRisks
// =============================================================================

func TestExtractYAMLRisks_YAMLDirective(t *testing.T) {
	input := "%YAML 1.2\n---\nname: test\n---"
	result := Parse(input)
	if len(result.YAMLRisks) < 1 {
		t.Fatalf("expected at least 1 YAML risk, got %d", len(result.YAMLRisks))
	}
	hasDirective := false
	for _, risk := range result.YAMLRisks {
		if risk.Kind == "directive" {
			hasDirective = true
			if risk.Line != 1 {
				t.Errorf("YAML directive should be on line 1, got %d", risk.Line)
			}
		}
	}
	if !hasDirective {
		t.Error("expected a YAML directive risk")
	}
}

func TestExtractYAMLRisks_DocumentEndSeparator(t *testing.T) {
	input := "content\n...\nmore"
	result := Parse(input)
	hasDocEnd := false
	for _, risk := range result.YAMLRisks {
		if risk.Kind == "document-end" {
			hasDocEnd = true
			if risk.Line != 2 {
				t.Errorf("... separator should be on line 2, got %d", risk.Line)
			}
		}
	}
	if !hasDocEnd {
		t.Error("expected a document-end YAML risk")
	}
}

func TestExtractYAMLRisks_MultiDocStart(t *testing.T) {
	// Additional --- beyond line 1 is a document-start separator.
	input := "---\nname: first\n---\nname: second"
	result := Parse(input)
	hasDocStart := false
	for _, risk := range result.YAMLRisks {
		if risk.Kind == "document-start" {
			hasDocStart = true
			if risk.Line != 3 {
				t.Errorf("multi-doc --- should be on line 3, got %d", risk.Line)
			}
		}
	}
	if !hasDocStart {
		t.Error("expected a document-start YAML risk (multi-doc ---)")
	}
}

func TestExtractYAMLRisks_NoRisksInPlainText(t *testing.T) {
	input := "# Just a header\nSome content.\n"
	result := Parse(input)
	if len(result.YAMLRisks) != 0 {
		t.Errorf("expected 0 YAML risks, got %d", len(result.YAMLRisks))
	}
}

func TestExtractYAMLRisks_Combined(t *testing.T) {
	input := "%YAML 1.2\n---\nkey: val\n...\n---\nkey2: val2"
	result := Parse(input)
	// directive (line 1), doc-start (line 2, i>0), doc-end (line 4), doc-start (line 5)
	if len(result.YAMLRisks) != 4 {
		t.Fatalf("expected 4 YAML risks, got %d: %+v", len(result.YAMLRisks), result.YAMLRisks)
	}
}

// =============================================================================
// FrontmatterValue edge cases
// =============================================================================

func TestFrontmatterValue_NilFrontmatter(t *testing.T) {
	val, ok := FrontmatterValue(nil, "name")
	if ok {
		t.Error("nil frontmatter should return ok=false")
	}
	if val != "" {
		t.Errorf("expected empty string, got %q", val)
	}
}

func TestFrontmatterValue_SimpleKey(t *testing.T) {
	fm := &Frontmatter{Lines: []string{"name: my-skill", "version: 1.0"}}
	val, ok := FrontmatterValue(fm, "name")
	if !ok {
		t.Error("expected ok=true for existing key")
	}
	if val != "my-skill" {
		t.Errorf("expected 'my-skill', got %q", val)
	}
}

func TestFrontmatterValue_MissingKey(t *testing.T) {
	fm := &Frontmatter{Lines: []string{"name: my-skill"}}
	_, ok := FrontmatterValue(fm, "description")
	if ok {
		t.Error("expected ok=false for missing key")
	}
}

func TestFrontmatterValue_EmptyValue(t *testing.T) {
	fm := &Frontmatter{Lines: []string{"name:"}}
	_, ok := FrontmatterValue(fm, "name")
	if ok {
		t.Error("expected ok=false for empty value ('' after trimming)")
	}
}

func TestFrontmatterValue_WhitespaceOnlyValue(t *testing.T) {
	fm := &Frontmatter{Lines: []string{"name:   "}}
	_, ok := FrontmatterValue(fm, "name")
	if ok {
		t.Error("expected ok=false for whitespace-only value")
	}
}

func TestFrontmatterValue_DoubleQuotedValue(t *testing.T) {
	fm := &Frontmatter{Lines: []string{`name: "quoted value"`}}
	val, ok := FrontmatterValue(fm, "name")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if val != "quoted value" {
		t.Errorf("expected 'quoted value', got %q", val)
	}
}

func TestFrontmatterValue_SingleQuotedValue(t *testing.T) {
	fm := &Frontmatter{Lines: []string{"name: 'single quoted'"}}
	val, ok := FrontmatterValue(fm, "name")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if val != "single quoted" {
		t.Errorf("expected 'single quoted', got %q", val)
	}
}

func TestFrontmatterValue_EmptyQuotes(t *testing.T) {
	fm := &Frontmatter{Lines: []string{`name: ""`}}
	_, ok := FrontmatterValue(fm, "name")
	if ok {
		t.Error("expected ok=false for empty double-quoted value")
	}

	fm2 := &Frontmatter{Lines: []string{"name: ''"}}
	_, ok2 := FrontmatterValue(fm2, "name")
	if ok2 {
		t.Error("expected ok=false for empty single-quoted value")
	}
}

func TestFrontmatterValue_CommentLines(t *testing.T) {
	fm := &Frontmatter{Lines: []string{"# this is a comment", "name: my-skill"}}
	val, ok := FrontmatterValue(fm, "name")
	if !ok {
		t.Fatal("comment line should be skipped")
	}
	if val != "my-skill" {
		t.Errorf("expected 'my-skill', got %q", val)
	}
}

func TestFrontmatterValue_EmptyLines(t *testing.T) {
	fm := &Frontmatter{Lines: []string{"", "  ", "name: my-skill"}}
	val, ok := FrontmatterValue(fm, "name")
	if !ok {
		t.Fatal("empty lines should be skipped")
	}
	if val != "my-skill" {
		t.Errorf("expected 'my-skill', got %q", val)
	}
}

func TestFrontmatterValue_CaseInsensitiveKey(t *testing.T) {
	fm := &Frontmatter{Lines: []string{"NAME: my-skill"}}
	val, ok := FrontmatterValue(fm, "name")
	if !ok {
		t.Fatal("key matching should be case-insensitive")
	}
	if val != "my-skill" {
		t.Errorf("expected 'my-skill', got %q", val)
	}
}

func TestFrontmatterValue_KeyWithSpaces(t *testing.T) {
	fm := &Frontmatter{Lines: []string{"  name  :  my-skill  "}}
	val, ok := FrontmatterValue(fm, "name")
	if !ok {
		t.Fatal("key with surrounding spaces should be found")
	}
	if val != "my-skill" {
		t.Errorf("expected 'my-skill', got %q", val)
	}
}

func TestFrontmatterValue_ColonInValue(t *testing.T) {
	// YAML value containing : — strings.Cut splits on first : only.
	fm := &Frontmatter{Lines: []string{"url: https://example.com"}}
	val, ok := FrontmatterValue(fm, "url")
	if !ok {
		t.Fatal("expected ok=true for key with colon in value")
	}
	if val != "https://example.com" {
		t.Errorf("expected 'https://example.com', got %q", val)
	}
}

func TestFrontmatterValue_YAMLLikeNestedKey(t *testing.T) {
	// Line like "key:\n  sub: val" — Cut on first : gives "key" and "\n  sub: val"
	// The key "key" won't match if searching for a specific sub-key.
	// This test ensures we don't crash on nested YAML.
	fm := &Frontmatter{Lines: []string{"nested: ", "  sub: value"}}
	_, ok := FrontmatterValue(fm, "nested")
	if ok {
		t.Error("nested key with empty trailing value should return false")
	}
}

func TestFrontmatterValue_SingleCharacterQuotes(t *testing.T) {
	// A single quote that starts but doesn't end — "value"
	// \"...\" check: starts with \", ends with \", len >= 2
	fm := &Frontmatter{Lines: []string{`name: "a"`}}
	val, ok := FrontmatterValue(fm, "name")
	if !ok {
		t.Fatal("single-char quoted value should be found")
	}
	if val != "a" {
		t.Errorf("expected 'a', got %q", val)
	}
}

func TestFrontmatterValue_UnquotedValueWithHash(t *testing.T) {
	// Unquoted value containing # should not be treated as comment.
	fm := &Frontmatter{Lines: []string{"tag: #important"}}
	val, ok := FrontmatterValue(fm, "tag")
	if !ok {
		t.Fatal("expected ok=true for unquoted value with #")
	}
	if val != "#important" {
		t.Errorf("expected '#important', got %q", val)
	}
}

// =============================================================================
// extractFrontmatter
// =============================================================================

func TestExtractFrontmatter_Standard(t *testing.T) {
	input := "---\nname: test\nversion: 1.0\n---\nbody"
	result := Parse(input)
	if result.Frontmatter == nil {
		t.Fatal("expected frontmatter")
	}
	if len(result.Frontmatter.Lines) != 2 {
		t.Errorf("expected 2 frontmatter lines, got %d", len(result.Frontmatter.Lines))
	}
	if result.Frontmatter.StartLine != 1 {
		t.Errorf("expected StartLine 1, got %d", result.Frontmatter.StartLine)
	}
	if result.Frontmatter.EndLine != 4 {
		t.Errorf("expected EndLine 4, got %d", result.Frontmatter.EndLine)
	}
}

func TestExtractFrontmatter_NoFrontmatter(t *testing.T) {
	input := "# No frontmatter here\nplain text"
	result := Parse(input)
	if result.Frontmatter != nil {
		t.Error("expected nil frontmatter when file doesn't start with ---")
	}
}

func TestExtractFrontmatter_Unclosed(t *testing.T) {
	input := "---\nname: test\nversion: 1.0"
	result := Parse(input)
	if result.Frontmatter != nil {
		t.Error("expected nil frontmatter when no closing ---")
	}
}

func TestExtractFrontmatter_EmptyFile(t *testing.T) {
	result := Parse("")
	if result.Frontmatter != nil {
		t.Error("expected nil frontmatter for empty file")
	}
}

func TestExtractFrontmatter_WhitespaceOnlyFirstLine(t *testing.T) {
	// First line is whitespace only — not "---" after trimming.
	input := "   \n---\nname: test\n---"
	result := Parse(input)
	if result.Frontmatter != nil {
		t.Error("expected nil frontmatter when file doesn't start with ---")
	}
}

func TestExtractFrontmatter_TrailingSpacesOnDelimiter(t *testing.T) {
	// "---   " (with trailing spaces) is valid after TrimSpace.
	input := "---   \nname: test\n---"
	result := Parse(input)
	if result.Frontmatter == nil {
		t.Fatal("frontmatter with trailing spaces on --- should be detected")
	}
}

// =============================================================================
// SuspiciousChar.Format
// =============================================================================

func TestSuspiciousChar_Format(t *testing.T) {
	sc := SuspiciousChar{
		Rune: 0x200B,
		Name: "ZERO WIDTH SPACE",
		Line: 14,
		Col:  7,
	}
	got := sc.Format()
	expected := "[U+200B ZERO WIDTH SPACE — line 14, col 7]"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// =============================================================================
// YAMLRisk.Format
// =============================================================================

func TestYAMLRisk_Format(t *testing.T) {
	tests := []struct {
		name     string
		risk     YAMLRisk
		expected string
	}{
		{
			"directive",
			YAMLRisk{Line: 1, Content: "%YAML 1.2", Kind: "directive"},
			"[line 1 — YAML %YAML 1.2]",
		},
		{
			"document-end",
			YAMLRisk{Line: 4, Content: "...", Kind: "document-end"},
			"[line 4 — YAML document separator: ...]",
		},
		{
			"document-start",
			YAMLRisk{Line: 5, Content: "---", Kind: "document-start"},
			"[line 5 — YAML document separator: --- (multi-doc start)]",
		},
		{
			"unknown kind",
			YAMLRisk{Line: 2, Content: "unknown", Kind: "bogus"},
			"[line 2 — YAML risk: unknown]",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.risk.Format()
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}
