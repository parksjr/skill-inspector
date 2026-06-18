package loader

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeGitHubBlobURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "github blob url",
			in:   "https://github.com/owner/repo/blob/main/SKILL.md",
			want: "https://raw.githubusercontent.com/owner/repo/main/SKILL.md",
		},
		{
			name: "www github blob url",
			in:   "https://www.github.com/owner/repo/blob/main/skills/my-skill/SKILL.md",
			want: "https://raw.githubusercontent.com/owner/repo/main/skills/my-skill/SKILL.md",
		},
		{
			name: "non github unchanged",
			in:   "https://example.com/owner/repo/blob/main/SKILL.md",
			want: "https://example.com/owner/repo/blob/main/SKILL.md",
		},
		{
			name: "non blob github unchanged",
			in:   "https://github.com/owner/repo/tree/main/skills",
			want: "https://github.com/owner/repo/tree/main/skills",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeGitHubBlobURL(tt.in)
			if got != tt.want {
				t.Fatalf("normalizeGitHubBlobURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLoadFromURLNormalizesGitHubBlobURL(t *testing.T) {
	blobURL := "https://github.com/owner/repo/blob/main/SKILL.md"
	expectedURL := "https://raw.githubusercontent.com/owner/repo/main/SKILL.md"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != expectedURL {
			t.Fatalf("request URL = %q, want %q", req.URL.String(), expectedURL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("# Demo Skill")),
			Request:    req,
		}, nil
	})

	sf, err := loadFromURL(blobURL)
	if err != nil {
		t.Fatalf("loadFromURL returned error: %v", err)
	}
	if sf.Content != "# Demo Skill" {
		t.Fatalf("content = %q, want %q", sf.Content, "# Demo Skill")
	}
	if sf.SourcePath != blobURL {
		t.Fatalf("sourcePath = %q, want %q", sf.SourcePath, blobURL)
	}
}

func TestLoadFromURLUsesFrontmatterName(t *testing.T) {
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("---\nname: My Cool Skill / v2!\n---\n# Demo Skill")),
			Request:    req,
		}, nil
	})

	sf, err := loadFromURL("https://example.com/SKILL.md")
	if err != nil {
		t.Fatalf("loadFromURL returned error: %v", err)
	}
	if sf.SkillName != "My-Cool-Skill-v2" {
		t.Fatalf("SkillName = %q, want %q", sf.SkillName, "My-Cool-Skill-v2")
	}
}

func TestLoadFromFileFallsBackWhenFrontmatterNameMissing(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(filePath, []byte("---\ntitle: No Name\n---\n# Demo Skill"), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	sf, err := loadFromFile(filePath)
	if err != nil {
		t.Fatalf("loadFromFile returned error: %v", err)
	}
	if sf.SkillName != "SKILL" {
		t.Fatalf("SkillName = %q, want %q", sf.SkillName, "SKILL")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
