package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/parksjr/skill-inspector/internal/colorize"
	"github.com/parksjr/skill-inspector/internal/installer"
)

func TestBuildInstallPreviewLinesShowsMissingNameAlert(t *testing.T) {
	preview := &installer.InstallPreview{
		SkillName:   "SKILL",
		InstallPath: filepath.Join("/home/test", ".agents", "skills", "SKILL"),
		Links: []installer.PlannedLink{
			{
				Agent:       "claude",
				Source:      filepath.Join("/home/test", ".agents", "skills", "SKILL"),
				Destination: filepath.Join("/home/test", ".claude", "skills", "SKILL"),
				Available:   true,
			},
		},
	}

	lines := buildInstallPreviewLines(preview, true)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, `frontmatter is missing "name"`) {
		t.Fatalf("expected missing-name warning in preview lines")
	}
	if !strings.Contains(joined, preview.InstallPath) {
		t.Fatalf("expected install path %q in preview lines", preview.InstallPath)
	}
}

func TestBuildInstallPreviewLinesOmitsAlertWhenNamePresent(t *testing.T) {
	preview := &installer.InstallPreview{
		SkillName:   "my-skill",
		InstallPath: filepath.Join("/home/test", ".agents", "skills", "my-skill"),
	}

	lines := buildInstallPreviewLines(preview, false)
	joined := strings.Join(lines, "\n")

	if strings.Contains(joined, `frontmatter is missing "name"`) {
		t.Fatalf("did not expect missing-name warning when frontmatter name exists")
	}
}

// =============================================================================
// maxScrollOffset
// =============================================================================

func TestMaxScrollOffset(t *testing.T) {
	tests := []struct {
		name   string
		lines  []string
		height int
		want   int
	}{
		{"empty lines", nil, 24, 0},
		{"fewer lines than content height", []string{"a", "b"}, 10, 0},
		{"exactly fits", []string{"a", "b", "c"}, 5, 0},           // 5-2=3 content rows, 3 lines → max=0
		{"one line overflow", []string{"a", "b", "c", "d"}, 5, 1}, // 3 content rows, 4 lines → max=1
		{"many lines overflow", make([]string, 100), 24, 78},      // 22 content rows, 100 lines → max=78
		{"single line tall terminal", []string{"x"}, 100, 0},
		{"negative after subtract", []string{"a"}, 4, 0}, // 4-2=2 content rows, 1 line → max=-1 → 0
		{"exact zero", []string{"a", "b"}, 4, 0},         // 2 content rows, 2 lines → max=0
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := maxScrollOffset(tc.lines, tc.height)
			if got != tc.want {
				t.Errorf("maxScrollOffset(%d lines, h=%d) = %d, want %d",
					len(tc.lines), tc.height, got, tc.want)
			}
		})
	}
}

// =============================================================================
// padRight
// =============================================================================

func TestPadRight(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"needs padding", "hello", 10, "hello     "},
		{"exact width", "hello", 5, "hello"},
		{"wider than width", "hello world", 5, "hello world"},
		{"empty string padded", "", 5, "     "},
		{"zero width", "hello", 0, "hello"},
		{"negative width", "hello", -1, "hello"},
		{"ANSI content shorter", "\033[1mbold\033[0m", 9, "\033[1mbold\033[0m     "},
		{"ANSI content exact", "\033[1mhi\033[0m", 2, "\033[1mhi\033[0m"},
		{"ANSI content wider", "\033[1mhello\033[0m", 3, "\033[1mhello\033[0m"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := padRight(tc.input, tc.width)
			if got != tc.want {
				t.Errorf("padRight(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
			}
			// Visible length after padding should be at least width (unless input was wider).
			visible := stripANSI(got)
			if len(visible) < tc.width && len(stripANSI(tc.input)) < tc.width {
				t.Errorf("padRight result visible len=%d, expected at least %d", len(visible), tc.width)
			}
		})
	}
}

// =============================================================================
// stripANSI — edge cases
// =============================================================================

func TestStripANSI_PlainText(t *testing.T) {
	tests := []string{
		"",
		"hello",
		"line with spaces",
		"12345",
		"!@#$%^&*()",
	}
	for _, input := range tests {
		got := stripANSI(input)
		if got != input {
			t.Errorf("stripANSI(%q) = %q, want unchanged", input, got)
		}
	}
}

func TestStripANSI_CSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"SGR bold", "\033[1mbold\033[0m", "bold"},
		{"color cyan", "\033[36mcyan\033[0m", "cyan"},
		{"cursor movement", "\033[2J\033[H", ""},
		{"cursor position", "\033[10;20H", ""},
		{"erase line", "\033[K", ""},
		{"bold cyan combo", "\033[1;36mtext\033[0m", "text"},
		{"mid-string CSI", "pre\033[1mBOLD\033[0mpost", "preBOLDpost"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripANSI(tc.input)
			if got != tc.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestStripANSI_OSC(t *testing.T) {
	// OSC: ESC ] ... BEL or ESC \
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"window title BEL", "\033]0;My Title\007hello", "hello"},
		{"window title ST", "\033]0;Title\033\\after", "after"},
		{"empty OSC", "\033]\007", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripANSI(tc.input)
			if got != tc.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestStripANSI_DCS_SOS_PM_APC(t *testing.T) {
	// DCS (P), SOS (X), PM (^), APC (_): ESC type ... (BEL or ST)
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"DCS BEL", "\033Pdata\007after", "after"},
		{"SOS ST", "\033Xhidden\033\\after", "after"},
		{"PM BEL", "\033^msg\007after", "after"},
		{"APC ST", "\033_cmd\033\\after", "after"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripANSI(tc.input)
			if got != tc.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestStripANSI_StandaloneESC(t *testing.T) {
	// Stand-alone ESC byte (not followed by a recognized introducer).
	got := stripANSI("\033xplain")
	if got != "\033xplain" {
		t.Errorf("standalone ESC should be preserved: got %q", got)
	}

	// Single ESC at end of string.
	got = stripANSI("end\033")
	if got != "end\033" {
		t.Errorf("trailing ESC should be preserved: got %q", got)
	}
}

func TestStripANSI_MultiByteBoundaries(t *testing.T) {
	// Multi-byte UTF-8 characters should not be corrupted.
	got := stripANSI("日本語\033[1m太字\033[0mテスト")
	if got != "日本語太字テスト" {
		t.Errorf("multi-byte UTF-8 should be preserved: got %q", got)
	}

	// Emoji (4-byte UTF-8)
	got = stripANSI("🎉\033[31m🚀\033[0m✨")
	if got != "🎉🚀✨" {
		t.Errorf("emoji should be preserved: got %q", got)
	}
}

func TestStripANSI_UnclosedCSI(t *testing.T) {
	// CSI without a final byte: ESC [ 3 1 — no terminating byte in range 0x40-0x7E
	// The for loop advances past the params but then i >= len(s), so the final byte
	// consume is skipped. This should still strip the incomplete sequence.
	got := stripANSI("\033[31")
	if got != "" {
		t.Errorf("unclosed CSI should be stripped: got %q", got)
	}
}

func TestStripANSI_UnclosedOSC(t *testing.T) {
	// OSC without a terminator (BEL or ST) consumes everything after ESC ].
	got := stripANSI("before\033]0;Titleafter")
	want := "before"
	if got != want {
		t.Errorf("unclosed OSC consumes remaining: got %q, want %q", got, want)
	}
}

func TestStripANSI_MultipleSequences(t *testing.T) {
	input := "\033[1m\033[36mBold Cyan\033[0m\033]0;Title\007normal"
	want := "Bold Cyannormal"
	got := stripANSI(input)
	if got != want {
		t.Errorf("multiple sequences: got %q, want %q", got, want)
	}
}

// =============================================================================
// truncateLine
// =============================================================================

func TestTruncateLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		wantEnds string // the string should end with this
	}{
		{"no truncation needed", "hello", 10, "hello"},
		{"truncation adds reset", "hello world", 5, colorize.Reset},
		{"very narrow: returns unchanged when maxWidth <= 3", "hello world", 1, "hello world"},
		{"min width boundary", "abc", 3, "abc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateLine(tc.input, tc.maxWidth)
			if tc.wantEnds == colorize.Reset {
				if !strings.HasSuffix(got, colorize.Reset) {
					t.Errorf("truncated line should end with Reset: got %q", got)
				}
				// Visible length should be <= maxWidth
				visible := stripANSI(got)
				if len([]rune(visible)) > tc.maxWidth {
					t.Errorf("visible length %d exceeds maxWidth %d", len([]rune(visible)), tc.maxWidth)
				}
			} else if got != tc.wantEnds {
				t.Errorf("truncateLine(%q, %d) = %q, want %q", tc.input, tc.maxWidth, got, tc.wantEnds)
			}
		})
	}
}

func TestTruncateLine_ANSI(t *testing.T) {
	// Line with ANSI codes — visible length determines truncation.
	input := "\033[1mhello world\033[0m" // visible = "hello world" = 11 chars
	got := truncateLine(input, 5)
	if !strings.HasSuffix(got, colorize.Reset) {
		t.Errorf("truncated ANSI line should end with Reset: got %q", got)
	}
	visible := stripANSI(got)
	if len([]rune(visible)) != 5 {
		t.Errorf("expected 5 visible chars after truncation, got %d: %q", len([]rune(visible)), visible)
	}
}

func TestTruncateLine_MultiByteUTF8(t *testing.T) {
	// 日本語extra = 8 visible characters. maxWidth=4 → truncates after 4 runes.
	// maxWidth <= 3 returns unchanged (guard in truncateLine).
	input := "日本語extra"
	got := truncateLine(input, 4)
	if !strings.HasSuffix(got, colorize.Reset) {
		t.Errorf("truncated UTF-8 should end with Reset: got %q", got)
	}
	visible := stripANSI(got)
	if len([]rune(visible)) != 4 {
		t.Errorf("expected 4 visible runes (日本語e), got %d: %q", len([]rune(visible)), visible)
	}
}
