package loader

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
	return nil, nil
}
