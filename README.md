# skill-inspector

**A minimal-dependency CLI/TUI tool for inspecting agent skill files before installation.**

Agent skills can carry hidden instructions through prompt injection techniques — invisible HTML comments, YAML frontmatter, and unusual Unicode characters can all hide content an agent may read but you might not notice. `skill-inspector` surfaces this hidden content before you install a skill, so you can audit what you're actually running.

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

The script downloads the prebuilt binary for your platform and installs it to `~/.local/bin/skill-inspector`. If `~/.local/bin` is not in your PATH, the installer will warn you with instructions to add it.

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

# Inspect a directory (looks for SKILL.md)
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
YAML frontmatter at the start of the file (enclosed in `---`). This often contains skill metadata but can also include hidden instructions or configuration that agents will process.

### 2. HTML Comments

All `<!-- ... -->` comments in the file. HTML comments are invisible in rendered Markdown and commonly used to hide instructions.

### 3. Suspicious Characters
Known Unicode codepoints that may be used for obfuscation or prompt injection, including:
- **Zero-Width Space** (U+200B)
- **Zero-Width Non-Joiner** (U+200C)
- **Zero-Width Joiner** (U+200D)
- **Zero-Width No-Break Space / BOM** (U+FEFF)
- **Soft Hyphen** (U+00AD)
- **No-Break Space** (U+00A0)
- A broader set of invisible formatting/spacing characters

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

| Agent  | Default Path             | Platform    | Verified |
| ------ | ------------------------ | ----------- | -------- |
| Claude | `~/.claude/skills`       | macOS/Linux | ✓ (Claude Code docs) |
| Goose  | `~/.config/goose/skills` | macOS/Linux | ✓ (XDG config convention) |
| Pi     | `~/.pi/skills`           | macOS/Linux | Best-effort default |

These defaults are probed at install time. Only directories that already exist on your system receive symlinks. You can override all defaults with a [config file](#configuration).

To customize or add agent directories, create `~/.config/skill-inspector/config`:

```
# One skill directory path per line.
# Lines starting with # are comments.
# Replaces all built-in defaults — include all agents you want.
~/.claude/skills
~/.config/goose/skills
~/.pi/skills
```

**Important:** If you create a custom config file, it _replaces_ all built-in defaults entirely. Include all agents you want symlinks for.

## Security

This tool exists because **skill files are code**. Agent skills are executed by LLM agents with access to your tools, files, and API keys. A malicious or compromised skill can:

- Hide malicious instructions in zero-width Unicode characters
- Embed instructions in HTML comments that won't appear in rendered Markdown
- Use YAML frontmatter to set dangerous parameters agents will obey

`skill-inspector` makes all of this visible before installation.

`skill-inspector` is intentionally human-led: it does **not** score risk, label vulnerabilities, or auto-block installs. It surfaces evidence so you can decide.

### Dependency Security

`skill-inspector` is a self-contained, single-binary Go application with no runtime dependencies. It uses two compile-time Go modules for terminal handling:

| Dependency | Purpose | Notes |
|---|---|---|
| `golang.org/x/term` | Raw terminal mode for the TUI | Maintained by the Go core team. Required for reading keystrokes without waiting for Enter. |
| `golang.org/x/sys` | Indirect, pulled by `x/term` | Platform-specific terminal syscalls. No additional transitive dependencies. |

Both are part of the `golang.org/x` extended standard library, maintained under the same security standards as the Go standard library. The binary has no other dependencies — no third-party Go modules, no shared libraries, no runtime downloads.

## Project Structure

```
skill-inspector/
├── main.go                 # Entry point, CLI parsing
├── internal/
│   ├── colorize/          # ANSI syntax coloring
│   ├── installer/         # Install flow and symlink management
│   ├── loader/            # Load from file, directory, or URL
│   ├── parser/            # Extract frontmatter, comments, suspicious chars
│   └── tui/               # TUI event loop and rendering
└── go.mod                 # Go module definition
```

## License

MIT

---

**Questions? Found a bug?** Open an issue on [GitHub](https://github.com/parksjr/skill-inspector).
