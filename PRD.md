# PRD: skill-inspector

## Overview

`skill-inspector` is a zero-dependency, single-binary CLI tool for inspecting and installing agent skill files (`.md`). It targets security-conscious users who want to manually review skill files — including hidden content (frontmatter, HTML comments, invisible Unicode) — before installing them into their local agent toolchains.

---

## Goals & Success Criteria

- A user can pass a local file path or direct raw URL to a `SKILL.md` file and immediately view its contents in a terminal
- Hidden content (frontmatter, HTML comments, suspicious characters) is surfaced in a clearly separated view — not mixed with the readable source
- A user can install the skill to a canonical local directory and auto-symlink it to any detected agent directories, all from within the inspector
- The binary runs on macOS, Linux, and Unix with no runtime dependencies — nothing to install before using the tool
- Installation of the tool itself is a single `curl | sh` command

## Non-Goals

- Full markdown-to-HTML/terminal rendering (this is an inspection tool, not a viewer)
- GitHub directory/repository URL support (direct raw file URLs only, v1)
- GUI or browser-based interface
- Windows support (v1)
- Third-party Go module dependencies (stdlib only)

---

## Users & Scope

**Primary user:** The author themselves — a developer who regularly installs agent skills and wants a fast, trustworthy way to audit them before use.

**Secondary users:** Coworkers who may be shared the tool via the public GitHub repo.

**Out of scope:** Multi-user environments, access control, remote skill registries.

---

## CLI Interface

### Invocation

```sh
skill-inspector <url-or-file-path>
```

- `<url-or-file-path>`: A local file path (single `.md` file or directory containing `SKILL.md`) or a direct HTTP/HTTPS URL returning raw file contents.
- No subcommands. All functionality lives inside the TUI after the file is loaded.

### Examples

```sh
skill-inspector ./my-skill/SKILL.md
skill-inspector https://raw.githubusercontent.com/user/repo/main/skills/my-skill/SKILL.md
skill-inspector ./my-skill/          # directory with SKILL.md inside
```

---

## TUI: Views

The TUI has two views, toggled with `Tab`. One view is shown at a time.

### View 1 — Source (default)

Displays the raw source of the `SKILL.md` file with ANSI colorization to aid readability:

- **Headers** (`#`, `##`, etc.) — bold + cyan
- **Code blocks** (fenced ` ``` `) — dim/grey background
- **HTML comments** (`<!-- -->`) — highlighted in yellow (still shown, not hidden)
- **Frontmatter block** (`---` delimiters) — highlighted in magenta
- **Bold/italic markers** — subtle highlight to make them visible without rendering
- Everything else: default terminal color

> The raw syntax is always visible. Nothing is rendered/hidden in this view.

### View 2 — Hidden Content

Displays extracted hidden/suspicious content in labeled sections. If a section has nothing to report, it shows `✓ None found`.

#### Sections:

**Frontmatter**
- Content between opening and closing `---` at the top of the file
- Displayed as raw YAML with line numbers

**HTML Comments**
- All `<!-- ... -->` blocks extracted and listed
- Each shown with its line number range

**Suspicious Characters**
- Zero-width characters (`U+200B`, `U+200C`, `U+200D`, `U+FEFF`, `U+00AD`, etc.)
- Unicode homoglyphs (characters outside standard ASCII/Latin that visually resemble common characters)
- Unusual whitespace (non-breaking space `U+00A0`, em space, hair space, etc.)
- Each finding reported as: `[U+200B ZERO WIDTH SPACE — line 14, col 7]`

---

## TUI: Keyboard Controls

| Key | Action |
|-----|--------|
| `Tab` | Toggle between Source and Hidden Content views |
| `j` / `↓` | Scroll down |
| `k` / `↑` | Scroll up |
| `i` | Initiate install (prompts for confirmation) |
| `q` / `Ctrl+C` | Quit |

---

## Install Flow (triggered by `i`)

1. Derive skill name from directory name (if dir input) or filename stem (e.g. `my-skill.md` → `my-skill`)
2. Prompt user to confirm: `Install "my-skill" to ~/.agents/skills/my-skill/? [y/N]`
3. On confirm:
   - Create `~/.agents/skills/<skill-name>/`
   - Copy `SKILL.md` (and any sibling files if input was a directory) into it
   - Probe each configured agent directory path
   - For each that exists: create a symlink `<agent-dir>/<skill-name>` → `~/.agents/skills/<skill-name>/`
4. Print summary:
   ```
   ✓ Installed to ~/.agents/skills/my-skill/
   ✓ Linked: claude  (~/.claude/skills/my-skill)
   ✓ Linked: goose   (~/.config/goose/skills/my-skill)
   — Skipped: pi     (directory not found)
   ```

---

## Agent Directory Detection

### Hardcoded defaults (probed in order, symlinked if they exist)

| Agent | Default path |
|-------|-------------|
| claude | `~/.claude/skills/` |
| goose | `~/.config/goose/skills/` |
| pi | `~/.pi/skills/` |

### User config override

Path: `~/.config/skill-inspector/config`

Format: one directory path per line (comments with `#` supported).

```
# Custom agent skill directories
~/.my-custom-agent/skills/
~/work/agent-tools/skills/
```

Entries in the config file **replace** the hardcoded defaults entirely, giving the user full control.

---

## URL Input Behavior

- Only direct file URLs are supported (single HTTP GET, raw file content returned in body)
- Supports `http://` and `https://`
- On fetch error (non-200, network failure): print error and exit with non-zero code
- Downloaded content is held in memory only — not written to disk unless the user explicitly installs

---

## Skill Directory Structure

A skill is a directory containing at minimum a `SKILL.md` file, plus optional companion files:

```
my-skill/
  SKILL.md        ← required
  README.md       ← optional
  examples/       ← optional
```

When a single `.md` file is passed (not a directory), the tool treats it as the `SKILL.md` and creates the directory structure on install.

---

## Distribution & Installation

### Binary distribution

- Hosted as GitHub Releases assets on the public repo
- Built for: `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`
- Binary name: `skill-inspector`
- Release artifacts: `skill-inspector-darwin-amd64`, `skill-inspector-darwin-arm64`, `skill-inspector-linux-amd64`, `skill-inspector-linux-arm64`

### Install script

Users install via:

```sh
curl -fsSL https://raw.githubusercontent.com/<user>/skill-inspector/main/install.sh | sh
```

The `install.sh` script:
1. Detects OS (`uname -s`) and architecture (`uname -m`)
2. Downloads the appropriate binary from the latest GitHub Release
3. Makes it executable
4. Places it in `~/.local/bin/` (creates dir if needed, warns if not in PATH)
5. Prints confirmation: `skill-inspector installed to ~/.local/bin/skill-inspector`

### Build requirements (developer only)

- Go 1.21+ (stdlib only, no `go get` needed beyond the standard toolchain)
- `go build -o skill-inspector .` produces the binary

---

## Technical Constraints

- **Language:** Go
- **Dependencies:** stdlib only — no third-party modules (`go.sum` should be empty or absent)
- **TUI:** Raw terminal mode via `golang.org/x/term` — wait, that's external. Use `syscall` + `os` for raw mode; ANSI escape codes for color/cursor control
- **HTTP:** `net/http` stdlib
- **File I/O:** `os`, `bufio`, `strings`, `unicode/utf8` stdlib packages

> Note: `golang.org/x/term` is a Go extended stdlib package maintained by the Go team — if raw terminal handling via `syscall` proves brittle across platforms, this is a considered acceptable single exception given it has no transitive deps and is maintained by the Go core team. Decision deferred to implementation.

---

## Open Items

- Confirm exact default paths for goose and pi agent directories (verify against their official documentation/source)
- Decide on `golang.org/x/term` exception vs. pure `syscall` for raw terminal mode
- GitHub Actions workflow for automated cross-platform builds on tag push

---

## Out of Scope (Future)

- GitHub repo/directory URL support
- Skill update/upgrade command
- Listing installed skills (`skill-inspector list`)
- Uninstall command
- Windows support
