package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/parksjr/skill-inspector/internal/installer"
)

func TestBuildInstallPreviewLinesShowsMissingNameAlert(t *testing.T) {
	preview := &installer.InstallPreview{
		SkillName:   "SKILL",
		InstallPath: filepath.Join("/home/test", ".agents", "skills", "SKILL"),
		Links: []installer.PlannedLink{
			{
				Agent:       "claude",
				Source:      filepath.Join("/home/test", ".agents", "skills", "SKILL"),
				Destination: filepath.Join("/home/test", ".claude", "skills", "SKILL"),
				Available:   true,
			},
		},
	}

	lines := buildInstallPreviewLines(preview, true)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, `frontmatter is missing "name"`) {
		t.Fatalf("expected missing-name warning in preview lines")
	}
	if !strings.Contains(joined, preview.InstallPath) {
		t.Fatalf("expected install path %q in preview lines", preview.InstallPath)
	}
}

func TestBuildInstallPreviewLinesOmitsAlertWhenNamePresent(t *testing.T) {
	preview := &installer.InstallPreview{
		SkillName:   "my-skill",
		InstallPath: filepath.Join("/home/test", ".agents", "skills", "my-skill"),
	}

	lines := buildInstallPreviewLines(preview, false)
	joined := strings.Join(lines, "\n")

	if strings.Contains(joined, `frontmatter is missing "name"`) {
		t.Fatalf("did not expect missing-name warning when frontmatter name exists")
	}
}
