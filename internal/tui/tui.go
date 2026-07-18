package tui

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"unicode/utf8"

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
	currentView   view
	scrollOffset  int
	width         int
	height        int
	wrappedSrc    []string
	wrappedHidden []string
	lastWrapWidth int
}

// ANSI control sequences.
const (
	clearScreen    = "\033[2J"
	moveCursorHome = "\033[H"
	enterAltScreen = "\033[?1049h"
	exitAltScreen  = "\033[?1049l"
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
		fmt.Print(showCursor + exitAltScreen)
		_ = term.Restore(int(os.Stdin.Fd()), oldState)
	}()

	fmt.Print(enterAltScreen + hideCursor)

	s := &state{currentView: viewSource}
	s.updateSize()

	sourceLines := buildSourceLines(sf)
	hiddenLines := buildHiddenLines(result)

	// Read keypresses in a dedicated goroutine so the main loop can also
	// react to SIGWINCH without waiting for a keypress.
	keyCh := make(chan action)
	go func() {
		for {
			keyCh <- readKey()
		}
	}()

	sigWinch := make(chan os.Signal, 1)
	signal.Notify(sigWinch, syscall.SIGWINCH)
	defer signal.Stop(sigWinch)

	for {
		s.updateSize()

		// Re-wrap content whenever the terminal width changes.
		if s.width != s.lastWrapWidth {
			s.wrappedSrc = wrapLines(sourceLines, s.width)
			s.wrappedHidden = wrapLines(hiddenLines, s.width)
			s.lastWrapWidth = s.width
			// Clamp scroll offset in case wrapping made content shorter.
			active := s.wrappedSrc
			if s.currentView == viewHidden {
				active = s.wrappedHidden
			}
			if max := maxScrollOffset(active, s.height); s.scrollOffset > max {
				s.scrollOffset = max
			}
		}

		lines := s.wrappedSrc
		if s.currentView == viewHidden {
			lines = s.wrappedHidden
		}
		s.render(sf.SkillName, lines)

		var act action
		select {
		case act = <-keyCh:
		case <-sigWinch:
			continue // re-render with updated size, no key action
		}

		switch act {
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
		case actionScrollTop:
			s.scrollOffset = 0
		case actionScrollBottom:
			s.scrollOffset = maxScrollOffset(lines, s.height)
		case actionInstall:
			runInstall(sf, result, s, keyCh)
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
	actionScrollTop
	actionScrollBottom
	actionInstall
	actionConfirmYes
	actionConfirmNo
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
	case n == 1 && (buf[0] == 'y' || buf[0] == 'Y'):
		return actionConfirmYes
	case n == 1 && (buf[0] == 'n' || buf[0] == 'N' || buf[0] == '\r' || buf[0] == '\n'):
		return actionConfirmNo
	case n == 1 && buf[0] == ' ':
		return actionPageDown
	case n == 1 && buf[0] == 'b':
		return actionPageUp
	case n == 1 && buf[0] == 'g':
		return actionScrollTop
	case n == 1 && buf[0] == 'G':
		return actionScrollBottom
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
		sb.WriteString(line + "\033[K\r\n")
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
func runInstall(sf *loader.SkillFile, parsed *parser.ParseResult, s *state, keyCh <-chan action) {
	preview, err := installer.PlanInstall(sf.SkillName)
	if err != nil {
		showInstallResult(nil, err, s, keyCh)
		return
	}

	_, hasFrontmatterName := parser.FrontmatterValue(parsed.Frontmatter, "name")
	showInstallPreviewModal(preview, !hasFrontmatterName, s)
	for {
		switch <-keyCh {
		case actionConfirmYes:
			result, installErr := installer.Install(sf.SkillName, sf.SourcePath, sf.Content, sf.IsURL)
			showInstallResult(result, installErr, s, keyCh)
			return
		case actionConfirmNo, actionQuit:
			return
		}
	}
}

func showInstallPreviewModal(preview *installer.InstallPreview, missingFrontmatterName bool, s *state) {
	lines := buildInstallPreviewLines(preview, missingFrontmatterName)
	renderModal(s, "Install confirmation", lines)
}

func buildInstallPreviewLines(preview *installer.InstallPreview, missingFrontmatterName bool) []string {
	lines := []string{
		fmt.Sprintf("Install %q?", preview.SkillName),
		"",
	}
	if missingFrontmatterName {
		lines = append(lines,
			`⚠ Warning: frontmatter is missing "name".`,
			fmt.Sprintf("  Using fallback folder name: %s", preview.SkillName),
			"",
		)
	}

	lines = append(lines,
		"Files:",
		fmt.Sprintf("  %s", preview.InstallPath),
		"",
		"Planned symlinks:",
	)
	for _, link := range preview.Links {
		entry := fmt.Sprintf("  %s -> %s (%s)", link.Source, link.Destination, link.Agent)
		if !link.Available {
			entry = fmt.Sprintf("  %s -> %s (%s missing: skipped)", link.Source, link.Destination, link.Agent)
		}
		lines = append(lines, entry)
	}
	lines = append(lines, "", "Confirm: y = install, n/Enter = cancel")
	return lines
}

func renderModal(s *state, title string, lines []string) {
	maxInnerWidth := len(stripANSI(title))
	for _, line := range lines {
		if l := len(stripANSI(line)); l > maxInnerWidth {
			maxInnerWidth = l
		}
	}

	maxAllowed := s.width - 6
	if maxAllowed < 20 {
		maxAllowed = 20
	}
	if maxInnerWidth > maxAllowed {
		maxInnerWidth = maxAllowed
	}
	if maxInnerWidth < 20 {
		maxInnerWidth = 20
	}

	maxLines := s.height - 6
	if maxLines < 1 {
		maxLines = 1
	}
	if len(lines) > maxLines {
		lines = append([]string{}, lines[:maxLines]...)
		lines[len(lines)-1] = "..."
	}

	boxWidth := maxInnerWidth + 4
	boxHeight := len(lines) + 4
	left := (s.width-boxWidth)/2 + 1
	top := (s.height-boxHeight)/2 + 1
	if left < 1 {
		left = 1
	}
	if top < 1 {
		top = 1
	}

	fmt.Printf("\033[%d;%dH%s", top, left, "┌"+strings.Repeat("─", boxWidth-2)+"┐")
	titleLine := "│ " + padRight(truncateLine(title, maxInnerWidth), maxInnerWidth) + " │"
	fmt.Printf("\033[%d;%dH%s%s%s", top+1, left, invertOn, titleLine, invertOff)
	for i, line := range lines {
		bodyLine := "│ " + padRight(truncateLine(line, maxInnerWidth), maxInnerWidth) + " │"
		fmt.Printf("\033[%d;%dH%s", top+2+i, left, bodyLine)
	}
	fmt.Printf("\033[%d;%dH%s", top+len(lines)+2, left, "└"+strings.Repeat("─", boxWidth-2)+"┘")
}

// showInstallResult displays the install outcome and waits for a keypress.
func showInstallResult(result *installer.InstallResult, installErr error, s *state, keyCh <-chan action) {
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

	<-keyCh
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

// wrapLine splits a single line (which may contain ANSI escape sequences) into
// segments of at most maxWidth visible characters, preserving escape sequences
// in the correct segment.
func wrapLine(line string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{line}
	}
	if len([]rune(stripANSI(line))) <= maxWidth {
		return []string{line}
	}

	var result []string
	var cur strings.Builder
	visible := 0
	i := 0

	for i < len(line) {
		// Consume ANSI escape sequence — not a visible character.
		if line[i] == '\033' && i+1 < len(line) && line[i+1] == '[' {
			j := i + 2
			for j < len(line) && (line[j] < 'A' || line[j] > 'Z') && (line[j] < 'a' || line[j] > 'z') {
				j++
			}
			if j < len(line) {
				j++
			}
			cur.WriteString(line[i:j])
			i = j
			continue
		}

		// Flush current segment and start a new one when width is reached.
		if visible == maxWidth {
			result = append(result, cur.String())
			cur.Reset()
			visible = 0
		}

		r, size := utf8.DecodeRuneInString(line[i:])
		_ = r
		cur.WriteString(line[i : i+size])
		visible++
		i += size
	}

	if cur.Len() > 0 {
		result = append(result, cur.String())
	}
	return result
}

// wrapLines expands every line in lines into one or more wrapped segments.
func wrapLines(lines []string, maxWidth int) []string {
	var out []string
	for _, l := range lines {
		out = append(out, wrapLine(l, maxWidth)...)
	}
	return out
}

// truncateLine returns the line truncated to maxWidth visible characters,
// preserving the Reset code at the end if the line was truncated.
// Used only for header/footer bars.
func truncateLine(line string, maxWidth int) string {
	if utf8.RuneCountInString(stripANSI(line)) <= maxWidth {
		return line
	}
	if maxWidth <= 3 {
		return line
	}

	// Walk the raw string counting visible runes (skipping ANSI escapes)
	// and stop at maxWidth-1 visible characters to leave room for the
	// resetAll marker. This avoids slicing through multi-byte runes.
	visible := 0
	i := 0
	for i < len(line) {
		if line[i] == '\033' && i+1 < len(line) && line[i+1] == '[' {
			i += 2
			for i < len(line) && (line[i] < 'A' || line[i] > 'Z') && (line[i] < 'a' || line[i] > 'z') {
				i++
			}
			if i < len(line) {
				i++
			}
			continue
		}
		_, size := utf8.DecodeRuneInString(line[i:])
		visible++
		i += size
		if visible >= maxWidth {
			return line[:i] + resetAll
		}
	}
	return line
}
