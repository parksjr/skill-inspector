# skill-inspector

**A zero-dependency CLI/TUI tool for inspecting agent skill files before installation.**

Agent skills can carry hidden malicious instructions through prompt injection attacks — zero-width Unicode characters, invisible HTML comments, and YAML frontmatter are all commonly used to insert instructions that LLM agents will execute without the user ever seeing them. `skill-inspector` surfaces this hidden content before you install a skill, so you can audit what you're actually running.

## Showcase

**Inspecting a clean skill** — source view with syntax highlighting, hidden content view shows nothing suspicious:

![Inspecting a clean skill file](demos/demo-clean.gif)

**Catching a malicious skill** — source looks innocent, but the hidden content view exposes YAML frontmatter with hidden directives, an HTML comment with override instructions, and embedded zero-width Unicode characters:

![Catching a malicious skill file](demos/demo-malicious.gif)

## Installation

### Quick Install

```sh
curl -fsSL https://raw.githubusercontent.com/parksjr/skill-inspector/main/install.sh | sh
```

The script downloads the prebuilt binary for your platform and installs it to `/usr/local/bin/skill-inspector`.

### Build from Source

Requires Go 1.21 or later.

```sh
git clone https://github.com/parksjr/skill-inspector.git
cd skill-inspector
go build -o skill-inspector .
sudo mv skill-inspector /usr/local/bin/
```

## Usage

You can inspect a skill from a local file, directory, or remote HTTP/HTTPS URL:

```sh
# Inspect a local skill file
skill-inspector ./my-skill/SKILL.md

# Inspect a directory (looks for any .md files)
skill-inspector ./my-skill/

# Inspect from a remote URL
skill-inspector https://raw.githubusercontent.com/user/repo/main/skills/my-skill/SKILL.md
```

The tool opens an interactive terminal UI showing the skill's source code with ANSI color syntax highlighting. Use the keyboard controls below to navigate and audit the content.

## TUI Controls

| Key        | Action                                         |
| ---------- | ---------------------------------------------- |
| Tab        | Toggle between Source and Hidden Content views |
| j / ↓      | Scroll down one line                           |
| k / ↑      | Scroll up one line                             |
| Space      | Page down                                      |
| b          | Page up                                        |
| i          | Install the skill (interactive confirmation)   |
| q / Ctrl+C | Quit                                           |

## Hidden Content View

When you toggle to the **Hidden Content** view (press Tab), the tool reveals three categories of potentially suspicious content:

### 1. Frontmatter

YAML or TOML metadata at the start of the file (enclosed in `---` or `+++`). This often contains skill metadata but can also include hidden instructions or configuration that agents will process.

### 2. HTML Comments

All `<!-- ... -->` comments in the file. HTML comments are invisible in rendered Markdown and commonly used to hide instructions.

### 3. Suspicious Characters

Unicode codepoints that may be used for obfuscation or prompt injection:

- **Zero-Width Space** (U+200B) — completely invisible, used to split keywords
- **Zero-Width Joiner** (U+200D) — invisible connector, obscures text
- **Byte Order Mark** (U+FEFF) — invisible at file start
- **Soft Hyphen** (U+00AD) — invisible line break indicator
- **Right-to-Left Mark** (U+200F) — reverses text direction
- **Homoglyphs** — visually identical but distinct Unicode characters (e.g., Latin 'a' vs Cyrillic 'а')

The tool reports the character name, Unicode codepoint, position in file, and surrounding context for each suspicious character found.

## Installing a Skill

To install a skill you've audited:

1. Open the skill file in `skill-inspector`
2. Press `i` to start the installation
3. Confirm the prompt (you'll be asked to type "y" to proceed)
4. The skill is installed to `~/.agents/skills/<name>/`
5. Symlinks are automatically created in detected agent skill directories

**Note:** Symlinks are only created for agent directories that already exist on your system. The tool checks for Claude, Goose, and Pi installations and creates symlinks in their respective skill directories if they're found.

## Configuration

By default, `skill-inspector` knows about these agent skill directories:

| Agent  | Default Path             |
| ------ | ------------------------ |
| Claude | `~/.claude/skills`       |
| Goose  | `~/.config/goose/skills` |
| Pi     | `~/.pi/skills`           |

To customize or add agent directories, create `~/.config/skill-inspector/config`:

```
# One entry per line: agent_name=path  OR  just a path
# Lines starting with # are comments
claude=~/.claude/skills
goose=~/.config/goose/skills
pi=~/.pi/skills
```

**Important:** If you create a custom config file, it _replaces_ all built-in defaults entirely. Include all agents you want symlinks for.

## Security

This tool exists because **skill files are code**. Agent skills are executed by LLM agents with access to your tools, files, and API keys. A malicious or compromised skill can:

- Hide malicious instructions in zero-width Unicode characters
- Embed instructions in HTML comments that won't appear in rendered Markdown
- Use YAML frontmatter to set dangerous parameters agents will obey
- Use Unicode homoglyphs to disguise function names or API endpoints

`skill-inspector` makes all of this visible before installation.

### Dependency Security

`skill-inspector` is written in Go with **zero external runtime dependencies** for the binary itself. The single build dependency is `golang.org/x/term`, maintained by the Go team, with zero transitive dependencies. It's used only to set raw terminal mode for TUI rendering.

## Project Structure

```
skill-inspector/
├── main.go                 # Entry point, CLI parsing
├── tui/
│   ├── tui.go             # TUI event loop and rendering
│   ├── view.go            # View state and mode management
│   └── colors.go          # ANSI color definitions
├── inspector/
│   ├── loader.go          # Load from file, directory, or URL
│   ├── parser.go          # Extract frontmatter, comments, suspicious chars
│   └── characters.go      # Unicode detection and homoglyph analysis
├── installer/
│   ├── install.go         # Skill installation flow
│   └── symlink.go         # Create agent-specific symlinks
└── go.mod                 # Go module definition (no external deps)
```

## License

MIT

---

**Questions? Found a bug?** Open an issue on [GitHub](https://github.com/parksjr/skill-inspector).
