---
goal: Implement skill-inspector — a zero-dependency CLI/TUI tool for inspecting and installing agent skill files
version: 1.0
date_created: 2026-06-08
last_updated: 2026-06-08
owner: mparks
status: 'Planned'
tags: [feature, cli, tui, go, stdlib]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

`skill-inspector` is a single-binary, zero-runtime-dependency CLI/TUI tool written in Go. It lets security-conscious developers inspect agent skill files (`.md`) — surfacing hidden content like frontmatter, HTML comments, and suspicious Unicode characters — and install them into local agent toolchains with automatic symlink detection.

This plan is structured for a developer **new to Go** but experienced in general software development. Each phase:
- Produces a **runnable or testable artifact** (no "big bang" integrations)
- Explicitly calls out **Go language concepts** introduced for the first time
- Builds incrementally on the prior phase — earlier phases are never thrown away

The plan covers: project scaffolding → input loading → ANSI source view → hidden content analysis → raw-terminal TUI → install flow → config file → distribution scripts → CI/CD.

---

## 1. Requirements & Constraints

- **REQ-001**: Accept a single positional CLI argument: a local file path (`.md` file or directory containing `SKILL.md`) or a direct HTTP/HTTPS URL to a raw `.md` file.
- **REQ-002**: TUI must have two views toggled with `Tab`: Source view (ANSI-colorized raw file) and Hidden Content view (frontmatter, HTML comments, suspicious characters).
- **REQ-003**: Keyboard controls: `Tab` toggle, `j`/`↓` scroll down, `k`/`↑` scroll up, `i` install, `q`/`Ctrl+C` quit.
- **REQ-004**: Source view colorization rules: headers bold+cyan, code blocks dim/grey, HTML comments yellow, frontmatter blocks magenta, bold/italic markers subtly highlighted.
- **REQ-005**: Hidden Content view sections: Frontmatter (raw YAML + line numbers), HTML Comments (each with line range), Suspicious Characters (zero-width, homoglyphs, unusual whitespace) — each section shows `✓ None found` if empty.
- **REQ-006**: Suspicious character report format: `[U+200B ZERO WIDTH SPACE — line 14, col 7]`.
- **REQ-007**: Install flow: derive skill name, confirm prompt, copy files to `~/.agents/skills/<skill-name>/`, symlink to each detected/configured agent directory.
- **REQ-008**: Hardcoded default agent directories: `claude → ~/.claude/skills/`, `goose → ~/.config/goose/skills/`, `pi → ~/.pi/skills/`.
- **REQ-009**: User config at `~/.config/skill-inspector/config` (one path per line, `#` comments) **replaces** hardcoded defaults entirely.
- **REQ-010**: URL fetch errors (non-200, network failure) must print a human-readable error and exit with non-zero code.
- **REQ-011**: Downloaded URL content held in memory only — never written to disk unless user confirms install.
- **REQ-012**: Single-binary, runs on macOS and Linux, no runtime dependencies.
- **REQ-013**: Install script `install.sh` installs to `~/.local/bin/` via `curl | sh`.
- **REQ-014**: GitHub Actions workflow builds release artifacts for `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64` on tag push.
- **CON-001**: Go stdlib only — no third-party modules. `go.sum` must be empty or absent unless `golang.org/x/term` exception is adopted (see TASK-031 decision point).
- **CON-002**: Go 1.21+ minimum version.
- **CON-003**: No Windows support in v1.
- **CON-004**: No subcommands — all functionality lives inside the TUI after file load.
- **GUD-001**: Each phase must leave the binary in a buildable, runnable state (`go build -o skill-inspector .` succeeds).
- **GUD-002**: No global mutable state — pass data explicitly through function arguments and return values.
- **GUD-003**: All errors must be handled explicitly — never use `_` to discard errors from I/O operations.
- **PAT-001**: Internal packages under `internal/` — not importable by external code, enforced by Go toolchain.

---

## 2. Implementation Steps

---

### Implementation Phase 1 — Project Scaffolding & CLI Entry Point

- **GOAL-001**: Establish the Go module, directory layout, and a working `main.go` that validates the CLI argument and prints it back. After this phase, `go build` succeeds and `./skill-inspector ./some-file.md` prints the resolved argument.

> **Go Concepts Introduced in Phase 1:**
> - **Modules & `go.mod`**: Go's dependency manifest. `go mod init <module-path>` creates it. The module path is also the import path prefix for all your packages.
> - **`package main` + `func main()`**: Every executable Go program has exactly one `package main` with a `func main()` entry point.
> - **`os.Args`**: A `[]string` (slice of strings) holding command-line arguments. `os.Args[0]` is the binary name; `os.Args[1]` is the first user argument.
> - **`fmt.Fprintf(os.Stderr, ...)`**: Writing to stderr for error messages, as opposed to `fmt.Println` which writes to stdout.
> - **`os.Exit(code)`**: Terminate immediately with a specific exit code. `os.Exit(1)` signals error to the shell.
> - **Multi-file packages**: Each `.go` file in the same directory belongs to the same package and can freely call each other's exported and unexported identifiers.

| Task     | Description | Completed | Date |
| -------- | ----------- | --------- | ---- |
| TASK-001 | Run `go mod init github.com/<your-username>/skill-inspector` in the project root. This creates `go.mod` with module path and Go version. Verify the file exists and contains `module github.com/<your-username>/skill-inspector` and `go 1.21`. | | |
| TASK-002 | Create the top-level directory structure: `internal/loader/`, `internal/parser/`, `internal/colorize/`, `internal/tui/`, `internal/installer/`. The `internal/` prefix is a Go convention that prevents external packages from importing your private packages — the Go toolchain enforces this. Create a `.gitkeep` placeholder in each. | | |
| TASK-003 | Create `main.go` in the project root with `package main`. Implement `func main()` that: (1) checks `len(os.Args) != 2`, prints usage `"Usage: skill-inspector <url-or-file-path>"` to stderr and calls `os.Exit(1)`; (2) stores `os.Args[1]` in a variable named `input`; (3) prints `"Loading: " + input` to stdout. Import `"fmt"` and `"os"`. | | |
| TASK-004 | Run `go build -o skill-inspector .` and verify it succeeds. Run `./skill-inspector` (no args) and confirm it prints the usage message and exits with code 1. Run `./skill-inspector ./PRD.md` and confirm it prints `Loading: ./PRD.md`. | | |
| TASK-005 | Create `internal/loader/loader.go` with `package loader`. Define a struct `SkillFile` with fields: `Content string` (the raw file text), `SourcePath string` (original argument), `SkillName string` (derived name, populated later), `IsURL bool` (true if input was HTTP/HTTPS). Export all fields (capitalize first letter — Go's visibility rule). Add a stub function `func Load(input string) (*SkillFile, error) { return nil, nil }`. Confirm `go build` still passes. | | |

---

### Implementation Phase 2 — Input Loading: Local Files & URLs

- **GOAL-002**: Implement `loader.Load()` so it can read a local `.md` file, resolve a directory to its `SKILL.md`, and fetch a remote URL. After this phase, `main.go` can call `Load()` and print the first 5 lines of any loaded skill file.

> **Go Concepts Introduced in Phase 2:**
> - **Error handling idiom**: Go functions return `(value, error)` pairs. The caller checks `if err != nil { ... }` — there are no exceptions. This is intentional and central to Go's design.
> - **`strings.HasPrefix()`**: Checking string prefixes for URL detection (`"http://"`, `"https://"`).
> - **`os.Stat()` and `os.ReadFile()`**: `os.Stat()` returns file info (or an error if the path doesn't exist). `os.ReadFile()` reads an entire file into a `[]byte` (byte slice).
> - **`filepath.Join()`**: Platform-correct path joining. Use this instead of `+` for paths.
> - **`net/http.Get()`**: Makes an HTTP GET request. Returns a `*http.Response` and an `error`.
> - **`defer resp.Body.Close()`**: `defer` schedules a function call to run when the surrounding function returns — critical for closing HTTP response bodies and file handles to prevent resource leaks.
> - **`io.ReadAll()`**: Reads an entire `io.Reader` (like `resp.Body`) into a `[]byte`.
> - **`fmt.Errorf("context: %w", err)`**: Creates a new error that wraps an existing one with additional context. The `%w` verb enables error unwrapping with `errors.Is()` / `errors.As()`.

| Task     | Description | Completed | Date |
| -------- | ----------- | --------- | ---- |
| TASK-006 | In `internal/loader/loader.go`, implement a private helper `func isURL(s string) bool` that returns `strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")`. Import `"strings"`. | | |
| TASK-007 | Implement a private helper `func loadFromFile(path string) (*SkillFile, error)`. Use `os.Stat(path)` to check if path exists. If it's a directory, use `filepath.Join(path, "SKILL.md")` as the actual file path. Read the resolved file with `os.ReadFile()`. Convert the `[]byte` result to `string` with `string(data)`. Derive `SkillName`: if directory input, use `filepath.Base(dirPath)`; if file input, use `strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))`. Return a populated `*SkillFile`. | | |
| TASK-008 | Implement a private helper `func loadFromURL(rawURL string) (*SkillFile, error)`. Call `http.Get(rawURL)`. Check `err != nil` and return a wrapped error. Check `resp.StatusCode != 200` and return `fmt.Errorf("unexpected status %d for URL %s", resp.StatusCode, rawURL)`. Defer `resp.Body.Close()`. Call `io.ReadAll(resp.Body)` to get the content bytes. Derive `SkillName` from the last path segment of the URL using `path.Base(rawURL)` (import `"path"`, not `"path/filepath"`). Strip the `.md` extension. | | |
| TASK-009 | Implement `func Load(input string) (*SkillFile, error)` to call `loadFromURL` if `isURL(input)` is true, otherwise call `loadFromFile`. This is the only exported function in the package. | | |
| TASK-010 | Update `main.go`: call `loader.Load(input)`, handle the error (print to stderr + `os.Exit(1)`), then print the first 5 lines of `sf.Content` using `strings.Split(sf.Content, "\n")` and a `for` loop. Build and test with a local `.md` file, a local directory path, and a valid raw GitHub URL. Confirm correct output in each case. | | |

---

### Implementation Phase 3 — Parser: Frontmatter, HTML Comments, Suspicious Characters

- **GOAL-003**: Implement `internal/parser` so it can extract all hidden content from a loaded `SkillFile`. After this phase, `main.go` can print a structured summary of all findings to stdout.

> **Go Concepts Introduced in Phase 3:**
> - **Structs with methods**: Define data types with `type Foo struct { ... }` and attach behavior with `func (f Foo) MethodName() ...`. The receiver `(f Foo)` is like `self`/`this` in other languages.
> - **Slices**: `[]string`, `[]Finding` etc. — dynamically-sized arrays. Append with `append(slice, item)`. Iterate with `for i, v := range slice { ... }`.
> - **`bufio.Scanner`**: Efficiently reads text line by line. `scanner.Scan()` advances to the next line; `scanner.Text()` returns it. Idiomatic for processing files line-by-line.
> - **`unicode/utf8` and `unicode` packages**: `utf8.DecodeRuneInString()` decodes one Unicode code point (rune) from a string. A `rune` is Go's alias for `int32`, representing a Unicode code point — important because Go strings are UTF-8 byte sequences, not arrays of characters.
> - **`strings.Builder`**: An efficient way to construct strings via repeated `WriteString()` calls — avoids creating many intermediate string allocations.

| Task     | Description | Completed | Date |
| -------- | ----------- | --------- | ---- |
| TASK-011 | Create `internal/parser/parser.go` with `package parser`. Define the following exported structs: `Frontmatter { Lines []string; StartLine int; EndLine int }`, `HTMLComment { Raw string; StartLine int; EndLine int }`, `SuspiciousChar { Rune rune; Name string; Line int; Col int }`, `ParseResult { Frontmatter *Frontmatter; HTMLComments []HTMLComment; SuspiciousChars []SuspiciousChar }`. | | |
| TASK-012 | Implement `func extractFrontmatter(lines []string) *Frontmatter`. Logic: if `lines[0]` equals `"---"`, scan forward for a second `"---"`. If found, return a `Frontmatter` with the lines between the delimiters (exclusive), `StartLine: 1`, `EndLine: closingIndex + 1` (1-indexed for display). If opening `"---"` is absent or no closing delimiter is found, return `nil`. | | |
| TASK-013 | Implement `func extractHTMLComments(lines []string) []HTMLComment`. Strategy: join lines into a single string with newlines preserved (track cumulative line offsets), then scan for `<!--` and `-->` pairs. For each found pair, record the matched raw text and the start/end line numbers. Return the slice of `HTMLComment` values. Handle multi-line comments correctly by scanning byte offsets and mapping back to line numbers. | | |
| TASK-014 | Define the suspicious character table. Create a package-level `var suspiciousRunes = map[rune]string{ '\u200B': "ZERO WIDTH SPACE", '\u200C': "ZERO WIDTH NON-JOINER", '\u200D': "ZERO WIDTH JOINER", '\uFEFF': "ZERO WIDTH NO-BREAK SPACE (BOM)", '\u00AD': "SOFT HYPHEN", '\u00A0': "NO-BREAK SPACE", '\u2000': "EN QUAD", '\u2001': "EM QUAD", '\u2002': "EN SPACE", '\u2003': "EM SPACE", '\u200A': "HAIR SPACE", '\u3000': "IDEOGRAPHIC SPACE", '\u2060': "WORD JOINER", '\u2061': "FUNCTION APPLICATION", '\u2062': "INVISIBLE TIMES", '\u2063': "INVISIBLE SEPARATOR", }`. | | |
| TASK-015 | Implement `func extractSuspiciousChars(lines []string) []SuspiciousChar`. For each line (1-indexed), iterate over the runes using `for col, r := range line { ... }` (note: `col` here is the **byte** offset, not character position — document this). Look up each rune in `suspiciousRunes`. If found, append a `SuspiciousChar` with the rune, its name, the 1-indexed line number, and the byte column. Also flag runes where `r > 0x024F && r < 0x2000` that are not common punctuation — these may be homoglyphs in the Latin Extended Additional and other ranges. | | |
| TASK-016 | Implement the exported `func Parse(content string) *ParseResult`. Split content into lines with `strings.Split(content, "\n")`. Call all three extractors. Return a populated `*ParseResult`. | | |
| TASK-017 | Update `main.go` to call `parser.Parse(sf.Content)` and print the results: number of frontmatter lines found (or "none"), number of HTML comments, and each suspicious character finding formatted as `[U+XXXX NAME — line N, col M]` using `fmt.Sprintf("U+%04X", r)`. Build and test against a file that contains frontmatter, HTML comments, and zero-width spaces. | | |

---

### Implementation Phase 4 — ANSI Colorizer

- **GOAL-004**: Implement `internal/colorize` so that every line of a skill file can be returned with appropriate ANSI escape codes applied. After this phase, `main.go` can print the entire colorized source to stdout.

> **Go Concepts Introduced in Phase 4:**
> - **Package-level constants with `const`**: Define ANSI escape codes as named constants (`const Reset = "\033[0m"`). This is cleaner than magic strings.
> - **`iota`** (optional): Auto-incrementing integer for `const` blocks — commonly used for enumerations.
> - **State machines in Go**: A simple `bool` flag (e.g., `inCodeBlock bool`) maintained across a loop iteration is idiomatic Go for stateful line-by-line processing.
> - **`strings.HasPrefix()` / `strings.Contains()`**: Key string inspection functions used throughout colorization logic.
> - **Return multiple values**: Go functions can return multiple values — e.g., `func colorize(line string, state *State) (string, *State)`. This is idiomatic; use it for stateful transforms.

| Task     | Description | Completed | Date |
| -------- | ----------- | --------- | ---- |
| TASK-018 | Create `internal/colorize/colorize.go` with `package colorize`. Define ANSI constants: `Reset = "\033[0m"`, `Bold = "\033[1m"`, `Dim = "\033[2m"`, `Cyan = "\033[36m"`, `Yellow = "\033[33m"`, `Magenta = "\033[35m"`, `BoldCyan = "\033[1;36m"`. These are standard ANSI/VT100 escape codes supported by all modern Unix terminals. | | |
| TASK-019 | Define `type LineState struct { InFrontmatter bool; InCodeBlock bool; FrontmatterClosed bool }`. This carries state across lines since a code block or frontmatter region spans multiple lines. Implement `func ColorizeLines(lines []string) []string`. Initialize a `LineState`. For each line, call a `func colorizeLine(line string, state *LineState) string` function that: (1) if line is `"---"` and frontmatter not yet closed, toggle frontmatter state and return magenta-colored line; (2) if line starts with backtick-backtick-backtick, toggle code block state and return dim line; (3) if `state.InFrontmatter`, return magenta line; (4) if `state.InCodeBlock`, return dim line; (5) if line starts with `#`, return BoldCyan line; (6) if line contains `<!--`, return yellow line; (7) otherwise return the line unchanged. Return the slice of colorized lines. | | |
| TASK-020 | Update `main.go` to call `colorize.ColorizeLines(strings.Split(sf.Content, "\n"))` and print all lines. Visually verify colorization is correct by running against a `.md` file with headers, code blocks, and frontmatter. Confirm no escape codes leak across line boundaries (each colorized line must end with `Reset`). | | |

---

### Implementation Phase 5 — Raw Terminal TUI

- **GOAL-005**: Implement `internal/tui` with a full keyboard-driven pager loop. After this phase, the binary is functionally complete as a TUI — two views, scrolling, tab toggle, quit. Install is stubbed to print "Install not yet implemented."

> **DECISION POINT — TASK-031: `syscall` vs. `golang.org/x/term`**
>
> The TUI requires the terminal to be put into **raw mode** so keystrokes are delivered one character at a time without waiting for Enter, and without the terminal echoing them. There are two implementation paths:
>
> **Path A — `golang.org/x/term` (RECOMMENDED)**
> - `golang.org/x/term` is a package from the official Go extended standard library (`golang.org/x`), maintained by the Go core team at Google.
> - It has **zero transitive dependencies** — `go get golang.org/x/term` adds one line to `go.mod` and one to `go.sum`. No vendor tree required.
> - API: `term.MakeRaw(int(os.Stdin.Fd()))` enters raw mode; the returned `oldState` is passed to `term.Restore()` to exit it. `term.GetSize(fd)` returns terminal dimensions.
> - This handles the platform differences in `termios` structures between Linux and macOS transparently.
> - **Recommendation: Use this path.** The PRD explicitly sanctions it. The complexity savings over Path B are significant.
>
> **Path B — Pure `syscall` (Advanced, fragile)**
> - Requires `syscall.Syscall(syscall.SYS_IOCTL, ...)` with `syscall.TCGETS`/`syscall.TCSETS` and a manually declared `syscall.Termios` struct.
> - The `Termios` struct fields differ between Linux and macOS (different field names/types), requiring build tags: `//go:build darwin` and `//go:build linux` with separate files.
> - Raw mode requires clearing `ICANON` and `ECHO` flags manually: `termios.Lflag &^= syscall.ICANON | syscall.ECHO`.
> - This is ~100 lines of platform-specific plumbing vs. ~5 lines with `golang.org/x/term`.
> - **Only choose this path** if you specifically want to understand low-level terminal I/O or have a hard "zero external packages" requirement.
>
> The tasks below use Path A. If you choose Path B, TASK-031 will need to be replaced with platform-specific `syscall` implementation files.

> **Go Concepts Introduced in Phase 5:**
> - **`go get` and adding a module dependency**: Running `go get golang.org/x/term` updates `go.mod` and `go.sum`. The toolchain downloads and caches the module automatically.
> - **Interfaces**: `type Writer interface { Write([]byte) (int, error) }` — Go interfaces are implicit (no `implements` keyword). Any type with matching methods satisfies the interface. `os.Stdout` satisfies `io.Writer`.
> - **`os.Stdin.Read(buf)`**: Reading raw bytes from stdin. In raw mode, each keystroke produces 1–3 bytes immediately.
> - **`defer`**: Schedule cleanup (restoring terminal state) to run on function exit — ensures the terminal is always restored even if a panic occurs.
> - **Struct embedding**: `type TUI struct { skill *loader.SkillFile; result *parser.ParseResult; ... }` — composing structs from multiple package types.
> - **Named return values** (optional but useful for render functions): `func (t *TUI) renderSource() []string { ... }`.

| Task     | Description | Completed | Date |
| -------- | ----------- | --------- | ---- |
| TASK-021 | Run `go get golang.org/x/term` (Path A). Verify `go.mod` now contains `require golang.org/x/term v0.x.x` and `go.sum` has the corresponding hash entries. **Path B users**: skip this task and instead create `internal/tui/terminal_linux.go` and `internal/tui/terminal_darwin.go` with build-tag-guarded `termios` implementations. | | |
| TASK-022 | Create `internal/tui/tui.go` with `package tui`. Define `type View int` with constants `ViewSource View = 0` and `ViewHidden View = 1`. Define `type TUI struct { skill *loader.SkillFile; result *parser.ParseResult; currentView View; scrollOffset int; termWidth int; termHeight int }`. Define constructor `func New(sf *loader.SkillFile, pr *parser.ParseResult) *TUI`. | | |
| TASK-023 | Implement `func (t *TUI) getTerminalSize()` using `term.GetSize(int(os.Stdout.Fd()))` (Path A) or `syscall.Syscall` with `TIOCGWINSZ` ioctl (Path B). Store results in `t.termWidth` and `t.termHeight`. This is called once at startup and also on `SIGWINCH` (terminal resize signal). | | |
| TASK-024 | Implement terminal control helpers as package-level functions in a new file `internal/tui/screen.go`: `func clearScreen()` outputs `"\033[2J\033[H"` (ANSI clear screen + move cursor home), `func hideCursor()` outputs `"\033[?25l"`, `func showCursor()` outputs `"\033[?25h"`, `func moveCursor(row, col int)` outputs `fmt.Sprintf("\033[%d;%dH", row, col)`. Each function writes directly to `os.Stdout`. | | |
| TASK-025 | Implement `func (t *TUI) renderSourceView() []string`. This returns the colorized lines of the skill file (call `colorize.ColorizeLines`). The returned slice is the full virtual document; scrolling is handled by slicing it: `lines[t.scrollOffset : t.scrollOffset + t.termHeight - 2]`. Clamp offsets to valid range. | | |
| TASK-026 | Implement `func (t *TUI) renderHiddenView() []string`. Build a `[]string` result by appending labeled sections: `"═══ FRONTMATTER ═══"` header, then frontmatter lines with 1-indexed line numbers prefixed (or `"  ✓ None found"`); `"═══ HTML COMMENTS ═══"` header with each comment and its line range; `"═══ SUSPICIOUS CHARACTERS ═══"` header with each finding in `[U+XXXX NAME — line N, col M]` format. Use magenta ANSI codes for section headers. | | |
| TASK-027 | Implement `func (t *TUI) renderStatusBar() string`. Returns a single-line string showing: current view name (`[SOURCE]` or `[HIDDEN]`), scroll position, and key hints (`Tab:toggle  j/k:scroll  i:install  q:quit`). Apply inverse-video ANSI code (`"\033[7m"`) to make it visually distinct. Pad to terminal width. | | |
| TASK-028 | Implement `func (t *TUI) draw()`. Clear the screen. Get the appropriate lines slice (source or hidden) based on `t.currentView`. Print `t.termHeight - 1` lines (filling short views with empty lines). Print the status bar on the last terminal row using `moveCursor(t.termHeight, 1)`. Flush all output. | | |
| TASK-029 | Implement `func (t *TUI) handleKey(buf []byte) bool`. `buf` is a 3-byte array read from stdin in raw mode. Return `true` to signal quit. Handle: `'q'` or `0x03` (Ctrl+C) returns true; `'\t'` (Tab, `0x09`) toggles `t.currentView` and resets `t.scrollOffset = 0`; `'j'` or `0x1B 0x5B 0x42` (down arrow escape sequence) increments `t.scrollOffset`; `'k'` or `0x1B 0x5B 0x41` (up arrow) decrements `t.scrollOffset` clamped to 0; `'i'` calls `t.installPrompt()` (stub: print `"\r\nInstall not yet implemented.\r\n"`). | | |
| TASK-030 | Implement `func (t *TUI) Run() error`. Call `term.MakeRaw(int(os.Stdin.Fd()))` to enter raw mode. Immediately `defer term.Restore(int(os.Stdin.Fd()), oldState)`. Call `hideCursor()` and `defer showCursor()`. Call `t.getTerminalSize()`. Call `t.draw()`. Enter a `for` loop: read up to 3 bytes from `os.Stdin` into `buf [3]byte`, call `t.handleKey(buf[:n])`, redraw with `t.draw()` on each iteration. Return `nil` on quit. | | |
| TASK-031 | Update `main.go` to call `tui.New(sf, parser.Parse(sf.Content)).Run()`. Handle the returned error. Remove all debug `fmt.Println` calls from earlier phases. Build and test the full TUI: verify both views render, scrolling works, Tab toggles correctly, and the terminal is restored cleanly on quit. | | |

---

### Implementation Phase 6 — Install Flow

- **GOAL-006**: Implement `internal/installer` with the full install logic: config file reading, agent directory probing, file copying, and symlink creation. After this phase, pressing `i` in the TUI triggers the complete install flow.

> **Go Concepts Introduced in Phase 6:**
> - **`os/user` package**: `user.Current()` returns the current user including `HomeDir` — use this instead of hardcoding `~` (which the OS does not expand in Go).
> - **`~` expansion via `os.UserHomeDir()`**: `os.UserHomeDir()` returns the home directory. Manual tilde expansion: `strings.Replace(path, "~", homeDir, 1)`.
> - **`os.MkdirAll(path, perm)`**: Creates a directory and all necessary parents. Idempotent — no error if it already exists. Permission `0755` is standard for directories.
> - **`os.Symlink(oldname, newname)`**: Creates a symbolic link. `oldname` is the target (the actual file/dir); `newname` is the link path.
> - **`bufio.NewScanner(file)`**: Scans a file line by line. Used for config file reading.
> - **`os.WriteFile(path, data, perm)`**: Writes a `[]byte` to a file, creating or truncating it. Permission `0644` is standard for files.
> - **`io.Copy(dst, src)`**: Streams data from a `Reader` to a `Writer` — efficient for file copying without loading entirely into memory.
> - **Cooked mode for confirmation prompt**: The install `[y/N]` prompt needs normal line-buffered input. Restore the terminal before reading, then re-enter raw mode after.

| Task     | Description | Completed | Date |
| -------- | ----------- | --------- | ---- |
| TASK-032 | Create `internal/installer/config.go` with `package installer`. Define `type AgentDir struct { Name string; Path string }`. Implement `func DefaultAgentDirs() []AgentDir` returning the three hardcoded defaults with `~` expanded via `os.UserHomeDir()`: `{Name: "claude", Path: "~/.claude/skills/"}`, `{Name: "goose", Path: "~/.config/goose/skills/"}`, `{Name: "pi", Path: "~/.pi/skills/"}`. | | |
| TASK-033 | In `internal/installer/config.go`, implement `func LoadConfig() ([]AgentDir, error)`. Compute config path using `os.UserConfigDir()` (returns `~/.config` on Linux/macOS) joined with `"skill-inspector/config"`. If the file does not exist (`os.IsNotExist(err)`), return `DefaultAgentDirs(), nil`. If it exists, open it and scan line by line with `bufio.Scanner`. Skip blank lines and lines starting with `#`. For each remaining line, expand `~` and create an `AgentDir` with `Name` derived from the second-to-last path segment (e.g., `~/.my-agent/skills/` produces name `"my-agent"`). Return the parsed slice. | | |
| TASK-034 | Create `internal/installer/installer.go`. Define `type InstallResult struct { Agent string; Linked bool; AlreadyLinked bool; SkippedReason string }`. Implement `func Install(sf *loader.SkillFile, agentDirs []AgentDir) ([]InstallResult, error)`. Step 1: compute `destDir = filepath.Join(homeDir, ".agents", "skills", sf.SkillName)`. Step 2: `os.MkdirAll(destDir, 0755)`. Step 3: if `sf.IsURL`, write `sf.Content` to `filepath.Join(destDir, "SKILL.md")` via `os.WriteFile`. If local input, copy files from the source directory or single `.md` file using `os.ReadDir()` and `io.Copy()`. Step 4: iterate `agentDirs`; for each, check if its `Path` exists via `os.Stat()`; if yes, call `os.Symlink(destDir, filepath.Join(agentPath, sf.SkillName))`; if `os.IsExist(err)`, set `AlreadyLinked: true`. Return the results slice. | | |
| TASK-035 | Implement the `installPrompt()` method on `*tui.TUI`. Restore terminal to cooked mode with `term.Restore`, print `Install "<name>" to ~/.agents/skills/<name>/? [y/N]: `, read one line from `bufio.NewReader(os.Stdin)`, trim and lowercase the input. If `"y"`, call `installer.Install()`, print result summary with `"✓ Installed to ..."`, `"✓ Linked: <agent>"`, `"~ Already linked: <agent>"`, `"— Skipped: <agent> (directory not found)"`. Wait for any keypress, then re-enter raw mode. If not `"y"`, re-enter raw mode immediately. | | |
| TASK-036 | Test the full install flow end-to-end: run against a local skill directory, press `i`, confirm with `y`, verify `~/.agents/skills/<name>/SKILL.md` exists, verify symlinks are created for any existing agent directories, and the summary is printed correctly. Test `n` confirms no files are written. | | |

---

### Implementation Phase 7 — Distribution: `install.sh` & GitHub Actions

- **GOAL-007**: Produce a production-ready `install.sh` installer script and a GitHub Actions workflow that builds and publishes cross-platform release binaries on every tag push.

> **Go Concepts Introduced in Phase 7:**
> - **Cross-compilation**: Go natively supports cross-compilation via environment variables. `GOOS=linux GOARCH=amd64 go build -o skill-inspector-linux-amd64 .` produces a Linux binary from any host — no cross-compiler toolchain required. This is one of Go's major advantages.
> - **Build flags**: `-ldflags "-s -w"` strips debug symbols and DWARF tables, reducing binary size by ~30%. `-trimpath` removes local file system paths from the binary for reproducibility.
> - **`go vet`**: The built-in static analyzer. Run `go vet ./...` before releasing — catches common mistakes the compiler misses.

| Task     | Description | Completed | Date |
| -------- | ----------- | --------- | ---- |
| TASK-037 | Create `install.sh` in the project root. The script must: (1) use `#!/bin/sh` (POSIX sh, not bash); (2) detect OS with `OS=$(uname -s | tr '[:upper:]' '[:lower:]')` mapping `darwin` and `linux`, exiting with error for anything else; (3) detect arch with `ARCH=$(uname -m)` mapping `x86_64` to `amd64`, `arm64`/`aarch64` to `arm64`; (4) construct asset name `skill-inspector-${OS}-${ARCH}`; (5) fetch latest release tag from `https://api.github.com/repos/<user>/skill-inspector/releases/latest` via `curl` + `grep` + `cut`; (6) download binary to `/tmp/skill-inspector`; (7) `chmod +x /tmp/skill-inspector`; (8) `mkdir -p "$HOME/.local/bin"` and move binary there; (9) check if `$HOME/.local/bin` is in `$PATH` and print a warning if not; (10) print `skill-inspector installed to ~/.local/bin/skill-inspector`. | | |
| TASK-038 | Create `.github/workflows/release.yml`. Trigger: `on: push: tags: ['v*.*.*']`. Runner: `ubuntu-latest`. Steps: `actions/checkout@v4`; `actions/setup-go@v5` with `go-version: '1.21'`; run `go vet ./...`; matrix build for four targets using `env: GOOS/GOARCH` variables and `go build -ldflags "-s -w" -trimpath -o skill-inspector-$GOOS-$GOARCH .`; upload all four artifacts as release assets using `softprops/action-gh-release@v2` with `files: skill-inspector-*`. Set `permissions: contents: write` on the job. | | |
| TASK-039 | Add a `Makefile` to the project root. Targets: `build` (local arch), `build-all` (all four cross-compiled targets with `-ldflags "-s -w" -trimpath`), `clean` (remove `skill-inspector-*` binaries), `vet` (`go vet ./...`), `fmt` (`gofmt -w .`). This is optional convenience infrastructure. | | |
| TASK-040 | Write `README.md` with: one-liner description, install section showing the `curl | sh` command, usage examples from the PRD, keyboard controls table, config file format and location, agent directory defaults table, note about the `golang.org/x/term` dependency and rationale, and build-from-source instructions (`go build -o skill-inspector .`). | | |

---

### Implementation Phase 8 — Polish & Edge Cases

- **GOAL-008**: Harden the implementation against real-world edge cases discovered during end-to-end testing.

> **Go Concepts Introduced in Phase 8:**
> - **`os/signal` package**: `signal.Notify(ch, syscall.SIGWINCH)` registers a channel to receive OS signals. Use a goroutine to listen for terminal resize.
> - **Goroutines and channels**: `go func() { ... }()` launches a concurrent goroutine. `ch := make(chan os.Signal, 1)` creates a buffered channel. Keep usage simple here — one goroutine for signal handling.
> - **`context` package**: For URL fetch timeouts: `http.NewRequestWithContext(ctx, "GET", url, nil)` with `context.WithTimeout(context.Background(), 10*time.Second)`.

| Task     | Description | Completed | Date |
| -------- | ----------- | --------- | ---- |
| TASK-041 | Handle terminal resize (`SIGWINCH`): in `tui.Run()`, start a goroutine with `signal.Notify(sigCh, syscall.SIGWINCH)` that signals the main render loop via a boolean channel when a resize occurs. On receipt, call `t.getTerminalSize()` and redraw. This prevents garbled output when the user resizes their terminal window. | | |
| TASK-042 | Add a 10-second timeout to URL fetches in `internal/loader/loader.go`. Create a custom `http.Client` with `Timeout: 10 * time.Second` and use it instead of the default `http.Get()`. This prevents indefinite hangs on slow or unresponsive URLs. | | |
| TASK-043 | Handle the `os.IsExist` error case in TASK-034's symlink step: record the result as `AlreadyLinked: true` and display as `~ Already linked: claude (~/.claude/skills/my-skill)` in the install summary. | | |
| TASK-044 | Clamp `t.scrollOffset` correctly in `handleKey`: compute `maxScroll = max(0, len(visibleLines) - (t.termHeight - 2))` and enforce `t.scrollOffset` stays in `[0, maxScroll]` after every change. This prevents scrolling past the end of a document. | | |
| TASK-045 | Run `gofmt -d .` and confirm no output (all files are correctly formatted). Run `go vet ./...` and resolve any reported issues. These are effectively required for any Go code shared publicly. | | |
| TASK-046 | Confirm the `golang.org/x/term` decision is reflected accurately in `go.mod`, `go.sum`, and the README — including the rationale (zero transitive deps, Go team-maintained, PRD-sanctioned). Security-conscious users who build from source should understand this dependency clearly. | | |

---

## 3. Alternatives

- **ALT-001: Use a TUI framework (Bubble Tea / tview)** — Rejected. These are third-party dependencies that violate CON-001. Bubble Tea is excellent but adds ~15 transitive dependencies. The PRD is explicit about stdlib-only.
- **ALT-002: Pure `syscall` for raw terminal mode (Path B)** — Not chosen as the default because it requires platform-specific build-tagged files for Linux vs. macOS `termios` struct differences, ~100 lines of plumbing, and more fragile behavior across terminal emulators. `golang.org/x/term` is the sanctioned exception per the PRD.
- **ALT-003: `flag` package for argument parsing** — Unnecessary for a single positional argument. `os.Args[1]` is sufficient and avoids `flag.Parse()` behavior changes (e.g., treating `-`-prefixed strings as flags, adding `--help` automatically).
- **ALT-004: Render Markdown to terminal** — Out of scope per PRD Non-Goals. Rendered Markdown would obscure the raw syntax that is the purpose of the tool.
- **ALT-005: Single flat `main.go` file** — Rejected in favor of `internal/` packages. Even for a small tool, separating loader, parser, colorize, tui, and installer makes each component independently testable and the codebase navigable.
- **ALT-006: Write downloaded URL content to a temp file** — Rejected per REQ-011. Holding URL content in memory is a deliberate security property — no disk artifact unless the user explicitly installs.
- **ALT-007: Use `go:embed` to bundle install.sh** — Unnecessary. `install.sh` is a standalone shell script hosted directly on GitHub and fetched via `curl`; embedding it in the binary adds no value.

---

## 4. Dependencies

- **DEP-001**: `golang.org/x/term` — Go extended stdlib, maintained by the Go core team. Used for raw terminal mode (`term.MakeRaw`, `term.Restore`, `term.GetSize`). Zero transitive dependencies. Added via `go get golang.org/x/term`.
- **DEP-002**: Go 1.21+ toolchain — Required for `os.UserConfigDir()`, `os.UserHomeDir()` reliability, and forward compatibility. Available at `https://go.dev/dl/`.
- **DEP-003**: GitHub Actions `actions/checkout@v4` — CI workflow dependency for source checkout.
- **DEP-004**: GitHub Actions `actions/setup-go@v5` — CI workflow dependency for Go toolchain setup.
- **DEP-005**: GitHub Actions `softprops/action-gh-release@v2` — CI workflow dependency for publishing release assets to GitHub Releases.

---

## 5. Files

- **FILE-001**: `go.mod` — Module definition. Module path: `github.com/<user>/skill-inspector`. Min Go version: `1.21`. One `require` entry for `golang.org/x/term`.
- **FILE-002**: `go.sum` — Cryptographic hashes for module dependencies. Auto-managed by `go get`. Contains entries only for `golang.org/x/term`.
- **FILE-003**: `main.go` — Entry point. Parses `os.Args`, calls `loader.Load()`, calls `parser.Parse()`, launches `tui.New(...).Run()`.
- **FILE-004**: `internal/loader/loader.go` — `SkillFile` struct. `Load()` exported function. `loadFromFile()`, `loadFromURL()`, `isURL()` private helpers.
- **FILE-005**: `internal/parser/parser.go` — `ParseResult`, `Frontmatter`, `HTMLComment`, `SuspiciousChar` structs. `Parse()` exported function. `extractFrontmatter()`, `extractHTMLComments()`, `extractSuspiciousChars()` private helpers. `suspiciousRunes` package-level map.
- **FILE-006**: `internal/colorize/colorize.go` — ANSI escape code constants. `LineState` struct. `ColorizeLines()` exported function. `colorizeLine()` private helper.
- **FILE-007**: `internal/tui/tui.go` — `View` type and constants. `TUI` struct. `New()` constructor. `Run()` main loop. `handleKey()`, `draw()`, `installPrompt()`, `renderSourceView()`, `renderHiddenView()`, `renderStatusBar()`, `getTerminalSize()` methods.
- **FILE-008**: `internal/tui/screen.go` — Terminal control helpers: `clearScreen()`, `hideCursor()`, `showCursor()`, `moveCursor()`.
- **FILE-009**: `internal/installer/config.go` — `AgentDir` struct. `DefaultAgentDirs()`. `LoadConfig()`.
- **FILE-010**: `internal/installer/installer.go` — `InstallResult` struct. `Install()` exported function.
- **FILE-011**: `install.sh` — POSIX shell end-user installer script.
- **FILE-012**: `.github/workflows/release.yml` — GitHub Actions release workflow for cross-platform binary builds.
- **FILE-013**: `Makefile` — Local developer convenience targets.
- **FILE-014**: `README.md` — User and developer documentation.

---

## 6. Testing

> Go has a built-in test runner. Test files are named `*_test.go` and live alongside the code they test. Run all tests with `go test ./...`. No external test framework is needed.

- **TEST-001**: `internal/parser` — `TestExtractFrontmatter`: test with valid frontmatter, no frontmatter, and an unclosed opening `---`. Assert `ParseResult.Frontmatter` fields are correct in each case.
- **TEST-002**: `internal/parser` — `TestExtractHTMLComments`: test with a single-line comment, a multi-line comment, and multiple comments. Assert `StartLine`/`EndLine` values are correct.
- **TEST-003**: `internal/parser` — `TestExtractSuspiciousChars`: construct a string containing known zero-width characters (`\u200B`, `\uFEFF`) at known line and byte positions. Assert each `SuspiciousChar` finding has correct `Rune`, `Name`, `Line`, and `Col`.
- **TEST-004**: `internal/colorize` — `TestColorizeLines`: assert headers return lines containing the `BoldCyan` constant, frontmatter lines contain `Magenta`, code block lines contain `Dim`, and plain lines pass through unchanged. Use `strings.Contains()`.
- **TEST-005**: `internal/loader` — `TestLoadFromFile`: create a temp file with `os.CreateTemp`, write known content, call `loadFromFile`, assert `Content` and `SkillName` match. Use `defer os.Remove(tmpFile.Name())` for cleanup.
- **TEST-006**: `internal/loader` — `TestLoadFromURL`: use `net/http/httptest.NewServer()` (stdlib) to spin up a local mock HTTP server returning known content. Call `Load("http://127.0.0.1:<port>/SKILL.md")`. Assert correct content. Also test non-200 response returns a non-nil error.
- **TEST-007**: `internal/installer` — `TestLoadConfig`: write a temp config file with known paths and `#` comments, call a modified `LoadConfig` that accepts a path override, assert returned `[]AgentDir` matches expected names and expanded paths.
- **TEST-008**: `internal/installer` — `TestInstall`: use `t.TempDir()` as a stand-in for home directory. Call `Install()` with a mock `SkillFile` and empty `AgentDirs`. Assert `SKILL.md` exists in the expected destination path. Test symlink creation: create a temp agent dir, pass it in `AgentDirs`, assert `os.Lstat()` returns a symlink mode after install.
- **TEST-009**: Integration — `TestCLINoArgs`: use `os/exec` to run the built binary with no arguments. Assert exit code is `1` and stderr contains `"Usage:"`.
- **TEST-010**: Integration — `TestCLILocalFile`: run the built binary against a test fixture `.md` file with stdin redirected from `/dev/null`. Verify the binary handles the non-TTY stdin gracefully (either exits cleanly or prints a clear error about requiring an interactive terminal).

---

## 7. Risks & Assumptions

- **RISK-001**: Terminal raw mode behavior varies across terminal emulators (iTerm2, Terminal.app, GNOME Terminal, tmux, screen). `golang.org/x/term` mitigates this significantly, but edge cases may appear. Mitigation: test in macOS Terminal.app, iTerm2, and a Linux terminal before first release.
- **RISK-002**: The `golang.org/x/term` package version may receive breaking API changes in a future major release. Mitigation: the version is pinned in `go.mod`; updates are always explicit via `go get`.
- **RISK-003**: Agent directory paths for `goose` and `pi` are sourced from the PRD — they may differ from the tools' actual defaults. Mitigation: verify against official `goose` and `pi` documentation before implementing TASK-032. These are trivially changed string constants.
- **RISK-004**: `os.Symlink` may fail on filesystems that don't support symlinks (FAT32, some network mounts). Mitigation: check error in TASK-034, report as a skipped agent with the reason in the install summary.
- **RISK-005**: The GitHub API rate-limits unauthenticated requests to 60/hour. The `install.sh` script uses the API to find the latest release tag. Mitigation: document the manual install fallback (direct binary URL with explicit version) in the README.
- **RISK-006**: Cross-compiled `darwin/amd64` binary behavior under Rosetta 2 on Apple Silicon. Mitigation: test both the native `darwin/arm64` and Rosetta `darwin/amd64` binaries on Apple Silicon hardware before release.
- **ASSUMPTION-001**: The tool is run in an interactive terminal that supports ANSI escape codes. Non-TTY environments (CI, piped output) are not a supported v1 use case. No `NO_COLOR` / `TERM=dumb` detection required for v1.
- **ASSUMPTION-002**: `~/.local/bin` is the standard install target. This is standard on modern Linux (XDG Base Directory Spec) and common on macOS developer machines.
- **ASSUMPTION-003**: Skill files are UTF-8 encoded. Go strings are UTF-8 natively; no encoding detection needed for v1.
- **ASSUMPTION-004**: A skill directory passed as input always contains a file named exactly `SKILL.md` (case-sensitive), consistent with the PRD definition.

---

## 8. Related Specifications / Further Reading

- [PRD: skill-inspector](../PRD.md) — Full product requirements document
- [Go Module Reference](https://go.dev/ref/mod) — Authoritative guide to `go.mod`, `go.sum`, and module versioning
- [Effective Go](https://go.dev/doc/effective_go) — Idiomatic Go patterns; essential reading for Go newcomers
- [golang.org/x/term package docs](https://pkg.go.dev/golang.org/x/term) — API reference for `MakeRaw`, `Restore`, `GetSize`
- [ANSI Escape Code Reference](https://en.wikipedia.org/wiki/ANSI_escape_code) — Complete table of ANSI/VT100 codes used in Phases 4 and 5
- [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html) — Defines `~/.config/` and `~/.local/bin/` conventions
- [Unicode Character Database](https://www.unicode.org/charts/) — Reference for suspicious character code point ranges in Phase 3
- [net/http/httptest package docs](https://pkg.go.dev/net/http/httptest) — Stdlib mock HTTP server used in TEST-006
- [Go Blog: Error Handling and Go](https://go.dev/blog/error-handling-and-go) — Explains the `(value, error)` idiom introduced in Phase 2
- [Go Blog: The Laws of Reflection](https://go.dev/blog/laws-of-reflection) — Background on runes, bytes, and strings (relevant to Phase 3 Unicode handling)
- [GitHub Actions: Workflow syntax](https://docs.github.com/en/actions/writing-workflows/workflow-syntax-for-github-actions) — Reference for TASK-038
