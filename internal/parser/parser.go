package parser

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Frontmatter holds the YAML frontmatter extracted from the top of a skill file.
type Frontmatter struct {
	Lines     []string // lines between the --- delimiters (exclusive)
	StartLine int      // 1-indexed line number of opening ---
	EndLine   int      // 1-indexed line number of closing ---
}

// HTMLComment holds a single extracted HTML comment block.
type HTMLComment struct {
	Raw       string // full text of the comment including <!-- and -->
	StartLine int    // 1-indexed line where <!-- appears
	EndLine   int    // 1-indexed line where --> appears
}

// SuspiciousChar records a single suspicious Unicode character found in the file.
type SuspiciousChar struct {
	Rune rune   // the Unicode code point
	Name string // human-readable name
	Line int    // 1-indexed line number
	Col  int    // byte offset within the line (0-indexed)
}

// Format returns the standard display string for this finding.
func (s SuspiciousChar) Format() string {
	return fmt.Sprintf("[U+%04X %s — line %d, col %d]", s.Rune, s.Name, s.Line, s.Col)
}

// ParseResult holds all hidden/suspicious content extracted from a skill file.
type ParseResult struct {
	Frontmatter     *Frontmatter
	HTMLComments    []HTMLComment
	SuspiciousChars []SuspiciousChar
}

// suspiciousRunes maps known invisible/unusual Unicode code points to their names.
var suspiciousRunes = map[rune]string{
	'\u200B': "ZERO WIDTH SPACE",
	'\u200C': "ZERO WIDTH NON-JOINER",
	'\u200D': "ZERO WIDTH JOINER",
	'\uFEFF': "ZERO WIDTH NO-BREAK SPACE (BOM)",
	'\u00AD': "SOFT HYPHEN",
	'\u00A0': "NO-BREAK SPACE",
	'\u2000': "EN QUAD",
	'\u2001': "EM QUAD",
	'\u2002': "EN SPACE",
	'\u2003': "EM SPACE",
	'\u2004': "THREE-PER-EM SPACE",
	'\u2005': "FOUR-PER-EM SPACE",
	'\u2006': "SIX-PER-EM SPACE",
	'\u2007': "FIGURE SPACE",
	'\u2008': "PUNCTUATION SPACE",
	'\u2009': "THIN SPACE",
	'\u200A': "HAIR SPACE",
	'\u202F': "NARROW NO-BREAK SPACE",
	'\u205F': "MEDIUM MATHEMATICAL SPACE",
	'\u3000': "IDEOGRAPHIC SPACE",
	'\u2060': "WORD JOINER",
	'\u2061': "FUNCTION APPLICATION",
	'\u2062': "INVISIBLE TIMES",
	'\u2063': "INVISIBLE SEPARATOR",
	'\u2064': "INVISIBLE PLUS",
	'\u206A': "INHIBIT SYMMETRIC SWAPPING",
	'\u206B': "ACTIVATE SYMMETRIC SWAPPING",
	'\u206C': "INHIBIT ARABIC FORM SHAPING",
	'\u206D': "ACTIVATE ARABIC FORM SHAPING",
	'\u206E': "NATIONAL DIGIT SHAPES",
	'\u206F': "NOMINAL DIGIT SHAPES",
}

// Parse extracts all hidden and suspicious content from the raw text of a skill file.
func Parse(content string) *ParseResult {
	lines := strings.Split(content, "\n")
	return &ParseResult{
		Frontmatter:     extractFrontmatter(lines),
		HTMLComments:    extractHTMLComments(lines),
		SuspiciousChars: extractSuspiciousChars(lines),
	}
}

// extractFrontmatter looks for a YAML frontmatter block delimited by --- at the
// very start of the file. Returns nil if no valid frontmatter block is found.
func extractFrontmatter(lines []string) *Frontmatter {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	// Find closing ---
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return &Frontmatter{
				Lines:     lines[1:i],
				StartLine: 1,
				EndLine:   i + 1, // 1-indexed
			}
		}
	}
	return nil // opening --- found but no closing ---
}

// extractHTMLComments finds all <!-- ... --> comment blocks in the file,
// including multi-line comments.
func extractHTMLComments(lines []string) []HTMLComment {
	// Join with newlines to handle multi-line comments, then scan.
	joined := strings.Join(lines, "\n")
	var comments []HTMLComment

	searchFrom := 0
	for {
		start := strings.Index(joined[searchFrom:], "<!--")
		if start == -1 {
			break
		}
		start += searchFrom

		end := strings.Index(joined[start:], "-->")
		if end == -1 {
			// Unclosed comment — capture to end of file.
			raw := joined[start:]
			startLine := countLines(joined[:start]) + 1
			endLine := strings.Count(joined, "\n") + 1
			comments = append(comments, HTMLComment{
				Raw:       raw,
				StartLine: startLine,
				EndLine:   endLine,
			})
			break
		}
		end = start + end + 3 // include "-->"

		raw := joined[start:end]
		startLine := countLines(joined[:start]) + 1
		endLine := countLines(joined[:end]) + 1
		comments = append(comments, HTMLComment{
			Raw:       raw,
			StartLine: startLine,
			EndLine:   endLine,
		})
		searchFrom = end
	}
	return comments
}

// countLines counts the number of newline characters before position pos.
func countLines(s string) int {
	return strings.Count(s, "\n")
}

// extractSuspiciousChars scans every rune in every line for known invisible/
// unusual Unicode characters and returns one SuspiciousChar per finding.
func extractSuspiciousChars(lines []string) []SuspiciousChar {
	var findings []SuspiciousChar
	for lineIdx, line := range lines {
		byteOffset := 0
		for byteOffset < len(line) {
			r, size := utf8.DecodeRuneInString(line[byteOffset:])
			if r == utf8.RuneError && size == 1 {
				// Invalid UTF-8 byte — flag it.
				findings = append(findings, SuspiciousChar{
					Rune: r,
					Name: "INVALID UTF-8 BYTE",
					Line: lineIdx + 1,
					Col:  byteOffset,
				})
				byteOffset++
				continue
			}
			if name, ok := suspiciousRunes[r]; ok {
				findings = append(findings, SuspiciousChar{
					Rune: r,
					Name: name,
					Line: lineIdx + 1,
					Col:  byteOffset,
				})
			}
			byteOffset += size
		}
	}
	return findings
}
