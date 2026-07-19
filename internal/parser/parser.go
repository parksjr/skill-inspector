package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Level represents the advisory severity of a finding.
// This is lightweight classification only — no risk score, no pass/fail, no blocking.
type Level string

const (
	LevelInfo Level = "info" // informational, likely benign
	LevelWarn Level = "warn" // worth reviewing
	LevelHigh Level = "high" // potentially suspicious, needs attention
)

// EnableDeps controls whether the dependency/supply-chain hint detector runs.
// Set to true from CLI flags (--check-deps). Off by default.
var EnableDeps bool

// shortHash returns the first 8 hex chars of the SHA-256 hash of s.
func shortHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:8]
}

// Finding is implemented by all finding types that have stable IDs and advisory levels.
type Finding interface {
	ID() string
	Level() Level
}

// Frontmatter holds the YAML frontmatter extracted from the top of a skill file.
type Frontmatter struct {
	Lines     []string // lines between the --- delimiters (exclusive)
	StartLine int      // 1-indexed line number of opening ---
	EndLine   int      // 1-indexed line number of closing ---
}

// Level returns the advisory severity for frontmatter. Info-level — most skills have it.
func (f *Frontmatter) Level() Level { return LevelInfo }

// ID returns a stable identifier for this frontmatter finding.
func (f *Frontmatter) ID() string { return "fm:" + shortHash(strings.Join(f.Lines, "\n")) }

// HTMLComment holds a single extracted HTML comment block.
type HTMLComment struct {
	Raw       string // full text of the comment including <!-- and -->
	StartLine int    // 1-indexed line where <!-- appears
	EndLine   int    // 1-indexed line where --> appears
}

// Level returns the advisory severity for HTML comments. Warn-level — may hide content.
func (h HTMLComment) Level() Level { return LevelWarn }

// ID returns a stable identifier for this HTML comment finding.
func (h HTMLComment) ID() string { return "hc:" + shortHash(h.Raw) }

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

// Level returns the advisory severity for this suspicious character.
// Bidi controls and homoglyphs are high; zero-width and invisible operators are warn;
// spaces and BOM are info.
func (s SuspiciousChar) Level() Level {
	if lvl, ok := suspiciousCharLevels[s.Rune]; ok {
		return lvl
	}
	return LevelWarn
}

// ID returns a stable identifier for this suspicious character finding.
func (s SuspiciousChar) ID() string {
	return fmt.Sprintf("sc:U+%04X:line:%d:col:%d", s.Rune, s.Line, s.Col)
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

// Level returns the advisory severity for this YAML risk.
// Directives are info; document separators are warn.
func (y YAMLRisk) Level() Level {
	switch y.Kind {
	case "directive":
		return LevelInfo
	default:
		return LevelWarn
	}
}

// ID returns a stable identifier for this YAML risk finding.
func (y YAMLRisk) ID() string { return fmt.Sprintf("yr:%s:line:%d", y.Kind, y.Line) }

// CDATASection records a <![CDATA[...]]> block.
type CDATASection struct {
	Raw       string
	StartLine int
	EndLine   int
}

// Level returns the advisory severity for CDATA sections. Warn-level — unusual in markdown.
func (c CDATASection) Level() Level { return LevelWarn }

// ID returns a stable identifier for this CDATA section finding.
func (c CDATASection) ID() string { return "cd:" + shortHash(c.Raw) }

// HiddenComment records a JS or CSS comment detected in content.
type HiddenComment struct {
	Raw       string
	StartLine int
	EndLine   int
	Kind      string // "js-line", "css-block"
}

// Level returns the advisory severity for hidden comments. Warn-level — may hide content.
func (h HiddenComment) Level() Level { return LevelWarn }

// ID returns a stable identifier for this hidden comment finding.
func (h HiddenComment) ID() string { return fmt.Sprintf("jc:%s:line:%d", h.Kind, h.StartLine) }

// DependencyHint records a detected package/dependency reference in the skill content.
// All findings are advisory-only — no vulnerability verdicts or blocking.
type DependencyHint struct {
	Line       int    // 1-indexed line number
	Reference  string // the matched dependency reference (e.g. "pip install foo")
	Package    string // extracted package name
	Suspicious bool   // true if the reference matches a heuristic warning pattern
}

// Level returns the advisory severity. Info for normal references, warn for suspicious.
func (d DependencyHint) Level() Level {
	if d.Suspicious {
		return LevelWarn
	}
	return LevelInfo
}

// ID returns a stable identifier for this dependency hint.
func (d DependencyHint) ID() string { return fmt.Sprintf("dep:line:%d", d.Line) }

// ParseResult holds all hidden/suspicious content extracted from a skill file.
type ParseResult struct {
	Frontmatter     *Frontmatter
	HTMLComments    []HTMLComment
	SuspiciousChars []SuspiciousChar
	YAMLRisks       []YAMLRisk
	CDATASections   []CDATASection
	HiddenComments  []HiddenComment
	DependencyHints []DependencyHint
}

// Findings returns all findings in this result as a slice of Finding interface values.
func (r *ParseResult) Findings() []Finding {
	var out []Finding
	if r.Frontmatter != nil {
		out = append(out, r.Frontmatter)
	}
	for i := range r.HTMLComments {
		out = append(out, &r.HTMLComments[i])
	}
	for i := range r.SuspiciousChars {
		out = append(out, &r.SuspiciousChars[i])
	}
	for i := range r.YAMLRisks {
		out = append(out, &r.YAMLRisks[i])
	}
	for i := range r.CDATASections {
		out = append(out, &r.CDATASections[i])
	}
	for i := range r.HiddenComments {
		out = append(out, &r.HiddenComments[i])
	}
	for i := range r.DependencyHints {
		out = append(out, &r.DependencyHints[i])
	}
	return out
}

// LevelCounts holds the count of findings at each advisory level.
type LevelCounts struct {
	Info int
	Warn int
	High int
}

// Total returns the total number of findings across all levels.
func (lc LevelCounts) Total() int { return lc.Info + lc.Warn + lc.High }

// LevelSummary returns the count of findings at each advisory level.
func (r *ParseResult) LevelSummary() LevelCounts {
	var lc LevelCounts
	if r.Frontmatter != nil {
		lc.Info++
	}
	for _, h := range r.HTMLComments {
		_ = h
		lc.Warn++
	}
	for _, s := range r.SuspiciousChars {
		switch s.Level() {
		case LevelInfo:
			lc.Info++
		case LevelHigh:
			lc.High++
		default:
			lc.Warn++
		}
	}
	for _, y := range r.YAMLRisks {
		switch y.Level() {
		case LevelInfo:
			lc.Info++
		default:
			lc.Warn++
		}
	}
	for range r.CDATASections {
		lc.Warn++
	}
	for range r.HiddenComments {
		lc.Warn++
	}
	for _, d := range r.DependencyHints {
		if d.Suspicious {
			lc.Warn++
		} else {
			lc.Info++
		}
	}
	return lc
}

// LevelLabel returns a human-readable label for the given level.
func LevelLabel(l Level) string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelHigh:
		return "HIGH"
	default:
		return "WARN"
	}
}

// Detector identifies a single category of hidden or suspicious content.
// Each detector is run in a fixed, documented order by the pipeline.
type Detector interface {
	// Name returns a short, human-readable identifier for this detector
	// (e.g. "frontmatter", "html-comments", "suspicious-chars").
	Name() string

	// Detect scans lines and populates the relevant field(s) on result.
	Detect(lines []string, result *ParseResult)
}

// Pipeline holds an ordered list of detectors and runs them in sequence.
// The order is deterministic and documented.
type Pipeline struct {
	detectors []Detector
}

// Run executes every detector in the pipeline against content and returns
// the aggregated ParseResult.
func (p *Pipeline) Run(content string) *ParseResult {
	lines := strings.Split(content, "\n")
	result := &ParseResult{}
	for _, d := range p.detectors {
		d.Detect(lines, result)
	}
	return result
}

// DefaultPipeline returns a pipeline with all standard detectors in the
// documented execution order:
//
//  1. frontmatter     — YAML frontmatter block (---)
//  2. html-comments   — HTML <!-- --> comment blocks
//  3. suspicious-chars — invisible/confusable Unicode code points
//  4. yaml-risks      — YAML directives, document separators
//  5. cdata-sections  — <![CDATA[ ... ]]> blocks
//  6. hidden-comments — JS // and CSS /* */ comments
func DefaultPipeline() *Pipeline {
	dets := []Detector{
		frontmatterDetector{},
		htmlCommentDetector{},
		suspiciousCharDetector{},
		yamlRiskDetector{},
		cdataSectionDetector{},
		hiddenCommentDetector{},
	}
	if EnableDeps {
		dets = append(dets, dependencyHintDetector{})
	}
	return &Pipeline{detectors: dets}
}

// --- detector implementations ---

type frontmatterDetector struct{}

func (frontmatterDetector) Name() string { return "frontmatter" }
func (frontmatterDetector) Detect(lines []string, result *ParseResult) {
	result.Frontmatter = extractFrontmatter(lines)
}

type htmlCommentDetector struct{}

func (htmlCommentDetector) Name() string { return "html-comments" }
func (htmlCommentDetector) Detect(lines []string, result *ParseResult) {
	result.HTMLComments = extractHTMLComments(lines)
}

type suspiciousCharDetector struct{}

func (suspiciousCharDetector) Name() string { return "suspicious-chars" }
func (suspiciousCharDetector) Detect(lines []string, result *ParseResult) {
	result.SuspiciousChars = extractSuspiciousChars(lines)
}

type yamlRiskDetector struct{}

func (yamlRiskDetector) Name() string { return "yaml-risks" }
func (yamlRiskDetector) Detect(lines []string, result *ParseResult) {
	result.YAMLRisks = extractYAMLRisks(lines)
}

type cdataSectionDetector struct{}

func (cdataSectionDetector) Name() string { return "cdata-sections" }
func (cdataSectionDetector) Detect(lines []string, result *ParseResult) {
	result.CDATASections = extractCDATASections(lines)
}

type hiddenCommentDetector struct{}

func (hiddenCommentDetector) Name() string { return "hidden-comments" }
func (hiddenCommentDetector) Detect(lines []string, result *ParseResult) {
	result.HiddenComments = extractHiddenComments(lines)
}

type dependencyHintDetector struct{}

func (dependencyHintDetector) Name() string { return "dependency-hints" }
func (dependencyHintDetector) Detect(lines []string, result *ParseResult) {
	result.DependencyHints = extractDependencyHints(lines)
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

// suspiciousCharLevels maps specific runes to their advisory severity level.
// Runes not listed here default to LevelWarn.
var suspiciousCharLevels = map[rune]Level{
	// --- INFO: likely benign, informational ---
	'\uFEFF': LevelInfo, // BOM
	'\u00AD': LevelInfo, // soft hyphen
	'\u00A0': LevelInfo, // no-break space
	'\u2000': LevelInfo, '\u2001': LevelInfo, '\u2002': LevelInfo, '\u2003': LevelInfo,
	'\u2004': LevelInfo, '\u2005': LevelInfo, '\u2006': LevelInfo, '\u2007': LevelInfo,
	'\u2008': LevelInfo, '\u2009': LevelInfo, '\u200A': LevelInfo,
	'\u202F': LevelInfo, '\u205F': LevelInfo, '\u3000': LevelInfo,
	// --- HIGH: actively deceptive or dangerous ---
	// Bidi controls (Trojan Source, CVE-2021-42574)
	'\u202A': LevelHigh, '\u202B': LevelHigh, '\u202C': LevelHigh,
	'\u202D': LevelHigh, '\u202E': LevelHigh,
	'\u2066': LevelHigh, '\u2067': LevelHigh, '\u2068': LevelHigh, '\u2069': LevelHigh,
	// Homoglyphs
	'\u0430': LevelHigh, '\u0435': LevelHigh, '\u043E': LevelHigh,
	'\u0440': LevelHigh, '\u0441': LevelHigh, '\u0443': LevelHigh, '\u0445': LevelHigh,
	'\u0410': LevelHigh, '\u0412': LevelHigh, '\u0415': LevelHigh,
	'\u041A': LevelHigh, '\u041C': LevelHigh, '\u041D': LevelHigh,
	'\u041E': LevelHigh, '\u0420': LevelHigh, '\u0421': LevelHigh,
	'\u0422': LevelHigh, '\u0425': LevelHigh,
	'\u0391': LevelHigh, '\u0392': LevelHigh, '\u0395': LevelHigh, '\u0396': LevelHigh,
	'\u0397': LevelHigh, '\u0399': LevelHigh, '\u039A': LevelHigh, '\u039C': LevelHigh,
	'\u039D': LevelHigh, '\u039F': LevelHigh, '\u03A1': LevelHigh,
	'\u03A4': LevelHigh, '\u03A5': LevelHigh, '\u03A7': LevelHigh,
	// Language tag
	'\U000E0001': LevelHigh,
	// --- WARN: zero-width, invisible operators, variation selectors, bidi formatting ---
	'\u200B': LevelWarn, '\u200C': LevelWarn, '\u200D': LevelWarn,
	'\u2060': LevelWarn, '\u2061': LevelWarn, '\u2062': LevelWarn,
	'\u2063': LevelWarn, '\u2064': LevelWarn,
	'\u206A': LevelWarn, '\u206B': LevelWarn, '\u206C': LevelWarn,
	'\u206D': LevelWarn, '\u206E': LevelWarn, '\u206F': LevelWarn,
	'\uFE00': LevelWarn, '\uFE01': LevelWarn, '\uFE02': LevelWarn,
	'\uFE03': LevelWarn, '\uFE04': LevelWarn, '\uFE05': LevelWarn,
	'\uFE06': LevelWarn, '\uFE07': LevelWarn, '\uFE08': LevelWarn,
	'\uFE09': LevelWarn, '\uFE0A': LevelWarn, '\uFE0B': LevelWarn,
	'\uFE0C': LevelWarn, '\uFE0D': LevelWarn, '\uFE0E': LevelWarn, '\uFE0F': LevelWarn,
}

// Parse extracts all hidden and suspicious content from the raw text of a skill file.
// It delegates to the default detector pipeline, which runs detectors in a fixed,
// documented order.
func Parse(content string) *ParseResult {
	return DefaultPipeline().Run(content)
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
		} else if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
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
// including multi-line comments. Content inside fenced code blocks (``` or ~~~)
// and inline code (`...`) is excluded to avoid false positives.
func extractHTMLComments(lines []string) []HTMLComment {
	joined := strings.Join(lines, "\n")
	// Search in a masked copy where code blocks are replaced with spaces.
	// Line positions are preserved, and raw content is still extracted
	// from the original joined string.
	masked := maskCodeBlocks(joined)
	var comments []HTMLComment

	searchFrom := 0
	for {
		start := strings.Index(masked[searchFrom:], "<!--")
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

// maskCodeBlocks returns a copy of s where content inside fenced code blocks
// (``` or ~~~) and inline code (`...`) is replaced with spaces. Newlines are
// preserved. The result is used to suppress false-positive comment detection
// without affecting line-number calculations.
func maskCodeBlocks(s string) string {
	b := []byte(s)
	inFence := byte(0)
	fenceCount := 0

	for i := 0; i < len(b); i++ {
		lineStart := i == 0 || b[i-1] == '\n'

		// Check for closing fence when inside a code block.
		if lineStart && inFence != 0 {
			j := i
			for j < len(b) && b[j] == inFence {
				j++
			}
			count := j - i
			if count >= fenceCount {
				// A closing fence must be followed only by whitespace.
				restOK := true
				for k := j; k < len(b) && b[k] != '\n'; k++ {
					if b[k] != ' ' && b[k] != '\t' && b[k] != '\r' {
						restOK = false
						break
					}
				}
				if restOK {
					inFence = 0
					fenceCount = 0
					continue
				}
			}
		}

		if inFence != 0 {
			// Inside a fenced code block: mask non-newline characters.
			if b[i] != '\n' {
				b[i] = ' '
			}
			continue
		}

		// Check for opening fenced code block at line start.
		if lineStart && i < len(b) && (b[i] == '`' || b[i] == '~') {
			ch := b[i]
			j := i
			for j < len(b) && b[j] == ch {
				j++
			}
			if count := j - i; count >= 3 {
				inFence = ch
				fenceCount = count
				i = j - 1 // will advance past on next iteration
				continue
			}
		}

		// Check for inline code: `content`
		if b[i] == '`' {
			j := i + 1
			for j < len(b) && b[j] != '`' && b[j] != '\n' {
				j++
			}
			if j < len(b) && b[j] == '`' && j > i+1 {
				for k := i + 1; k < j; k++ {
					if b[k] != '\n' {
						b[k] = ' '
					}
				}
				i = j // will advance past closing backtick
				continue
			}
		}
	}
	return string(b)
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

// installPatterns matches common package manager install commands.
// Group 1 captures the package name.
var installPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b(?:pip|pip3)\s+install\s+(?:-+\S+\s+)*['"]?(\S+?)['"]?(?:\s|$)`),
	regexp.MustCompile(`\bnpm\s+(?:install|i)\s+(?:-+\S+\s+)*['"]?(\S+?)['"]?(?:\s|$)`),
	regexp.MustCompile(`\bgem\s+install\s+['"]?(\S+?)['"]?(?:\s|$)`),
	regexp.MustCompile(`\bgo\s+get\s+['"]?(\S+?)['"]?(?:\s|$)`),
	regexp.MustCompile(`\bcargo\s+install\s+['"]?(\S+?)['"]?(?:\s|$)`),
	regexp.MustCompile(`\bbrew\s+install\s+['"]?(\S+?)['"]?(?:\s|$)`),
	regexp.MustCompile(`\b(?:apt-get|apt|yum|dnf)\s+install\s+(?:-+\S+\s+)*['"]?(\S+?)['"]?(?:\s|$)`),
}

// suspiciousPkgRe matches patterns that may indicate suspicious package names.
var suspiciousPkgRe = regexp.MustCompile(`(?i)(?:.{30,}|^.{1,2}$|[^\w\-.\/@:])`)

// isSuspiciousPkg checks a package name for heuristic warning signs.
func isSuspiciousPkg(pkg string) bool {
	if suspiciousPkgRe.MatchString(pkg) {
		return true
	}
	// Check for 4+ consecutive repeated characters (typosquat hint).
	if len(pkg) >= 4 {
		for i := 0; i <= len(pkg)-4; i++ {
			if pkg[i] == pkg[i+1] && pkg[i] == pkg[i+2] && pkg[i] == pkg[i+3] {
				return true
			}
		}
	}
	return false
}

// extractDependencyHints scans lines for package manager install commands
// and returns advisory DependencyHint findings.
func extractDependencyHints(lines []string) []DependencyHint {
	var hints []DependencyHint
	for i, line := range lines {
		for _, pat := range installPatterns {
			matches := pat.FindStringSubmatch(line)
			if len(matches) < 2 {
				continue
			}
			pkg := matches[1]
			suspicious := isSuspiciousPkg(pkg)
			hints = append(hints, DependencyHint{
				Line:       i + 1,
				Reference:  strings.TrimSpace(matches[0]),
				Package:    pkg,
				Suspicious: suspicious,
			})
			break // one hint per line
		}
	}
	return hints
}
