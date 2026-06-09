package tui

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/parksjr/skill-inspector/internal/colorize"
	"github.com/parksjr/skill-inspector/internal/installer"
	"github.com/parksjr/skill-inspector/internal/loader"
	"github.com/parksjr/skill-inspector/internal/parser"
)

// view represents which panel is currently displayed.
type view int

const (
	viewSource view = iota
	viewHidden
)

// state holds all mutable TUI state.
type state struct {
	currentView  view
	scrollOffset int
	width        int
	height       int
}

// ANSI control sequences.
const (
	clearScreen    = "\033[2J"
	moveCursorHome = "\033[H"
	hideCursor     = "\033[?25l"
	showCursor     = "\033[?25h"
	invertOn       = "\033[7m"
	invertOff      = "\033[0m"
	boldOn         = "\033[1m"
	resetAll       = "\033[0m"
)

// Run is the entry point for the TUI. It loads the skill file, parses it,
// enters raw terminal mode, and runs the interactive pager loop.
// It returns a non-nil error only for unrecoverable failures.
func Run(sf *loader.SkillFile, result *parser.ParseResult) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("not a terminal — skill-inspector requires an interactive terminal")
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to enter raw terminal mode: %w", err)
	}
	defer func() {
		_ = term.Restore(int(os.Stdin.Fd()), oldState)
		fmt.Print(showCursor)
	}()

	fmt.Print(hideCursor)

	s := &state{currentView: viewSource}
	s.updateSize()

	sourceLines := buildSourceLines(sf)
	hiddenLines := buildHiddenLines(result)

	for {
		s.updateSize()
		lines := sourceLines
		if s.currentView == viewHidden {
			lines = hiddenLines
		}
		s.render(sf.SkillName, lines)

		action := readKey()
		switch action {
		case actionQuit:
			fmt.Print(clearScreen + moveCursorHome)
			return nil
		case actionToggleView:
			if s.currentView == viewSource {
				s.currentView = viewHidden
			} else {
				s.currentView = viewSource
			}
			s.scrollOffset = 0
		case actionScrollDown:
			maxScroll := maxScrollOffset(lines, s.height)
			if s.scrollOffset < maxScroll {
				s.scrollOffset++
			}
		case actionScrollUp:
			if s.scrollOffset > 0 {
				s.scrollOffset--
			}
		case actionPageDown:
			maxScroll := maxScrollOffset(lines, s.height)
			s.scrollOffset += s.height - 2
			if s.scrollOffset > maxScroll {
				s.scrollOffset = maxScroll
			}
		case actionPageUp:
			s.scrollOffset -= s.height - 2
			if s.scrollOffset < 0 {
				s.scrollOffset = 0
			}
		case actionInstall:
			runInstall(sf, s)
		}
	}
}

// action represents a user input action.
type action int

const (
	actionNone action = iota
	actionQuit
	actionToggleView
	actionScrollDown
	actionScrollUp
	actionPageDown
	actionPageUp
	actionInstall
)

// readKey reads a single keypress (or escape sequence) from stdin and maps it
// to an action. Blocks until a key is available.
func readKey() action {
	buf := make([]byte, 4)
	n, err := os.Stdin.Read(buf)
	if err != nil || n == 0 {
		return actionNone
	}

	switch {
	case n == 1 && buf[0] == 'q':
		return actionQuit
	case n == 1 && buf[0] == 3: // Ctrl+C
		return actionQuit
	case n == 1 && buf[0] == '\t':
		return actionToggleView
	case n == 1 && buf[0] == 'j':
		return actionScrollDown
	case n == 1 && buf[0] == 'k':
		return actionScrollUp
	case n == 1 && buf[0] == 'i':
		return actionInstall
	case n == 1 && buf[0] == ' ':
		return actionPageDown
	case n == 1 && buf[0] == 'b':
		return actionPageUp
	case n >= 3 && buf[0] == 27 && buf[1] == '[' && buf[2] == 'A':
		return actionScrollUp
	case n >= 3 && buf[0] == 27 && buf[1] == '[' && buf[2] == 'B':
		return actionScrollDown
	case n >= 4 && buf[0] == 27 && buf[1] == '[' && buf[2] == '5' && buf[3] == '~':
		return actionPageUp
	case n >= 4 && buf[0] == 27 && buf[1] == '[' && buf[2] == '6' && buf[3] == '~':
		return actionPageDown
	case n == 1 && buf[0] == 27:
		return actionNone
	}

	return actionNone
}

// updateSize refreshes terminal dimensions. Falls back to 80x24 on error.
func (s *state) updateSize() {
	w, h, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil || w < 10 || h < 4 {
		s.width, s.height = 80, 24
		return
	}
	s.width, s.height = w, h
}

// render clears the screen and draws the current view.
func (s *state) render(skillName string, lines []string) {
	var sb strings.Builder

	sb.WriteString(moveCursorHome)

	viewLabel := "SOURCE"
	if s.currentView == viewHidden {
		viewLabel = "HIDDEN CONTENT"
	}
	header := fmt.Sprintf(" skill-inspector  │  %s  │  %s ", skillName, viewLabel)
	if len(header) > s.width {
		header = header[:s.width]
	}
	header = padRight(header, s.width)
	sb.WriteString(invertOn + header + invertOff + "\r\n")

	contentHeight := s.height - 2
	visibleLines := lines
	if s.scrollOffset < len(visibleLines) {
		visibleLines = visibleLines[s.scrollOffset:]
	} else {
		visibleLines = nil
	}

	drawn := 0
	for _, line := range visibleLines {
		if drawn >= contentHeight {
			break
		}
		printLine := truncateLine(line, s.width)
		sb.WriteString(printLine + "\033[K\r\n")
		drawn++
	}
	for drawn < contentHeight {
		sb.WriteString("\033[K\r\n")
		drawn++
	}

	total := len(lines)
	pct := 0
	if total > 0 {
		bottom := s.scrollOffset + contentHeight
		if bottom > total {
			bottom = total
		}
		pct = bottom * 100 / total
	}
	footer := fmt.Sprintf(" [Tab] toggle view  [j/k] scroll  [Space/b] page  [i] install  [q] quit  %d%% (%d lines)", pct, total)
	if len(footer) > s.width {
		footer = footer[:s.width]
	}
	footer = padRight(footer, s.width)
	sb.WriteString(invertOn + footer + invertOff)

	fmt.Print(sb.String())
}

// buildSourceLines returns the ANSI-colorized source lines for the source view.
func buildSourceLines(sf *loader.SkillFile) []string {
	raw := strings.Split(sf.Content, "\n")
	return colorize.ColorizeLines(raw)
}

// buildHiddenLines constructs the hidden-content view as a slice of display lines.
func buildHiddenLines(result *parser.ParseResult) []string {
	var lines []string
	add := func(s string) { lines = append(lines, s) }

	add(boldOn + "── Frontmatter " + strings.Repeat("─", 60) + resetAll)
	if result.Frontmatter == nil {
		add("  ✓ None found")
	} else {
		fm := result.Frontmatter
		add(fmt.Sprintf("  Lines %d–%d:", fm.StartLine, fm.EndLine))
		for i, l := range fm.Lines {
			add(fmt.Sprintf("  %3d │ %s", fm.StartLine+i, l))
		}
	}
	add("")

	add(boldOn + "── HTML Comments " + strings.Repeat("─", 58) + resetAll)
	if len(result.HTMLComments) == 0 {
		add("  ✓ None found")
	} else {
		for i, c := range result.HTMLComments {
			add(fmt.Sprintf("  [%d] Lines %d–%d:", i+1, c.StartLine, c.EndLine))
			for _, l := range strings.Split(c.Raw, "\n") {
				add("      " + l)
			}
			add("")
		}
	}
	add("")

	add(boldOn + "── Suspicious Characters " + strings.Repeat("─", 50) + resetAll)
	if len(result.SuspiciousChars) == 0 {
		add("  ✓ None found")
	} else {
		for _, sc := range result.SuspiciousChars {
			add("  " + sc.Format())
		}
	}
	add("")

	return lines
}

// runInstall handles the interactive install confirmation and execution.
// It runs within raw terminal mode — all I/O uses the raw terminal directly.
func runInstall(sf *loader.SkillFile, s *state) {
	home, _ := os.UserHomeDir()
	installPath := home + "/.agents/skills/" + sf.SkillName

	// Show confirmation prompt in footer area.
	prompt := fmt.Sprintf(" Install %q → %s ? [y/N] ", sf.SkillName, installPath)
	if len(prompt) > s.width {
		prompt = prompt[:s.width]
	}
	fmt.Print("\033[999;1H" + "\033[7m" + padRight(prompt, s.width) + "\033[0m")

	buf := make([]byte, 1)
	os.Stdin.Read(buf) //nolint:errcheck
	if buf[0] != 'y' && buf[0] != 'Y' {
		// Cancelled — redraw will clear the prompt on next render cycle.
		return
	}

	// Run the install.
	result, err := installer.Install(sf.SkillName, sf.SourcePath, sf.Content, sf.IsURL)
	showInstallResult(result, err, s)
}

// showInstallResult displays the install outcome and waits for a keypress.
func showInstallResult(result *installer.InstallResult, installErr error, s *state) {
	var lines []string
	if installErr != nil {
		lines = append(lines, fmt.Sprintf(" ✗ Install failed: %v", installErr))
	} else {
		lines = append(lines, fmt.Sprintf(" ✓ Installed to %s", result.InstallPath))
		for _, lr := range result.Links {
			switch {
			case lr.Err != nil:
				lines = append(lines, fmt.Sprintf("   ✗ Error  %-10s %v", lr.Agent, lr.Err))
			case lr.Skipped:
				lines = append(lines, fmt.Sprintf("   — Skipped %-10s (directory not found)", lr.Agent))
			case lr.Linked:
				lines = append(lines, fmt.Sprintf("   ✓ Linked  %-10s %s", lr.Agent, lr.Path))
			}
		}
	}
	lines = append(lines, " Press any key to continue…")

	// Display lines starting near the bottom of the screen.
	startRow := s.height - len(lines) - 1
	if startRow < 2 {
		startRow = 2
	}
	for i, line := range lines {
		if len(line) > s.width {
			line = line[:s.width]
		}
		fmt.Printf("\033[%d;1H\033[7m%s\033[0m", startRow+i, padRight(line, s.width))
	}

	buf := make([]byte, 1)
	os.Stdin.Read(buf) //nolint:errcheck
}

// maxScrollOffset returns the maximum valid scroll offset given total lines and
// terminal height.
func maxScrollOffset(lines []string, height int) int {
	contentHeight := height - 2
	max := len(lines) - contentHeight
	if max < 0 {
		return 0
	}
	return max
}

// padRight pads s with spaces on the right to exactly width characters.
// If s is already >= width, it is returned unchanged.
func padRight(s string, width int) string {
	visible := stripANSI(s)
	pad := width - len(visible)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

// stripANSI removes ANSI escape sequences from s for length calculation.
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && (s[i] < 'A' || s[i] > 'Z') && (s[i] < 'a' || s[i] > 'z') {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

// truncateLine returns the line truncated to maxWidth visible characters,
// preserving the Reset code at the end if the line was truncated.
func truncateLine(line string, maxWidth int) string {
	if len(stripANSI(line)) <= maxWidth {
		return line
	}
	if maxWidth > 3 && len(line) > maxWidth {
		return line[:maxWidth-1] + resetAll
	}
	return line
}
