# Security Policy

## Security Model

`skill-inspector` is a **local audit tool**, not a sandbox or scanner. Its security model is intentionally simple:

- **Read-only by default.** The tool reads skill files and displays them. It does not execute code, make network requests (beyond what you explicitly ask it to fetch), or modify your system unless you choose to install a skill.
- **Human-led decisions.** `skill-inspector` does not score risk, label vulnerabilities, or auto-block installs. It surfaces hidden content so you can decide.
- **No privilege escalation.** The tool runs with your user's permissions. Installation writes to `~/.agents/skills/` and creates symlinks in detected agent directories — no `sudo` required.
- **No telemetry.** No data leaves your machine.

If you find a way for a crafted skill file to cause `skill-inspector` to execute code, write to unintended paths, or exfiltrate data, that is a security vulnerability — please report it.

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

We are currently in early release. Only the latest version receives security updates.

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, report them via:

- **GitHub Security Advisories:** Use the [Private Vulnerability Reporting](https://github.com/parksjr/skill-inspector/security/advisories/new) feature on the repository.

You should receive a response within 72 hours. If the issue is confirmed, we will release a patch as soon as possible depending on complexity.

### What to Include

- A description of the vulnerability and its impact
- Steps to reproduce (a malicious skill file that demonstrates the issue is ideal)
- The version of `skill-inspector` you tested against
- Your assessment of severity

## Disclosure Policy

- Reporters will be credited in the release notes (unless you request anonymity).
- We follow coordinated disclosure: the fix is released before the vulnerability is publicly discussed.
- Critical vulnerabilities will receive a patch release within 7 days of confirmation.

## Dependency Policy

`skill-inspector` uses a minimal dependency footprint by design:

| Dependency | Purpose | Rationale |
|---|---|---|
| `golang.org/x/term` | Raw terminal mode for the TUI | Only non-stdlib dependency. Required for reading individual keystrokes (Tab, arrows, etc.) without waiting for Enter. Maintained by the Go team as part of `golang.org/x`. |
| `golang.org/x/sys` | Indirect, pulled by `x/term` | Required by `x/term` for platform-specific terminal syscalls. |

We pin specific versions in `go.mod` and will not add new dependencies without strong justification. `golang.org/x/term` was chosen as the sole external dependency because:

1. It is maintained by the Go core team under the same security standards as the standard library.
2. There is no stdlib package that provides raw terminal input.
3. The alternative — wrapping `stty`/`ncurses` — would be less portable and harder to audit than a single Go package.
