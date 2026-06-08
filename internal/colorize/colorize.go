package colorize

import "strings"

// ANSI escape code constants. Each colorized line must end with Reset to
// prevent color bleeding into the next line.
const (
	Reset    = "\033[0m"
	Bold     = "\033[1m"
	Dim      = "\033[2m"
	Cyan     = "\033[36m"
	Yellow   = "\033[33m"
	Magenta  = "\033[35m"
	BoldCyan = "\033[1;36m"
)

// LineState tracks which multi-line region the colorizer is currently inside.
// It is passed by pointer through ColorizeLines so state persists across lines.
type LineState struct {
	InFrontmatter     bool
	FrontmatterClosed bool // true once the closing --- has been seen
	InCodeBlock       bool
}

// ColorizeLines takes a slice of raw markdown lines and returns a new slice
// where each line has appropriate ANSI escape codes applied.
// Lines that are already plain (no special syntax) are returned unchanged.
func ColorizeLines(lines []string) []string {
	state := &LineState{}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = colorizeLine(line, state, i)
	}
	return out
}

// colorizeLine applies ANSI color to a single line based on its content and
// the current LineState. lineIdx is 0-based and used only to detect the very
// first line for frontmatter detection.
func colorizeLine(line string, state *LineState, lineIdx int) string {
	trimmed := strings.TrimSpace(line)

	// --- Frontmatter delimiter handling ---
	if trimmed == "---" {
		if lineIdx == 0 && !state.FrontmatterClosed {
			// Opening frontmatter delimiter (must be first line)
			state.InFrontmatter = true
			return Magenta + line + Reset
		}
		if state.InFrontmatter && !state.FrontmatterClosed {
			// Closing frontmatter delimiter
			state.InFrontmatter = false
			state.FrontmatterClosed = true
			return Magenta + line + Reset
		}
		// A --- elsewhere is a thematic break — leave as-is
		return line
	}

	// Inside frontmatter block
	if state.InFrontmatter {
		return Magenta + line + Reset
	}

	// --- Code block fence ---
	if strings.HasPrefix(trimmed, "```") {
		state.InCodeBlock = !state.InCodeBlock
		return Dim + line + Reset
	}

	// Inside a code block
	if state.InCodeBlock {
		return Dim + line + Reset
	}

	// --- HTML comment (single-line or start of multi-line) ---
	if strings.Contains(line, "<!--") {
		return Yellow + line + Reset
	}

	// --- ATX headers (# through ######) ---
	if strings.HasPrefix(trimmed, "#") {
		return BoldCyan + line + Reset
	}

	// --- Bold/italic markers — subtle highlight ---
	// We don't render them, but we dim them slightly so they stand out
	// as syntax rather than content.
	if strings.Contains(line, "**") || strings.Contains(line, "__") ||
		strings.Contains(line, "*") || strings.Contains(line, "_") {
		return Dim + line + Reset
	}

	return line
}
