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

// YAMLRisk records a YAML directive (%YAML) or multi-document separator (... or extra ---).
type YAMLRisk struct {
	Line    int    // 1-indexed
	Content string // the trimmed line content
	Kind    string // "directive", "document-end", or "document-start"
}

// Format returns the standard display string for this finding.
func (y YAMLRisk) Format() string {
	switch y.Kind {
	case "directive":
		return fmt.Sprintf("[line %d — YAML %s]", y.Line, y.Content)
	case "document-end":
		return fmt.Sprintf("[line %d — YAML document separator: %s]", y.Line, y.Content)
	case "document-start":
		return fmt.Sprintf("[line %d — YAML document separator: %s (multi-doc start)]", y.Line, y.Content)
	}
	return fmt.Sprintf("[line %d — YAML risk: %s]", y.Line, y.Content)
}

// CDATASection records a <![CDATA[...]]> block.
type CDATASection struct {
	Raw       string
	StartLine int
	EndLine   int
}

// HiddenComment records a JS or CSS comment detected in content.
type HiddenComment struct {
	Raw       string
	StartLine int
	EndLine   int
	Kind      string // "js-line", "css-block"
}

// ParseResult holds all hidden/suspicious content extracted from a skill file.
type ParseResult struct {
	Frontmatter     *Frontmatter
	HTMLComments    []HTMLComment
	SuspiciousChars []SuspiciousChar
	YAMLRisks       []YAMLRisk
	CDATASections   []CDATASection
	HiddenComments  []HiddenComment
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
	// Bidi control characters (Trojan Source attack, CVE-2021-42574)
	'\u202A': "LEFT-TO-RIGHT EMBEDDING",
	'\u202B': "RIGHT-TO-LEFT EMBEDDING",
	'\u202C': "POP DIRECTIONAL FORMATTING",
	'\u202D': "LEFT-TO-RIGHT OVERRIDE",
	'\u202E': "RIGHT-TO-LEFT OVERRIDE",
	'\u2066': "LEFT-TO-RIGHT ISOLATE",
	'\u2067': "RIGHT-TO-LEFT ISOLATE",
	'\u2068': "FIRST STRONG ISOLATE",
	'\u2069': "POP DIRECTIONAL ISOLATE",
	// Tag characters (Unicode tag block, can hide content)
	'\U000E0001': "LANGUAGE TAG",
	// Variation selectors
	'\uFE00': "VARIATION SELECTOR-1",
	'\uFE01': "VARIATION SELECTOR-2",
	'\uFE02': "VARIATION SELECTOR-3",
	'\uFE03': "VARIATION SELECTOR-4",
	'\uFE04': "VARIATION SELECTOR-5",
	'\uFE05': "VARIATION SELECTOR-6",
	'\uFE06': "VARIATION SELECTOR-7",
	'\uFE07': "VARIATION SELECTOR-8",
	'\uFE08': "VARIATION SELECTOR-9",
	'\uFE09': "VARIATION SELECTOR-10",
	'\uFE0A': "VARIATION SELECTOR-11",
	'\uFE0B': "VARIATION SELECTOR-12",
	'\uFE0C': "VARIATION SELECTOR-13",
	'\uFE0D': "VARIATION SELECTOR-14",
	'\uFE0E': "VARIATION SELECTOR-15",
	'\uFE0F': "VARIATION SELECTOR-16",
	// Homoglyphs — characters visually similar to ASCII (confusable detection).
	// Cyrillic lowercase homoglyphs.
	'\u0430': "CYRILLIC SMALL A (homoglyph: a)",
	'\u0435': "CYRILLIC SMALL IE (homoglyph: e)",
	'\u043E': "CYRILLIC SMALL O (homoglyph: o)",
	'\u0440': "CYRILLIC SMALL ER (homoglyph: p)",
	'\u0441': "CYRILLIC SMALL ES (homoglyph: c)",
	'\u0443': "CYRILLIC SMALL U (homoglyph: y)",
	'\u0445': "CYRILLIC SMALL HA (homoglyph: x)",
	// Cyrillic uppercase homoglyphs.
	'\u0410': "CYRILLIC CAPITAL A (homoglyph: A)",
	'\u0412': "CYRILLIC CAPITAL VE (homoglyph: B)",
	'\u0415': "CYRILLIC CAPITAL IE (homoglyph: E)",
	'\u041A': "CYRILLIC CAPITAL KA (homoglyph: K)",
	'\u041C': "CYRILLIC CAPITAL EM (homoglyph: M)",
	'\u041D': "CYRILLIC CAPITAL EN (homoglyph: H)",
	'\u041E': "CYRILLIC CAPITAL O (homoglyph: O)",
	'\u0420': "CYRILLIC CAPITAL ER (homoglyph: P)",
	'\u0421': "CYRILLIC CAPITAL ES (homoglyph: C)",
	'\u0422': "CYRILLIC CAPITAL TE (homoglyph: T)",
	'\u0425': "CYRILLIC CAPITAL HA (homoglyph: X)",
	// Greek uppercase homoglyphs.
	'\u0391': "GREEK CAPITAL ALPHA (homoglyph: A)",
	'\u0392': "GREEK CAPITAL BETA (homoglyph: B)",
	'\u0395': "GREEK CAPITAL EPSILON (homoglyph: E)",
	'\u0396': "GREEK CAPITAL ZETA (homoglyph: Z)",
	'\u0397': "GREEK CAPITAL ETA (homoglyph: H)",
	'\u0399': "GREEK CAPITAL IOTA (homoglyph: I)",
	'\u039A': "GREEK CAPITAL KAPPA (homoglyph: K)",
	'\u039C': "GREEK CAPITAL MU (homoglyph: M)",
	'\u039D': "GREEK CAPITAL NU (homoglyph: N)",
	'\u039F': "GREEK CAPITAL OMICRON (homoglyph: O)",
	'\u03A1': "GREEK CAPITAL RHO (homoglyph: P)",
	'\u03A4': "GREEK CAPITAL TAU (homoglyph: T)",
	'\u03A5': "GREEK CAPITAL UPSILON (homoglyph: Y)",
	'\u03A7': "GREEK CAPITAL CHI (homoglyph: X)",
}

// Parse extracts all hidden and suspicious content from the raw text of a skill file.
func Parse(content string) *ParseResult {
	lines := strings.Split(content, "\n")
	return &ParseResult{
		Frontmatter:     extractFrontmatter(lines),
		HTMLComments:    extractHTMLComments(lines),
		SuspiciousChars: extractSuspiciousChars(lines),
		YAMLRisks:       extractYAMLRisks(lines),
		CDATASections:   extractCDATASections(lines),
		HiddenComments:  extractHiddenComments(lines),
	}
}

// FrontmatterValue returns the string value for key from frontmatter lines.
// It supports simple YAML key/value lines like "name: value".
func FrontmatterValue(fm *Frontmatter, key string) (string, bool) {
	if fm == nil {
		return "", false
	}
	for _, line := range fm.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		k, v, found := strings.Cut(trimmed, ":")
		if !found || !strings.EqualFold(strings.TrimSpace(k), key) {
			continue
		}
		value := strings.TrimSpace(v)
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") && len(value) >= 2 {
			value = value[1 : len(value)-1]
		}
		if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
			value = value[1 : len(value)-1]
		}
		if value == "" {
			return "", false
		}
		return value, true
	}
	return "", false
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

// extractYAMLRisks detects YAML directives (%YAML at line start) and
// multi-document separators (... or additional --- beyond the frontmatter block).
func extractYAMLRisks(lines []string) []YAMLRisk {
	var risks []YAMLRisk
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// YAML directive like %YAML 1.2
		if strings.HasPrefix(trimmed, "%YAML") {
			risks = append(risks, YAMLRisk{
				Line:    i + 1,
				Content: trimmed,
				Kind:    "directive",
			})
		}
		// YAML document end marker: ... on a line by itself
		if trimmed == "..." {
			risks = append(risks, YAMLRisk{
				Line:    i + 1,
				Content: trimmed,
				Kind:    "document-end",
			})
		}
		// Additional --- separators (not the first line, which is frontmatter opener)
		if i > 0 && trimmed == "---" {
			risks = append(risks, YAMLRisk{
				Line:    i + 1,
				Content: trimmed,
				Kind:    "document-start",
			})
		}
	}
	return risks
}

// extractCDATASections finds <![CDATA[...]]> sections in the file content.
func extractCDATASections(lines []string) []CDATASection {
	joined := strings.Join(lines, "\n")
	var sections []CDATASection

	searchFrom := 0
	for {
		start := strings.Index(joined[searchFrom:], "<![CDATA[")
		if start == -1 {
			break
		}
		start += searchFrom

		end := strings.Index(joined[start:], "]]>")
		if end == -1 {
			// Unclosed CDATA — capture to end of file.
			raw := joined[start:]
			startLine := countLines(joined[:start]) + 1
			endLine := strings.Count(joined, "\n") + 1
			sections = append(sections, CDATASection{
				Raw:       raw,
				StartLine: startLine,
				EndLine:   endLine,
			})
			break
		}
		end = start + end + 3 // include "]]>"

		raw := joined[start:end]
		startLine := countLines(joined[:start]) + 1
		endLine := countLines(joined[:end]) + 1
		sections = append(sections, CDATASection{
			Raw:       raw,
			StartLine: startLine,
			EndLine:   endLine,
		})
		searchFrom = end
	}
	return sections
}

// extractHiddenComments detects JavaScript // line comments and CSS /* */ block
// comments that could be used to hide content.
func extractHiddenComments(lines []string) []HiddenComment {
	joined := strings.Join(lines, "\n")
	var comments []HiddenComment

	// CSS /* */ block comments.
	searchFrom := 0
	for {
		start := strings.Index(joined[searchFrom:], "/*")
		if start == -1 {
			break
		}
		start += searchFrom

		end := strings.Index(joined[start:], "*/")
		if end == -1 {
			raw := joined[start:]
			startLine := countLines(joined[:start]) + 1
			endLine := strings.Count(joined, "\n") + 1
			comments = append(comments, HiddenComment{
				Raw:       raw,
				StartLine: startLine,
				EndLine:   endLine,
				Kind:      "css-block",
			})
			break
		}
		end = start + end + 2 // include "*/"

		raw := joined[start:end]
		startLine := countLines(joined[:start]) + 1
		endLine := countLines(joined[:end]) + 1
		comments = append(comments, HiddenComment{
			Raw:       raw,
			StartLine: startLine,
			EndLine:   endLine,
			Kind:      "css-block",
		})
		searchFrom = end
	}

	// JS // line comments: scan line by line.
	for i, line := range lines {
		// Find // outside of URL protocols to reduce false positives.
		idx := strings.Index(line, "//")
		if idx >= 0 {
			// Skip // that are part of URL protocols (http://, https://, ws://, etc.)
			if idx >= 5 && line[idx-1] == ':' {
				continue
			}
			comments = append(comments, HiddenComment{
				Raw:       line,
				StartLine: i + 1,
				EndLine:   i + 1,
				Kind:      "js-line",
			})
		}
	}

	return comments
}
