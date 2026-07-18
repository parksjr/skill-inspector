package loader

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/parksjr/skill-inspector/internal/parser"
)

// maxFetchBytes is the maximum size (10 MiB) allowed when fetching from a URL.
const maxFetchBytes = 10 << 20

// httpClient is the HTTP client used for URL fetching. Tests may replace it.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// SkillFile holds the loaded contents and metadata for a skill file.
type SkillFile struct {
	Content    string // raw file text
	SourcePath string // original CLI argument (file path or URL)
	SkillName  string // derived name used for install directory
	IsURL      bool   // true if input was an HTTP/HTTPS URL
}

// Load reads a skill file from a local path or a direct HTTP/HTTPS URL.
// Returns a populated SkillFile or an error.
func Load(input string) (*SkillFile, error) {
	if isURL(input) {
		return loadFromURL(input)
	}
	return loadFromFile(input)
}

// isURL returns true if s begins with https://.
func isURL(s string) bool {
	return strings.HasPrefix(s, "https://")
}

// loadFromFile loads a SkillFile from a local path.
// If path is a directory, it looks for SKILL.md inside it.
func loadFromFile(inputPath string) (*SkillFile, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access %q: %w", inputPath, err)
	}

	var filePath string
	var skillName string

	if info.IsDir() {
		filePath = filepath.Join(inputPath, "SKILL.md")
		skillName = filepath.Base(inputPath)
		if _, err := os.Stat(filePath); err != nil {
			return nil, fmt.Errorf("directory %q has no SKILL.md: %w", inputPath, err)
		}
	} else {
		filePath = inputPath
		base := filepath.Base(filePath)
		skillName = strings.TrimSuffix(base, filepath.Ext(base))
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %q: %w", filePath, err)
	}
	content := string(data)

	return &SkillFile{
		Content:    content,
		SourcePath: inputPath,
		SkillName:  deriveSkillName(content, skillName),
		IsURL:      false,
	}, nil
}

// loadFromURL fetches a SkillFile from a direct HTTP/HTTPS URL.
// The content is held in memory and never written to disk.
func loadFromURL(rawURL string) (*SkillFile, error) {
	fetchURL := normalizeGitHubBlobURL(rawURL)

	resp, err := httpClient.Get(fetchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %q: %w", fetchURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d fetching %q", resp.StatusCode, fetchURL)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %q: %w", fetchURL, err)
	}

	// Fallback skill name from the last path segment of the URL, strip .md extension.
	base := path.Base(rawURL)
	fallbackName := strings.TrimSuffix(base, path.Ext(base))
	content := string(data)

	return &SkillFile{
		Content:    content,
		SourcePath: rawURL,
		SkillName:  deriveSkillName(content, fallbackName),
		IsURL:      true,
	}, nil
}

func deriveSkillName(content, fallback string) string {
	parsed := parser.Parse(content)
	if frontmatterName, ok := parser.FrontmatterValue(parsed.Frontmatter, "name"); ok {
		if sanitized := sanitizeSkillName(frontmatterName); sanitized != "" {
			return sanitized
		}
	}
	return fallback
}

func sanitizeSkillName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}

	var b strings.Builder
	lastHyphen := false
	for _, r := range trimmed {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_', r == '.':
			b.WriteRune(r)
			lastHyphen = false
		case unicode.IsSpace(r), r == '/', r == '\\':
			if !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		default:
			if !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}

	sanitized := strings.Trim(b.String(), "-.")
	if sanitized == "" || sanitized == "." || sanitized == ".." {
		return ""
	}
	return sanitized
}

func normalizeGitHubBlobURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	host := strings.ToLower(parsed.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return rawURL
	}

	segments := strings.Split(strings.TrimPrefix(parsed.EscapedPath(), "/"), "/")
	if len(segments) < 5 || segments[2] != "blob" {
		return rawURL
	}

	owner, repo := segments[0], segments[1]
	ref := segments[3]
	filePath := strings.Join(segments[4:], "/")
	if owner == "" || repo == "" || ref == "" || filePath == "" {
		return rawURL
	}

	return "https://raw.githubusercontent.com/" + owner + "/" + repo + "/" + ref + "/" + filePath
}
