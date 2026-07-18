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
