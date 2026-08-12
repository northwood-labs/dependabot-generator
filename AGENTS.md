# Agents Guide

This document provides AI agents with the context needed to work effectively in this repository. For detailed tooling reference, see [docs/agents-reference.md](docs/agents-reference.md).

## What this project does

`dependabot-generator` is a Go CLI tool that scans a directory tree for package ecosystem files (e.g., `go.mod`, `Cargo.toml`, `package.json`) and generates a valid `.github/dependabot.yml` configuration file. Output goes to stdout for shell redirection.

```bash
dependabot-generator run . > .github/dependabot.yml
```

The tool supports optional configuration via `.depgen.toml` files (local, user, global) for customizing header comments, directory ignore patterns, and per-ecosystem field defaults. Detection is driven by filesystem contents and the built-in rules table.

## Architecture at a glance

```text
main.go
  cmd/root.go      Cobra root command, --verbose flag, Execute()
  cmd/run.go       Primary command: config → Scan → Generate → stdout
  cmd/version.go   Version subcommand (delegated to cli-helpers)
  cmd/errors.go    Sentinel error definitions for the CLI layer
  cmd/doc.go       Package documentation

lib/config/
  config.go        Types, constants, defaults (Config, FileConfig, LoadOptions)
  loader.go        LoadConfig(), Validate(), layered TOML merge
  doc.go           Package documentation

lib/scanner/
  scanner.go       Scan(), Generate(), pattern matching, precedence
  rules.go         Data-driven ecosystem detection table (32 rules)
  comment.go       FormatComment(), WrapLine() for YAML header comments
  doc.go           Package documentation

src/               Test fixture directories (one per ecosystem)
docs/              Project documentation
```

The scanner uses an OR-of-AND pattern matching approach: the outer slice is OR (any group matching is sufficient), each inner slice is AND (all patterns in the group must match). After scanning, precedence rules suppress generic ecosystems when a more specific tool is detected:

* `bun` suppresses `npm`
* `uv` suppresses `pip`
* `opentofu` suppresses `terraform`

## Quick command reference

| Action                       | Command                         |
|------------------------------|---------------------------------|
| Build                        | `task build:go`                 |
| Run tests                    | `go test ./...`                 |
| Lint (changed files, fast)   | `task lint`                     |
| Lint (all files, exhaustive) | `task lint:deep:full`           |
| Lint (Go only)               | `golangci-lint run --fix ./...` |
| Update Go dependencies       | `task deps:go`                  |
| Tidy modules                 | `task tidy:go`                  |
| Install dev tools            | `task install:tools`            |
| Install Git hooks            | `task install:hooks`            |

The `Makefile` is a shim that forwards `make <target>` to `task <target>`.

## Technology stack

| Layer              | Technology                                            |
|--------------------|-------------------------------------------------------|
| Language           | Go 1.26.5                                             |
| CLI framework      | `github.com/spf13/cobra` + `charm.land/fang/v2`       |
| Glob matching      | `github.com/goreleaser/fileglob`                      |
| TOML parsing       | `github.com/BurntSushi/toml`                          |
| YAML output        | `gopkg.in/yaml.v3`                                    |
| Text dedentation   | `github.com/lithammer/dedent`                         |
| Shared CLI helpers | `go.nwlabs.dev/cli-helpers/v2` (aliased `clihelpers`) |
| Logging            | `log/slog` via `go.nwlabs.dev/x/logutils`             |
| Testing            | `testing` + `pgregory.net/rapid` (property-based)     |
| Task runner        | [Task](https://taskfile.dev) v3 (`Taskfile.dist.yml`) |
| Linting            | `golangci-lint` v2 with 60+ linters                   |
| Pre-commit hooks   | `hk` (runs via `task lint`)                           |
| Changelog          | `git-cliff` (Conventional Commits)                    |
| Config sync        | `config-manager` (`.config-manager.d/`)               |

## Kiro configuration

### Steering documents (`.kiro/steering/`)

Steering files inject contextual rules when certain files are opened:

| File                              | Triggers on | Purpose                                                                   |
|-----------------------------------|-------------|---------------------------------------------------------------------------|
| `go-code-conventions.md`          | `**/*.go`   | Zero Diagnostics Policy, 60+ lint rules, code style, suppression patterns |
| `go-cli-application.md`           | `**/*.go`   | CLI project structure, Cobra patterns, license header, import order       |
| `markdown-style.md`               | `**/*.md`   | Markdown formatting rules enforced by `rumdl`                             |
| `generate-agents-md.md`           | manual      | Instructions for generating AGENTS.md                                     |
| `comprehensive-and-quickstart.md` | manual      | Instructions for generating architecture docs                             |
| `add-code-comments.md`            | manual      | Instructions for adding WHY-focused code comments                         |
| `root-level-readme.md`            | manual      | Instructions for generating the root README                               |

The file-matched steering documents are the authoritative coding standards. Read them before making changes.

### Specs (`.kiro/specs/`)

| Spec directory         | Status    | Description                                         |
|------------------------|-----------|-----------------------------------------------------|
| `scanner/`             | Completed | Requirements, design, and tasks for the scanner     |
| `yaml-header-comment/` | Completed | Requirements, design, and tasks for header comments |

Specs follow the structure: `requirements.md` (acceptance criteria), `design.md` (architecture, data models, correctness properties), `tasks.md` (implementation plan with checkboxes).

### Hooks

No Kiro hooks are currently configured. The `.kiro/hooks/` directory does not exist. If you create hooks, place them at `.kiro/hooks/<id>.json`.

## Critical rules for agents

1. **Zero Diagnostics Policy** — All Go files must be free of lint errors. Run `golangci-lint run --fix ./...` and `go vet ./...` after every change. Code is not done until diagnostics are zero.

2. **Never edit `.golangci.yml`** — The linter config is managed externally. Resolve diagnostics by fixing code, not by disabling rules.

3. **Config-manager markers** — Sections between `@config-manager:start` and `@config-manager:end` are synced from an external repository. Do not edit them manually.

4. **License header** — Every `.go` file starts with the Apache 2.0 header (year: 2026, Northwood Labs, LLC).

5. **Suppression comments** — Use `// lint:` prefixed comments only. Never use `// nolint:`. See the steering docs for the full suppression table.

6. **Error handling** — Define sentinels with `errors.New`, wrap with `fmt.Errorf("%w: ...")`. No dynamic errors via `fmt.Errorf` alone.

7. **No external test libraries** — Use standard `testing` package. The `depguard` linter denies `stretchr/testify`.

8. **Declaration order** — Files must follow: `const`, `var`, `type`, `func`. Only one `var()` block per file.

9. **Import alias** — `clihelpers "go.nwlabs.dev/cli-helpers/v2"` is the standard alias for the shared helpers package.

10. **Conventional Commits** — Commit messages must follow the [Conventional Commits](https://www.conventionalcommits.org) format. Enforced by `gommit` in pre-commit hooks.

## Verification workflow

After any code change:

```bash
# 1. Check diagnostics
golangci-lint run --fix ./...

# 2. Confirm clean build
go vet ./...

# 3. Run tests for affected package
go test ./lib/scanner/ -count=1
# or for the full project:
go test ./... -count=1
```

## Adding a new ecosystem

1. Add a new entry to `ecosystemRules` in `lib/scanner/rules.go`
2. Create a fixture directory under `src/<ecosystem>/<variant>/` with the detection files
3. Add a test case in `lib/scanner/scanner_test.go`
4. If the ecosystem has precedence over another, add a `precedenceRule` in `lib/scanner/scanner.go`
5. Update `docs/ecosystem.md` with the new entry
6. Run the full test suite and lint

## File layout conventions

| Path                 | Purpose                                |
|----------------------|----------------------------------------|
| `cmd/`               | One file per Cobra command             |
| `lib/config/`        | Layered config resolution, decoupled   |
| `lib/scanner/`       | Core detection logic, decoupled        |
| `src/`               | Test fixture directories               |
| `docs/`              | Project documentation                  |
| `tools/`             | Brewfiles, sub-Taskfiles, tool configs |
| `.config-manager.d/` | Config-manager sync definitions        |
| `.kiro/steering/`    | Kiro AI steering documents             |
| `.kiro/specs/`       | Kiro spec sessions                     |
| `.github/workflows/` | CI/CD pipelines                        |
