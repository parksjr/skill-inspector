package loader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

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

// isURL returns true if s begins with http:// or https://.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
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

	return &SkillFile{
		Content:    string(data),
		SourcePath: inputPath,
		SkillName:  skillName,
		IsURL:      false,
	}, nil
}

// loadFromURL fetches a SkillFile from a direct HTTP/HTTPS URL.
// The content is held in memory and never written to disk.
func loadFromURL(rawURL string) (*SkillFile, error) {
	resp, err := http.Get(rawURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %q: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d fetching %q", resp.StatusCode, rawURL)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %q: %w", rawURL, err)
	}

	// Derive skill name from the last path segment of the URL, strip .md extension.
	base := path.Base(rawURL)
	skillName := strings.TrimSuffix(base, path.Ext(base))

	return &SkillFile{
		Content:    string(data),
		SourcePath: rawURL,
		SkillName:  skillName,
		IsURL:      true,
	}, nil
}
