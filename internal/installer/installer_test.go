package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// expandHome
// =============================================================================

func TestExpandHome(t *testing.T) {
	home := "/home/user"

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"tilde slash prefix", "~/docs", filepath.Join(home, "docs")},
		{"tilde only", "~", home},
		{"absolute path unchanged", "/etc/config", "/etc/config"},
		{"relative path unchanged", "relative/path", "relative/path"},
		{"no tilde prefix", "foo/bar", "foo/bar"},
		{"tilde mid-path unchanged", "foo/~/bar", "foo/~/bar"}, // not a prefix
		{"tilde without slash", "~foo", "~foo"},                // not ~/ pattern
		{"empty string", "", ""},
		{"tilde slash nested", "~/.config/app", filepath.Join(home, ".config/app")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := expandHome(tc.path, home)
			if got != tc.expected {
				t.Errorf("expandHome(%q, %q) = %q, want %q", tc.path, home, got, tc.expected)
			}
		})
	}
}

// =============================================================================
// defaultAgentDirs
// =============================================================================

func TestDefaultAgentDirs(t *testing.T) {
	dirs := defaultAgentDirs()
	if len(dirs) != 3 {
		t.Fatalf("expected 3 defaults, got %d", len(dirs))
	}

	expected := map[string]string{
		"claude": "~/.claude/skills",
		"goose":  "~/.config/goose/skills",
		"pi":     "~/.pi/skills",
	}

	for _, d := range dirs {
		if expected[d.Name] != d.Path {
			t.Errorf("agent %q: expected path %q, got %q", d.Name, expected[d.Name], d.Path)
		}
	}
}

// =============================================================================
// copyFile
// =============================================================================

func TestCopyFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	src := filepath.Join(srcDir, "source.txt")
	want := "hello world"
	if err := os.WriteFile(src, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dstDir, "dest.txt")
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("expected %q, got %q", want, string(got))
	}
}

func TestCopyFile_Overwrites(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	src := filepath.Join(srcDir, "source.txt")
	if err := os.WriteFile(src, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dstDir, "dest.txt")
	if err := os.WriteFile(dst, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile (overwrite) failed: %v", err)
	}

	got, _ := os.ReadFile(dst)
	if string(got) != "new content" {
		t.Errorf("expected 'new content', got %q", string(got))
	}
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	err := copyFile("/nonexistent/path/file.txt", "/tmp/dest.txt")
	if err == nil {
		t.Error("expected error for missing source")
	}
}

// =============================================================================
// copyDir
// =============================================================================

func TestCopyDir(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source files
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("# Skill"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("readme content"), 0o644)

	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify both files copied
	for _, name := range []string{"SKILL.md", "README.md"} {
		got, err := os.ReadFile(filepath.Join(dstDir, name))
		if err != nil {
			t.Errorf("expected %s to exist after copy: %v", name, err)
		}
		if len(got) == 0 {
			t.Errorf("%s is empty after copy", name)
		}
	}
}

func TestCopyDir_SkipsSubdirectories(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("skill"), 0o644)
	subDir := filepath.Join(srcDir, "examples")
	os.MkdirAll(subDir, 0o755)
	os.WriteFile(filepath.Join(subDir, "example.md"), []byte("example"), 0o644)

	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// examples/ should not exist in dst
	if _, err := os.Stat(filepath.Join(dstDir, "examples")); !os.IsNotExist(err) {
		t.Error("subdirectories should not be copied")
	}
}

func TestCopyDir_SkipsSymlinks(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("skill"), 0o644)
	realFile := filepath.Join(srcDir, "target.txt")
	os.WriteFile(realFile, []byte("target"), 0o644)
	os.Symlink(realFile, filepath.Join(srcDir, "link.txt"))

	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Symlink should not be copied
	if _, err := os.Lstat(filepath.Join(dstDir, "link.txt")); !os.IsNotExist(err) {
		t.Error("symlinks should not be copied")
	}
}

func TestCopyDir_SourceNotFound(t *testing.T) {
	err := copyDir("/nonexistent/dir", "/tmp/dst")
	if err == nil {
		t.Error("expected error for missing source directory")
	}
}

// =============================================================================
// verifyAgentDir
// =============================================================================

func TestVerifyAgentDir_ValidDir(t *testing.T) {
	d := t.TempDir()
	ad := AgentDir{Name: "test-agent", Path: d}
	if err := verifyAgentDir(ad); err != nil {
		t.Errorf("expected no error for valid dir, got: %v", err)
	}
}

func TestVerifyAgentDir_NotExist(t *testing.T) {
	ad := AgentDir{Name: "missing", Path: "/nonexistent/path"}
	if err := verifyAgentDir(ad); err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestVerifyAgentDir_IsSymlink(t *testing.T) {
	d := t.TempDir()
	realDir := filepath.Join(d, "real")
	linkDir := filepath.Join(d, "link")
	os.MkdirAll(realDir, 0o755)
	os.Symlink(realDir, linkDir)

	ad := AgentDir{Name: "symlink-agent", Path: linkDir}
	if err := verifyAgentDir(ad); err == nil {
		t.Error("expected error for symlink agent dir")
	}
}

func TestVerifyAgentDir_IsFile(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "file.txt")
	os.WriteFile(f, []byte("not a dir"), 0o644)

	ad := AgentDir{Name: "file-agent", Path: f}
	if err := verifyAgentDir(ad); err == nil {
		t.Error("expected error when path is a file")
	}
}

// =============================================================================
// linkSkill
// =============================================================================

func TestLinkSkill_Success(t *testing.T) {
	home := t.TempDir()
	installDir := filepath.Join(home, "install")
	agentDir := filepath.Join(home, "agent")
	os.MkdirAll(installDir, 0o755)
	os.MkdirAll(agentDir, 0o755)

	ad := AgentDir{Name: "test", Path: agentDir}
	lr := linkSkill("my-skill", installDir, ad)

	if !lr.Linked {
		t.Errorf("expected Linked=true, got %+v", lr)
	}
	if lr.Skipped {
		t.Errorf("expected Skipped=false, got %+v", lr)
	}
	if lr.Err != nil {
		t.Errorf("expected nil error, got %v", lr.Err)
	}

	// Verify symlink was created
	linkPath := filepath.Join(agentDir, "my-skill")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("symlink not found: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink")
	}
}

func TestLinkSkill_AgentDirMissing(t *testing.T) {
	ad := AgentDir{Name: "missing", Path: "/nonexistent/path"}
	lr := linkSkill("my-skill", "/tmp/install", ad)

	if lr.Linked {
		t.Error("expected Linked=false for missing agent dir")
	}
	if !lr.Skipped {
		t.Error("expected Skipped=true for missing agent dir")
	}
}

func TestLinkSkill_CollisionWithNonSymlink(t *testing.T) {
	home := t.TempDir()
	installDir := filepath.Join(home, "install")
	agentDir := filepath.Join(home, "agent")
	os.MkdirAll(installDir, 0o755)
	os.MkdirAll(agentDir, 0o755)

	// Create a real file (not symlink) at the link path
	linkPath := filepath.Join(agentDir, "my-skill")
	os.WriteFile(linkPath, []byte("blocking file"), 0o644)

	ad := AgentDir{Name: "test", Path: agentDir}
	lr := linkSkill("my-skill", installDir, ad)

	if lr.Linked {
		t.Error("expected Linked=false when collision is not a symlink")
	}
	if lr.Err == nil {
		t.Error("expected error for non-symlink collision")
	}
}

func TestLinkSkill_ReplaceExistingSymlink(t *testing.T) {
	home := t.TempDir()
	installDir := filepath.Join(home, "install")
	agentDir := filepath.Join(home, "agent")
	os.MkdirAll(installDir, 0o755)
	os.MkdirAll(agentDir, 0o755)

	// Create an existing symlink (pointing elsewhere)
	oldTarget := filepath.Join(home, "old-target")
	os.MkdirAll(oldTarget, 0o755)
	linkPath := filepath.Join(agentDir, "my-skill")
	os.Symlink(oldTarget, linkPath)

	ad := AgentDir{Name: "test", Path: agentDir}
	lr := linkSkill("my-skill", installDir, ad)

	if !lr.Linked {
		t.Errorf("expected Linked=true when replacing symlink, got %+v", lr)
	}
	if lr.Err != nil {
		t.Errorf("expected nil error, got %v", lr.Err)
	}

	// Verify old symlink was replaced
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink failed: %v", err)
	}
	if target != installDir {
		t.Errorf("expected symlink target %q, got %q", installDir, target)
	}
}

// =============================================================================
// dirExists
// =============================================================================

func TestDirExists(t *testing.T) {
	d := t.TempDir()
	if !dirExists(d) {
		t.Error("temp dir should exist")
	}
}

func TestDirExists_NotExist(t *testing.T) {
	if dirExists("/nonexistent/path") {
		t.Error("nonexistent path should not exist")
	}
}

func TestDirExists_IsFile(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "file.txt")
	os.WriteFile(f, []byte("content"), 0o644)
	if dirExists(f) {
		t.Error("file should not report as dir")
	}
}

// =============================================================================
// loadAgentDirs
// =============================================================================

func TestLoadAgentDirs_NoConfig(t *testing.T) {
	home := t.TempDir()
	dirs, err := loadAgentDirs(home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirs) != 3 {
		t.Errorf("expected 3 default dirs when config absent, got %d", len(dirs))
	}
}

func TestLoadAgentDirs_CustomConfig(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "skill-inspector")
	os.MkdirAll(configDir, 0o755)
	configFile := filepath.Join(configDir, "config")
	os.WriteFile(configFile, []byte("/custom/path\n"), 0o644)

	dirs, err := loadAgentDirs(home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirs) != 1 {
		t.Fatalf("expected 1 custom dir, got %d", len(dirs))
	}
	if dirs[0].Path != "/custom/path" {
		t.Errorf("expected /custom/path, got %q", dirs[0].Path)
	}
}

func TestLoadAgentDirs_ConfigWithComments(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "skill-inspector")
	os.MkdirAll(configDir, 0o755)
	configFile := filepath.Join(configDir, "config")
	os.WriteFile(configFile, []byte("# comment line\n\n/real/path\n# another comment\n"), 0o644)

	dirs, err := loadAgentDirs(home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirs) != 1 {
		t.Fatalf("expected 1 dir (comments skipped), got %d", len(dirs))
	}
	if dirs[0].Path != "/real/path" {
		t.Errorf("expected /real/path, got %q", dirs[0].Path)
	}
}

func TestLoadAgentDirs_NameEqualsPathTreatedAsLiteral(t *testing.T) {
	// The "name=path" format is no longer supported. Lines containing "="
	// are treated as literal directory paths.
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "skill-inspector")
	os.MkdirAll(configDir, 0o755)
	configFile := filepath.Join(configDir, "config")
	os.WriteFile(configFile, []byte("claude=~/.claude/skills\n"), 0o644)

	dirs, err := loadAgentDirs(home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirs) != 1 {
		t.Fatalf("expected 1 dir, got %d", len(dirs))
	}
	// Entire line is treated as a literal path.
	if dirs[0].Path != "claude=~/.claude/skills" {
		t.Errorf("expected literal path, got %q", dirs[0].Path)
	}
}

func TestLoadAgentDirs_EmptyConfig(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "skill-inspector")
	os.MkdirAll(configDir, 0o755)
	configFile := filepath.Join(configDir, "config")
	os.WriteFile(configFile, []byte(""), 0o644)

	dirs, err := loadAgentDirs(home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty config returns defaults
	if len(dirs) != 3 {
		t.Errorf("expected 3 defaults for empty config, got %d", len(dirs))
	}
}

func TestLoadAgentDirs_ConfigIsSymlink(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "skill-inspector")
	os.MkdirAll(configDir, 0o755)

	realFile := filepath.Join(home, "real-config")
	os.WriteFile(realFile, []byte("/custom/path\n"), 0o644)

	configFile := filepath.Join(configDir, "config")
	os.Symlink(realFile, configFile)

	_, err := loadAgentDirs(home)
	if err == nil {
		t.Error("expected error for symlink config")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink: %v", err)
	}
}

func TestLoadAgentDirs_PathOnlyFormatDerivesName(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "skill-inspector")
	os.MkdirAll(configDir, 0o755)
	configFile := filepath.Join(configDir, "config")
	os.WriteFile(configFile, []byte("~/.my-agent/skills\n"), 0o644)

	dirs, err := loadAgentDirs(home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dirs[0].Name != ".my-agent" {
		t.Errorf("expected name derived from parent dir '.my-agent', got %q", dirs[0].Name)
	}
}

// =============================================================================
// resolvedAgentDirs — expandHome integration
// =============================================================================

func TestResolvedAgentDirs_ExpandsTilde(t *testing.T) {
	// Create no config → uses defaults with expansion.
	tmpHome := t.TempDir()
	dirs := resolvedAgentDirs(tmpHome)
	if len(dirs) != 3 {
		t.Fatalf("expected 3 dirs, got %d", len(dirs))
	}
	for _, d := range dirs {
		if strings.HasPrefix(d.Path, "~") {
			t.Errorf("path %q should have been expanded (no tilde)", d.Path)
		}
		if !filepath.IsAbs(d.Path) {
			t.Errorf("path %q should be absolute after expansion", d.Path)
		}
	}
}
