package installer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// excludedAgents is a set of agent names to skip during install.
// Populated via ExcludeAgent from CLI flags (e.g. --no-symlink-goose).
var excludedAgents = make(map[string]bool)

// ExcludeAgent adds an agent name to the exclusion set so symlinks
// are not created for that agent during install.
func ExcludeAgent(name string) {
	excludedAgents[strings.ToLower(name)] = true
}

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

// PlannedLink describes one symlink operation before install runs.
type PlannedLink struct {
	Agent       string
	Source      string
	Destination string
	Available   bool // false when destination directory does not exist
}

// InstallPreview describes the install target and symlink plan.
type InstallPreview struct {
	SkillName   string
	InstallPath string
	Links       []PlannedLink
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
		return nil, fmt.Errorf("cannot determine home directory: %w — ensure $HOME is set", err)
	}

	installBase := filepath.Join(home, ".agents", "skills")
	installDir := filepath.Join(installBase, skillName)

	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create install directory %q: %w — check write permissions for ~/.agents/skills", installDir, err)
	}

	if isURL {
		dest := filepath.Join(installDir, "SKILL.md")
		if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("cannot write SKILL.md: %w — check disk space and write permissions", err)
		}
	} else {
		info, err := os.Stat(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("cannot stat source %q: %w — verify the path exists and is readable", sourcePath, err)
		}
		if info.IsDir() {
			if err := copyDir(sourcePath, installDir); err != nil {
				return nil, fmt.Errorf("cannot copy directory: %w — check source directory permissions", err)
			}
		} else {
			dest := filepath.Join(installDir, "SKILL.md")
			if err := copyFile(sourcePath, dest); err != nil {
				return nil, fmt.Errorf("cannot copy file: %w — check source file permissions", err)
			}
		}
	}

	agentDirs := resolvedAgentDirs(home)

	result := &InstallResult{
		SkillName:   skillName,
		InstallPath: installDir,
	}
	for _, ad := range agentDirs {
		if err := verifyAgentDir(ad); err != nil {
			// Log once then skip this agent dir.
			continue
		}
		lr := linkSkill(skillName, installDir, ad)
		result.Links = append(result.Links, lr)
	}

	return result, nil
}

// PlanInstall returns where files will be installed and which symlinks are planned.
func PlanInstall(skillName string) (*InstallPreview, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w — ensure $HOME is set", err)
	}

	installDir := filepath.Join(home, ".agents", "skills", skillName)
	agentDirs := resolvedAgentDirs(home)

	preview := &InstallPreview{
		SkillName:   skillName,
		InstallPath: installDir,
	}
	for _, ad := range agentDirs {
		dest := filepath.Join(ad.Path, skillName)
		preview.Links = append(preview.Links, PlannedLink{
			Agent:       ad.Name,
			Source:      installDir,
			Destination: dest,
			Available:   dirExists(ad.Path),
		})
	}
	return preview, nil
}

func resolvedAgentDirs(home string) []AgentDir {
	agentDirs, err := loadAgentDirs(home)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skill-inspector: warning: cannot load agent dirs config: %v — using defaults\n", err)
		agentDirs = defaultAgentDirs()
	}
	for i := range agentDirs {
		agentDirs[i].Path = expandHome(agentDirs[i].Path, home)
	}

	// Filter out agents excluded via --no-symlink-<agent> flags.
	if len(excludedAgents) > 0 {
		filtered := agentDirs[:0]
		for _, ad := range agentDirs {
			if !excludedAgents[strings.ToLower(ad.Name)] {
				filtered = append(filtered, ad)
			}
		}
		agentDirs = filtered
	}

	return agentDirs
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
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
				lr.Err = fmt.Errorf("cannot remove existing symlink: %w — check permissions on the symlink", err)
				return lr
			}
		} else {
			lr.Err = fmt.Errorf("%q exists and is not a symlink — skipping — remove or rename the existing item and try again", linkPath)
			return lr
		}
	}

	if err := os.Symlink(installDir, linkPath); err != nil {
		lr.Err = fmt.Errorf("symlink failed: %w — check that the parent directory exists and is writable", err)
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

	// Verify config is a regular file (not a symlink).
	info, err := os.Lstat(configPath)
	if os.IsNotExist(err) {
		return defaultAgentDirs(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot stat config %q: %w — check ~/.config/skill-inspector/config", configPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("config %q is a symlink — refusing to open — replace the symlink with a regular file", configPath)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("config %q is not a regular file — remove and recreate as a regular file", configPath)
	}

	f, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open config %q: %w — check file permissions", configPath, err)
	}
	defer f.Close()

	var dirs []AgentDir
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Path-only format: derive agent name from parent directory.
		name := filepath.Base(filepath.Dir(line))
		if name == "." || name == "" {
			name = "custom"
		}
		dirs = append(dirs, AgentDir{Name: name, Path: line})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config: %w — verify config file format", err)
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
		return fmt.Errorf("cannot read directory %q: %w — check directory permissions", src, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
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
		return fmt.Errorf("cannot open %q: %w — check file permissions", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("cannot create %q: %w — check directory permissions and disk space", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy failed %q → %q: %w — check disk space", src, dst, err)
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

// verifyAgentDir checks that an agent directory exists, is a directory, and is
// not a symlink. Used as a TOCTOU defense at install time.
func verifyAgentDir(ad AgentDir) error {
	info, err := os.Lstat(ad.Path)
	if err != nil {
		return fmt.Errorf("agent dir %q (agent=%s): %w — ensure the agent skills directory exists", ad.Path, ad.Name, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("agent dir %q (agent=%s) is a symlink — refusing to use — replace the symlink with a real directory", ad.Path, ad.Name)
	}
	if !info.IsDir() {
		return fmt.Errorf("agent dir %q (agent=%s) is not a directory — remove the file and create a directory instead", ad.Path, ad.Name)
	}
	return nil
}
