//go:build integration
// +build integration

package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallFlow(t *testing.T) {
	// Clean up any prior test install
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}
	testInstallDir := filepath.Join(home, ".agents", "skills", "test-skill-install")
	os.RemoveAll(testInstallDir)

	// Test 1: Install from local directory
	t.Run("Install from local directory", func(t *testing.T) {
		result, err := Install("test-skill-install", "/tmp/test-full", "", false)
		if err != nil {
			t.Fatalf("Install failed: %v", err)
		}
		t.Logf("✓ Installed to: %s", result.InstallPath)
		for _, lr := range result.Links {
			switch {
			case lr.Err != nil:
				t.Logf("  ✗ Error  %s: %v", lr.Agent, lr.Err)
			case lr.Skipped:
				t.Logf("  — Skipped %s (dir not found)", lr.Agent)
			case lr.Linked:
				t.Logf("  ✓ Linked  %s → %s", lr.Agent, lr.Path)
			}
		}

		// Verify SKILL.md was copied
		skillMD := filepath.Join(result.InstallPath, "SKILL.md")
		if _, err := os.Stat(skillMD); err != nil {
			t.Fatalf("SKILL.md not found at %s: %v", skillMD, err)
		}
		t.Logf("✓ SKILL.md exists at %s", skillMD)
	})

	// Test 2: Idempotent reinstall (should not error)
	t.Run("Idempotent reinstall", func(t *testing.T) {
		result2, err := Install("test-skill-install", "/tmp/test-full", "", false)
		if err != nil {
			t.Fatalf("Reinstall errored: %v", err)
		}
		t.Logf("✓ Reinstall succeeded: %s", result2.InstallPath)
	})

	// Test 3: Install from URL content (isURL=true)
	t.Run("Install from URL content", func(t *testing.T) {
		os.RemoveAll(filepath.Join(home, ".agents", "skills", "url-skill"))
		content := "---\ntitle: URL Skill\n---\n# URL Skill\nInstalled from URL.\n"
		result3, err := Install("url-skill", "", content, true)
		if err != nil {
			t.Fatalf("URL install failed: %v", err)
		}
		t.Logf("✓ URL skill installed to: %s", result3.InstallPath)
		urlSkillMD := filepath.Join(result3.InstallPath, "SKILL.md")
		if _, err := os.Stat(urlSkillMD); err != nil {
			t.Fatalf("SKILL.md not found for URL install: %v", err)
		}
		t.Logf("✓ SKILL.md exists at %s", urlSkillMD)
	})

	// Test 4: Config file override
	t.Run("Config file override", func(t *testing.T) {
		configDir := filepath.Join(home, ".config", "skill-inspector")
		os.MkdirAll(configDir, 0o755)
		configFile := filepath.Join(configDir, "config")
		// Write a config with a custom dir that exists (/tmp) and one that doesn't
		os.WriteFile(configFile, []byte("# test config\ncustom-exists=/tmp\ncustom-missing=/tmp/nonexistent-agent-dir\n"), 0o644)
		defer os.Remove(configFile)

		os.RemoveAll(filepath.Join(home, ".agents", "skills", "config-test"))
		result4, err := Install("config-test", "/tmp/test-full", "", false)
		if err != nil {
			t.Fatalf("Config override install failed: %v", err)
		}
		foundLinked := false
		foundSkipped := false
		for _, lr := range result4.Links {
			if lr.Agent == "custom-exists" && lr.Linked {
				foundLinked = true
				t.Logf("✓ custom-exists linked correctly")
			}
			if lr.Agent == "custom-missing" && lr.Skipped {
				foundSkipped = true
				t.Logf("✓ custom-missing skipped correctly")
			}
		}
		if !foundLinked || !foundSkipped {
			t.Logf("FAIL: config override did not work as expected. Links: %+v", result4.Links)
		}
	})

	// Clean up test installs
	os.RemoveAll(testInstallDir)
	os.RemoveAll(filepath.Join(home, ".agents", "skills", "url-skill"))
	os.RemoveAll(filepath.Join(home, ".agents", "skills", "config-test"))
	// Clean up symlink in /tmp if created
	os.Remove(filepath.Join("/tmp", "config-test"))

	t.Logf("✓ All installer tests passed!")
}
