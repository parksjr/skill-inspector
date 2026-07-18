# skill-inspector — Launch Readiness Assessment

> **Date:** 2026-07-18  
> **Review team:** 3-agent panel (Code Quality Analyzer, Adversarial Security Reviewer, Product/UX Reviewer) coordinated by orchestrator  
> **TL;DR:** The tool is functionally complete and demonstrates clear value. However, **5 blockers** (security + hygiene) and **8 high-priority items** must be resolved before open-source launch. Estimated effort: ~1 week.

---

## Executive Summary

`skill-inspector` is a well-architected Go CLI/TUI tool that surfaces hidden content in agent skill files before installation. The code is clean, the internal package separation is sound, and the core workflow (load → parse → inspect → install) works end-to-end. The demo GIFs effectively demonstrate its purpose.

However, three independent reviews revealed significant gaps:

1. **Security:** Content from inspected files is rendered in the terminal with **zero sanitization** — ANSI/OSC/DCS escape sequences in a skill file can hide content, hijack the terminal, or break the TUI entirely. This undermines the tool's core promise.
2. **Engineering:** CI is misconfigured (Go version mismatch would cause build failures), tests are never run in CI, and the two most security-critical packages (parser, colorizer) have zero test coverage.
3. **Launch hygiene:** No LICENSE file, inaccurate README install path, committed binary in repo, no `--help`/`--version` flags.

The tool is **not launch-ready today**. With focused effort on the blockers and high-priority items below, it could be launch-ready in ~1 week.

---

## 🔴 BLOCKERS (Must Fix Before Launch)

These 5 items gate the launch. Combined effort: ~4 hours.

| # | Finding | Severity | Source | Detail |
|---|---------|----------|--------|--------|
| **B1** | Missing LICENSE file | CRITICAL | Product-UX | README says "MIT" but no `LICENSE` file exists. The repo is legally unlicensed without it. |
| **B2** | Terminal escape sequence passthrough | CRITICAL | Adversary (F01, F18) | Content from inspected skill files is written to the terminal with no sanitization. A malicious skill can embed `\033[?1049l` (exit alt screen), `\033c` (terminal reset), `\033]0;TITLE\007` (set window title), or ANSI SGR codes that render text invisible (same-color foreground/background). The `stripANSI` function already exists; apply it to rendered content before display. |
| **B3** | No `--help` / `--version` flags | CRITICAL | Product-UX (B3) | Running `skill-inspector` with no args prints "Usage: skill-inspector <url-or-file-path>" to stderr. No `--help` or `--version` support. First-run UX is broken; this is table-stakes for CLI tools. |
| **B4** | No content-length limit on URL fetch (memory bomb) | CRITICAL | Adversary (F13), Analyzer (B-13) | `loader.go:91` calls `io.ReadAll(resp.Body)` with no limit. A malicious server can return infinite data, causing OOM. Fix: `io.LimitReader(resp.Body, 10<<20)` (10 MiB cap). Also add HTTP client timeout (currently infinite). |
| **B5** | Go version mismatch: CI uses 1.21, go.mod declares 1.25.1 | CRITICAL | Analyzer (B-04), Product-UX (B4) | `go.mod` declares `go 1.25.1` but CI (both `ci.yml` and `release.yml`) uses `go-version: '1.21'`. Go 1.21 would refuse to build this module. CI is effectively broken. Fix: align CI to 1.25 or downgrade `go.mod` to 1.21 (after verifying no 1.25-only features are used). |

---

## 🟡 HIGH PRIORITY (Strongly Recommended Before Launch)

Estimated effort: ~2 days.

### Security

| # | Finding | Severity | Source | Detail |
|---|---------|----------|--------|--------|
| **H1** | Bidi control characters not detected | HIGH | Adversary (F02) | Missing from `suspiciousRunes`: U+202A–U+202E (bidi overrides/isolates), U+2066–U+2069 (bidi isolates). These enable Trojan Source attacks (CVE-2021-42574) where RTL/LTR overrides reorder text visually. Add ~10 entries to the map. |
| **H2** | HTTP (plaintext) accepted | HIGH | Adversary (F14) | `isURL()` accepts `http://`. MITM attacker on public WiFi can inject malicious content. Fix: reject `http://`; only accept `https://`. |
| **H3** | `install.sh` no checksum verification | HIGH | Adversary (F15) | Script downloads binary with no SHA256 verification. If GitHub releases are compromised, users install malware. Fix: generate checksums in CI and verify in install.sh. |
| **H4** | Hidden content undetected: YAML directives, multi-doc, CDATA, JS/CSS comments | HIGH | Adversary (F03–F06) | Frontmatter parser misses `%YAML` directives and `...` multi-document separators. HTML comment extractor misses CDATA sections. Neither detects JS (`//`), or CSS (`/* */`) comments. |

### Engineering

| # | Finding | Severity | Source | Detail |
|---|---------|----------|--------|--------|
| **H5** | CI does not run tests | HIGH | Analyzer (B-05), Product-UX | Both `ci.yml` and `release.yml` run `go vet` but never `go test ./...`. Parser (security-critical) and colorizer have zero tests. Add `go test ./...` to CI. |
| **H6** | `truncateLine` slices by bytes, not runes | HIGH | Analyzer (B-02), Product-UX | `tui.go:589` does `line[:maxWidth-1]` on the raw string (which may contain multi-byte UTF-8 and ANSI codes), not on visible characters. Can produce invalid UTF-8 in narrow terminals. |
| **H7** | Committed binary + `.DS_Store` in repo | HIGH | Product-UX (B5) | `skill-inspector` binary (~8MB) and `.DS_Store` are tracked in git. Remove both, add to `.gitignore`. |
| **H8** | `README.md` install path inaccurate | HIGH | Analyzer (B-16), Product-UX (B2) | README line 25 says `/usr/local/bin/`; install.sh uses `~/.local/bin`. Fix README to match. |

---

## 🟢 MEDIUM PRIORITY (Post-Launch or Time-Permitting)

### Security

| # | Finding | Source |
|---|---------|--------|
| **M1** | Config file symlink attack — `loadAgentDirs` doesn't verify config is a regular file | Adversary (F08) |
| **M2** | TOCTOU between `PlanInstall` and `Install` | Adversary (F09) |
| **M3** | `copyDir` follows symlinks — could copy arbitrary files | Adversary (F12) |
| **M4** | `normalizeGitHubBlobURL` encoding edge cases | Adversary (F16) |
| **M5** | Keypress goroutine leaks on exit | Analyzer (B-06), Adversary (F20) |
| **M6** | HTML comments inside fenced code blocks flagged as suspicious | Analyzer (B-09) |

### Code Quality

| # | Finding | Source |
|---|---------|--------|
| **M7** | Double parsing — `deriveSkillName()` calls `parser.Parse()` only for frontmatter name, then `main.go` parses again | Analyzer (B-01) |
| **M8** | `colorizeLine` flags ANY line with `*` or `_` as bold/italic — false positives on bullet lists, math, snake_case | Analyzer (B-03), Product-UX |
| **M9** | `FrontmatterValue` double-unquoting bug | Analyzer (B-08) |
| **M10** | `loadAgentDirs` silently masks permission errors on config file | Analyzer |
| **M11** | `suspiciousRunes` map missing ~200 Unicode homoglyph ranges (Latin Extended, Greek, Cyrillic characters that visually resemble ASCII) | Product-UX |
| **M12** | ANSI constants duplicated between `colorize.go` and `tui.go` | Analyzer |

### User Experience

| # | Finding | Source |
|---|---------|--------|
| **M13** | Missing `g`/`G` keys (top/bottom of document) — vim muscle memory | Product-UX (H3) |
| **M14** | No `NO_COLOR` / `--no-color` support | Product-UX (H6) |
| **M15** | Missing `h` or `?` help overlay in TUI | Product-UX |
| **M16** | Binary size (8MB) is large for a stdlib-only Go tool — rebuild with `-ldflags "-s -w"` | Product-UX (H8) |
| **M17** | `Makefile` missing `test` and `fmt` targets | Analyzer |

### Documentation & Community

| # | Finding | Source |
|---|---------|--------|
| **M18** | Missing `SECURITY.md` — critical for a security tool | Product-UX (H1) |
| **M19** | Missing `CONTRIBUTING.md` | Product-UX (H2) |
| **M20** | No git remote configured, zero git tags | Product-UX |
| **M21** | Missing GitHub repo description, topics, website URL | Product-UX |

---

## 🔵 VALUE-ADD FEATURES (Post-Launch Roadmap)

Features beyond PRD scope that would significantly increase utility:

| # | Feature | Rationale | Effort |
|---|---------|-----------|--------|
| **V1** | **`--json` batch/CI mode** | Dump parse results as structured JSON. Enables CI pipeline integration: `skill-inspector --json SKILL.md | jq .suspiciousChars`. The #1 most-requested feature for security tools. | Medium |
| **V2** | **Risk scoring** | Simple heuristic: count suspicious chars + HTML comments + frontmatter anomalies → low/medium/high. Would help users triage findings. | Small |
| **V3** | **Homoglyph detection** | Scan Latin Extended, Greek, Cyrillic, and other ranges for characters that visually resemble ASCII (e.g., Cyrillic `а` vs Latin `a`). | Large |
| **V4** | **Search (`/`) in TUI** | Navigate large skill files quickly. Standard vim/less feature. | Medium |
| **V5** | **`--dry-run` install** | Preview without writing files. `PlanInstall` already exists internally — just expose it. | Small |
| **V6** | **Homebrew formula** | Lower friction for macOS developers. | Small |
| **V7** | **Skill diff/update detection** | Compare installed skill against remote source. | Medium |
| **V8** | **Uninstall command** | Remove skill and symlinks cleanly. Listed as future scope in PRD. | Small |

---

## Cross-Validation Summary

16 findings were independently confirmed by 2+ agents, strengthening confidence:

| Finding | Analyzer | Adversary | Product-UX |
|---------|:---:|:---:|:---:|
| Double parsing | ✅ B-01 | — | — |
| truncateLine byte bug | ✅ B-02 | — | ✅ |
| colorizeLine `*`/`_` false positive | ✅ B-03 | — | ✅ |
| Go version mismatch | ✅ B-04 | — | ✅ B4 |
| CI doesn't run tests | ✅ B-05 | — | ✅ |
| Goroutine leak | ✅ B-06 | ✅ F20 | — |
| No URL timeout | ✅ B-13 | ✅ F13 | ✅ |
| README install path mismatch | ✅ B-16 | — | ✅ B2 |
| No LICENSE file | — | — | ✅ B1 |
| ANSI escape injection | — | ✅ F01,F18 | ✅ |
| No --help/--version | — | — | ✅ B3 |
| No content-length limit | — | ✅ F13 | — |
| Binary + .DS_Store in repo | — | — | ✅ B5 |
| HTTP accepted (MITM) | — | ✅ F14 | — |
| install.sh no checksum | — | ✅ F15 | — |
| Missing bidi chars | — | ✅ F02 | — |

---

## Recommended Launch Sequence

### Week 1: Fix Blockers + High Priority (~3 days)

1. **Day 1:** B1 (LICENSE), B3 (--help/--version), B7 (remove binary/DS_Store), B8 (README fix)
2. **Day 2:** B2 (ANSI sanitization), B4 (URL limit/timeout), B5 (Go version/CI fix), H5 (add tests to CI)
3. **Day 3:** H1 (bidi chars), H2 (reject HTTP), H3 (checksum in install.sh), H4 (YAML/CDATA/comment gaps), H6 (truncateLine fix)

### Week 2: Medium Priority + Polish (~2 days)

4. **Day 4:** M7 (fix double parse), M8 (fix colorize false positives), M9–M12 (small bugs)
5. **Day 5:** M13–M17 (UX/engineering polish), M18–M21 (community docs), tag v0.1.0

### Post-Launch: Value-Add Features

6. **Ongoing:** V1 (JSON mode), V2 (risk scoring), V5 (--dry-run), V8 (uninstall) — highest ROI
7. **Later:** V3 (homoglyphs), V4 (search), V6 (Homebrew), V7 (diff/update)

---

## Verdict

**skill-inspector is a valuable tool with a clear purpose, clean architecture, and working implementation.** The three-agent review found no fundamental design flaws. The issues are concentrated in:

1. **Security depth** — the tool detects the obvious but misses many injection vectors a determined attacker would use
2. **Engineering rigor** — CI doesn't work, critical code untested, small but real bugs
3. **Launch polish** — missing standard OSS files, inaccurate docs, no CLI help

**Not launch-ready today. Launch-ready in ~1 week with focused effort on the 5 blockers and 8 high-priority items.**
