package installer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// AgentDir describes a known agent skill directory.
type AgentDir struct {
	Name string // human-readable agent name, e.g. "claude"
	Path string // absolute expanded path, e.g. /home/user/.claude/skills
}

// LinkResult records the outcome of symlinking to one agent directory.
type LinkResult struct {
	Agent   string
	Path    string
	Linked  bool  // true if symlink was created
	Skipped bool  // true if agent dir does not exist
	Err     error // non-nil if something went wrong
}

// InstallResult is the summary returned by Install.
type InstallResult struct {
	SkillName   string
	InstallPath string       // canonical install dir: ~/.agents/skills/<name>
	Links       []LinkResult // one entry per configured agent dir
}

// defaultAgentDirs returns the hardcoded list of known agent skill directories.
// Paths use ~ which is expanded by expandHome.
func defaultAgentDirs() []AgentDir {
	return []AgentDir{
		{Name: "claude", Path: "~/.claude/skills"},
		{Name: "goose", Path: "~/.config/goose/skills"},
		{Name: "pi", Path: "~/.pi/skills"},
	}
}

// Install copies the skill into ~/.agents/skills/<skillName>/ and creates
// symlinks in each detected agent directory. sourcePath is the original
// file or directory path that was inspected (may be empty for URL sources,
// in which case content is used). For URL-sourced skills, content holds the
// raw SKILL.md text.
func Install(skillName, sourcePath, content string, isURL bool) (*InstallResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	installBase := filepath.Join(home, ".agents", "skills")
	installDir := filepath.Join(installBase, skillName)

	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create install directory %q: %w", installDir, err)
	}

	if isURL {
		dest := filepath.Join(installDir, "SKILL.md")
		if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("cannot write SKILL.md: %w", err)
		}
	} else {
		info, err := os.Stat(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("cannot stat source %q: %w", sourcePath, err)
		}
		if info.IsDir() {
			if err := copyDir(sourcePath, installDir); err != nil {
				return nil, fmt.Errorf("cannot copy directory: %w", err)
			}
		} else {
			dest := filepath.Join(installDir, "SKILL.md")
			if err := copyFile(sourcePath, dest); err != nil {
				return nil, fmt.Errorf("cannot copy file: %w", err)
			}
		}
	}

	agentDirs, err := loadAgentDirs(home)
	if err != nil {
		agentDirs = defaultAgentDirs()
	}

	for i := range agentDirs {
		agentDirs[i].Path = expandHome(agentDirs[i].Path, home)
	}

	result := &InstallResult{
		SkillName:   skillName,
		InstallPath: installDir,
	}
	for _, ad := range agentDirs {
		lr := linkSkill(skillName, installDir, ad)
		result.Links = append(result.Links, lr)
	}

	return result, nil
}

// linkSkill creates a symlink at <agentDir.Path>/<skillName> → installDir.
// Returns a LinkResult describing the outcome.
func linkSkill(skillName, installDir string, ad AgentDir) LinkResult {
	lr := LinkResult{Agent: ad.Name, Path: ad.Path}

	if _, err := os.Stat(ad.Path); os.IsNotExist(err) {
		lr.Skipped = true
		return lr
	}

	linkPath := filepath.Join(ad.Path, skillName)

	if existing, err := os.Lstat(linkPath); err == nil {
		if existing.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(linkPath); err != nil {
				lr.Err = fmt.Errorf("cannot remove existing symlink: %w", err)
				return lr
			}
		} else {
			lr.Err = fmt.Errorf("%q exists and is not a symlink — skipping", linkPath)
			return lr
		}
	}

	if err := os.Symlink(installDir, linkPath); err != nil {
		lr.Err = fmt.Errorf("symlink failed: %w", err)
		return lr
	}

	lr.Linked = true
	lr.Path = linkPath
	return lr
}

// loadAgentDirs reads ~/.config/skill-inspector/config.
// If the file does not exist, returns defaultAgentDirs().
// If it exists but is empty (or only comments), returns defaultAgentDirs().
// If it has entries, those entries REPLACE the defaults entirely.
func loadAgentDirs(home string) ([]AgentDir, error) {
	configPath := filepath.Join(home, ".config", "skill-inspector", "config")
	f, err := os.Open(configPath)
	if os.IsNotExist(err) {
		return defaultAgentDirs(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot open config %q: %w", configPath, err)
	}
	defer f.Close()

	var dirs []AgentDir
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if before, after, found := strings.Cut(line, "="); found {
			dirs = append(dirs, AgentDir{Name: strings.TrimSpace(before), Path: strings.TrimSpace(after)})
		} else {
			name := filepath.Base(filepath.Dir(line))
			if name == "." || name == "" {
				name = "custom"
			}
			dirs = append(dirs, AgentDir{Name: name, Path: line})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}
	if len(dirs) == 0 {
		return defaultAgentDirs(), nil
	}
	return dirs, nil
}

// copyDir copies all regular files from src directory into dst directory.
// Does not recurse into subdirectories (flat copy for skill files).
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("cannot read directory %q: %w", src, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		srcFile := filepath.Join(src, entry.Name())
		dstFile := filepath.Join(dst, entry.Name())
		if err := copyFile(srcFile, dstFile); err != nil {
			return err
		}
	}
	return nil
}

// copyFile copies a single file from src to dst, creating or overwriting dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open %q: %w", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("cannot create %q: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy failed %q → %q: %w", src, dst, err)
	}
	return nil
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path, home string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	if path == "~" {
		return home
	}
	return path
}
