# Agents Reference

Detailed reference material for AI agents working in this repository. For a quick overview, see [AGENTS.md](../AGENTS.md) at the project root.

## Dependency graph

### Direct dependencies

| Package                          | Purpose                                    |
|----------------------------------|--------------------------------------------|
| `github.com/spf13/cobra`         | CLI command framework                      |
| `charm.land/fang/v2`             | Terminal color detection wrapper for Cobra |
| `github.com/goreleaser/fileglob` | Advanced glob matching (supports `**`)     |
| `github.com/lithammer/dedent`    | Dedents multi-line help text strings       |
| `go.yaml.in/yaml/v4`             | YAML marshaling for output                 |
| `go.nwlabs.dev/cli-helpers/v2`   | Shared Northwood Labs CLI helpers          |
| `go.nwlabs.dev/x/logutils`       | Logger initialization based on verbosity   |
| `pgregory.net/rapid`             | Property-based testing framework           |

### Indirect dependencies (notable)

The Charm ecosystem (`lipgloss`, `bubbletea`, `glamour`) is pulled in by `cli-helpers` for rich terminal output. These are indirect and not used directly by this project.

## Scanner internals

### Pattern matching algorithm

For each directory encountered during the walk:

```text
for each rule in ecosystemRules:
    for each andGroup in rule.Files:       # OR level
        allMatch = true
        for each pattern in andGroup:      # AND level
            matches = fileglob.Glob(dir + "/" + pattern)
            matches = filterMatchesByDepth(dir, pattern, matches)
            if len(matches) == 0:
                allMatch = false
                break
        if allMatch:
            record ScanResult{Directory, Ecosystem}
            break  # first OR match is sufficient
```

### Depth filtering

`fileglob` can recurse into directories whose names match non-glob patterns. `filterMatchesByDepth` restricts matches to the expected path depth, unless the pattern contains `**` (which intentionally matches at any depth).

### Precedence resolution (post-processing)

```text
precedenceRules = [
    {Winner: "bun",      Loser: "npm"},
    {Winner: "opentofu", Loser: "terraform"},
    {Winner: "uv",       Loser: "pip"},
]
```

Two-pass approach:

1. Build a set of directories where each winner appears
2. Filter out losers from those directories

### YAML generation

* Copy results (avoid mutating caller's slice)
* Sort: directory ascending, then ecosystem ascending
* Map to `dependabotUpdate` structs
* Encode with `yaml.NewEncoder` (2-space indent)
* Prepend `---\n` document separator

## Supported ecosystems (32 total)

| Identifier       | Detection files                                                                                   |
|------------------|---------------------------------------------------------------------------------------------------|
| `bazel`          | `BUILD.bazel` or `BUILD`                                                                          |
| `bun`            | `bunfig.toml` or (`package.json` + `bun.lock`)                                                    |
| `bundler`        | `gems.rb` or `Gemfile` or `*.gemspec`                                                             |
| `cargo`          | `Cargo.toml`                                                                                      |
| `composer`       | `composer.json`                                                                                   |
| `conda`          | `environment.yaml` or `environment.yml`                                                           |
| `deno`           | `deno.json` or `deno.jsonc`                                                                       |
| `devcontainers`  | `.devcontainer.json` or `.devcontainer/devcontainer.json` or `.devcontainer/**/devcontainer.json` |
| `docker`         | `Dockerfile*` or `Containerfile*`                                                                 |
| `docker-compose` | `docker-compose*.yml` / `.yaml` or `compose*.yml` / `.yaml`                                       |
| `dotnet-sdk`     | `global.json`                                                                                     |
| `elm`            | `elm-package.json`                                                                                |
| `github-actions` | `action.yml` / `.yaml` or `.github/workflows/*.yml` / `.yaml`                                     |
| `gitsubmodule`   | `.gitmodules`                                                                                     |
| `gomod`          | `go.mod`                                                                                          |
| `gradle`         | `build.gradle` or `build.gradle.kts`                                                              |
| `helm`           | `Chart.lock`                                                                                      |
| `julia`          | `Project.toml` / `JuliaProject.toml` / `Manifest*.toml` / `JuliaManifest*.toml`                   |
| `maven`          | `pom.xml`                                                                                         |
| `mix`            | `mix.exs`                                                                                         |
| `nix`            | `flake.nix`                                                                                       |
| `npm`            | `package.json`                                                                                    |
| `nuget`          | `NuGet.Config`                                                                                    |
| `opentofu`       | `.terraform.lock.hcl` + `*.tofu`                                                                  |
| `pip`            | `*requirements*.txt` / `.in` / `setup.cfg` / `setup.py` / `pyproject.toml` / `Pipfile`            |
| `pre-commit`     | `.pre-commit-config.yml` / `.yaml` / `.pre-commit.yml` / `.yaml`                                  |
| `pub`            | `pubspec.yaml`                                                                                    |
| `rust-toolchain` | `rust-toolchain` or `rust-toolchain.toml`                                                         |
| `sbt`            | `build.sbt`                                                                                       |
| `swift`          | `Package.swift`                                                                                   |
| `terraform`      | `.terraform.lock.hcl` + `*.tf`                                                                    |
| `uv`             | `uv.lock`                                                                                         |
| `vcpkg`          | `vcpkg-configuration.json`                                                                        |

## Testing strategy

### Test types

| Type             | Location                               | Framework            |
|------------------|----------------------------------------|----------------------|
| Unit tests       | `lib/scanner/scanner_test.go`          | `testing`            |
| Generation tests | `lib/scanner/generate_test.go`         | `testing`            |
| Property tests   | `lib/scanner/scanner_property_test.go` | `pgregory.net/rapid` |

### Test fixtures

The `src/` directory contains real directory structures used by integration tests. Each ecosystem has one or more variant directories (e.g., `src/bazel/a`, `src/bun/b`). Tests verify that `Scan()` correctly detects the ecosystem based on the fixture files present.

### Running tests

```bash
# Full test suite
go test ./... -count=1

# Scanner package only, verbose
go test ./lib/scanner/ -count=1 -v

# Property tests with more iterations
go test ./lib/scanner/ -count=1 -rapid.checks=1000
```

### Test conventions

* No external assertion libraries (testify is denied by `depguard`)
* Property tests tagged: `// Feature: scanner, Property {N}: {title}`
* Minimum 100 iterations per property test
* Test-specific dynamic errors use `// lint:allow_errorf` suppression

## CI/CD pipelines

| Workflow                    | Trigger         | Purpose                               |
|-----------------------------|-----------------|---------------------------------------|
| `test.yml`                  | push/PR to main | Unit tests                            |
| `update-on-push.yml`        | push to main    | Generate CHANGELOG + CONTRIBUTORS     |
| `dependabot-auto-merge.yml` | PR events       | Auto-merge minor/patch Dependabot PRs |
| `codeql.yml`                | scheduled/PR    | CodeQL security analysis              |
| `dependency-review.yml`     | PR              | Dependency change review              |
| `osv-scanner.yml`           | scheduled       | OSV vulnerability scanning            |
| `scorecard.yml`             | scheduled       | OpenSSF Scorecard                     |
| `trufflehog.yml`            | push/PR         | Secret scanning                       |

All workflows use `step-security/harden-runner` with egress blocking. Action versions are pinned to full SHA hashes (managed by `pinact`).

## Config-manager system

The `.config-manager.d/` directory defines files synced from a central repository (`file://../.github`). Three TOML files control the sync:

| File            | Scope                                     |
|-----------------|-------------------------------------------|
| `config.toml`   | Global settings (upstream path, timeouts) |
| `standard.toml` | ~50+ shared config files                  |
| `golang.toml`   | Go-specific configs                       |

Files managed by config-manager contain `@config-manager:start`/`end` markers. Content between these markers is overwritten on sync. Do not edit these sections manually.

## Dev tool ecosystem

All development tools are installed via Homebrew:

* `tools/Brewfile.standard` - General tools (git-cliff, hk, lychee, rumdl, shellcheck, shuck, taplo, trivy, yamlfmt, zizmor, etc.)
* `tools/Brewfile.golang` - Go-specific tools (golangci-lint, gofumpt, golines, govulncheck, gotestsum, etc.)

The project uses GNU versions of common tools on macOS:

```text
BASH:  /opt/homebrew/opt/bash/bin/bash
FIND:  /opt/homebrew/opt/findutils/bin/gfind
GREP:  /opt/homebrew/opt/grep/bin/ggrep
SED:   /opt/homebrew/opt/gsed/bin/gsed
XARGS: /opt/homebrew/opt/findutils/bin/gxargs
RM:    /opt/homebrew/opt/coreutils/bin/grm
```

## Linting reference

### Key thresholds

| Linter     | Setting              |
|------------|----------------------|
| `lll`      | 120 characters max   |
| `gocognit` | complexity max 20    |
| `gocyclo`  | complexity max 21    |
| `funlen`   | (default, ~60 lines) |
| `goconst`  | min-len 80 chars     |

### Declaration order (enforced by `decorder`)

```text
const → var → type → func
```

Only one `var()` block per file. Inside closures assigned within that block (e.g., Cobra `RunE`), use short declarations (`:=`) instead of `var`.

### Common lint fix patterns

| Linter      | Pattern                                                    |
|-------------|------------------------------------------------------------|
| `sloglint`  | Use `*Context` variants, `snake_case` keys, no `slog.Attr` |
| `err113`    | Define sentinels, wrap with `%w`                           |
| `wrapcheck` | Wrap errors from external packages                         |
| `revive`    | Max 4 params (extract to struct), no bool control flags    |
| `gocritic`  | Pass structs >64 bytes by pointer                          |
| `govet`     | Rename inner shadowed variables (e.g., `closeErr`)         |
| `gosec`     | Dir perms `0o0755`, file perms `0o0666`                    |
| `dupl`      | Extract shared code or use `// lint:no_dupe`               |

### Suppression format

Always `// lint:<tag>`, never `// nolint:`. Full table in `.kiro/steering/go-code-conventions.md`.

## Steering document details

### `go-code-conventions.md`

The most important steering document. Covers:

* Zero Diagnostics Policy and verification workflow
* Resolution workflow (group by linter, fix fewest-errors-first)
* Linter categories with specific fix patterns
* Code conventions (interfaces, parameters, return values, error handling)
* Suppression comment table
* Performance patterns (use `strings.Builder` for stdout)

### `go-cli-application.md`

CLI-specific patterns:

* Cobra command structure (one file per command)
* Flag naming (lowercase `f` prefix: `fVerbose`, `fToggle`)
* `init()` with `// lint:allow_init` comment
* Version command pattern (delegates to `clihelpers.VersionScreen()`)
* Package documentation in `doc.go` files
* Import organization (stdlib, third-party, internal)

### `markdown-style.md`

Markdown formatting rules:

* ATX headings, sentence case (except H1 which is Title Case)
* Asterisk (`*`) for unordered lists
* Fenced code blocks with language identifiers
* Proper name casing (Dependabot, Northwood Labs, OpenTofu, etc.)
* 2-space indentation for Markdown

## Extending this project

### Adding a new CLI command

1. Create `cmd/<name>.go` with the Apache 2.0 header
2. Define a `var <name>Cmd = &cobra.Command{...}` in a `var()` block
3. Register with `rootCmd.AddCommand(<name>Cmd)` in `init()`
4. Mark `init()` with `// lint:allow_init`
5. Use `clihelpers.LongHelpText()` for long descriptions

### Adding a new library package

1. Create `lib/<name>/` directory
2. Add a `doc.go` with the Apache 2.0 header and `// Package <name>...`
3. Define sentinel errors for all failure modes
4. Return wrapped errors, never raw strings
5. Write property tests where input space is large/varied

### Creating a kiro spec

1. Create `.kiro/specs/<feature>/` directory
2. Add `requirements.md` with user stories and acceptance criteria
3. Add `design.md` with architecture, data models, and correctness properties
4. Add `tasks.md` with implementation plan and dependency graph
5. Tasks use `[x]`/`[ ]` checkboxes for progress tracking
