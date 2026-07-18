# Contributing to skill-inspector

Thanks for your interest in contributing! `skill-inspector` is a security audit tool for agent skill files. Contributions that improve safety, clarity, or portability are welcome.

## Development Environment

You need **Go 1.21 or later**.

```sh
git clone https://github.com/parksjr/skill-inspector.git
cd skill-inspector
```

No other dependencies — `skill-inspector` uses only `golang.org/x/term` (and its indirect dependency `golang.org/x/sys`) beyond the standard library.

## Building

```sh
make build        # Build for your current platform
make vet          # Run static analysis (go vet)
make release      # Cross-compile all release targets (darwin/linux, amd64/arm64)
make clean        # Remove build artifacts
```

### Without Make

```sh
go build -o skill-inspector .
```

## Running Tests

```sh
go vet ./...      # Static analysis
```

We do not yet have a test suite. If you add tests, place them alongside the code they test (e.g., `internal/parser/parser_test.go`).

## Code Style

- Run `gofmt` (or `go fmt ./...`) before committing. We enforce `gofmt` on all Go files.
- Keep the dependency footprint minimal. New dependencies need strong justification.
- Follow standard Go conventions: clear variable names, doc comments on exported symbols, errors returned (not panicked).

## Pull Request Process

1. **Open an issue first** for anything beyond a typo fix. Describe what you want to change and why.
2. **Fork the repo** and create a feature branch from `main`.
3. **Keep changes focused.** One PR, one concern.
4. **Run `go vet ./...`** and ensure it passes.
5. **Open a PR** against `main` with a clear description of what changed and why.

## Developer Certificate of Origin (DCO)

By contributing, you certify that:

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2005 The Linux Foundation and its contributors.

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.
```

Include a `Signed-off-by: Your Name <email>` line in your commit messages to confirm this.

## Questions?

Open an issue on [GitHub](https://github.com/parksjr/skill-inspector/issues).
