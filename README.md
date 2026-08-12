# Dependabot Generator

A Go CLI tool that scans a directory tree for package ecosystem files (e.g., `go.mod`, `Cargo.toml`, `package.json`) and generates a valid `.github/dependabot.yml` configuration file. Instead of manually maintaining your Dependabot config as your project evolves, point this tool at your repository root and it produces a correct configuration based on what it finds.

The tool detects 32 package ecosystems out of the box, handles precedence relationships between overlapping tools (e.g., Bun over npm, uv over pip), and outputs deterministic YAML to stdout for easy shell composition.

## Prerequisites

* **Go 1.26.5+** — required to build from source.
* **Task** ([taskfile.dev](https://taskfile.dev)) — the project's task runner, used for building and linting. Install via Homebrew (`brew install go-task`) or follow the official installation guide.
* **golangci-lint** — required only if you plan to contribute or run lint checks locally.

If you just want to run the pre-built binary, only the compiled `dependabot-generator` executable is needed.

## Installation

Build from source using the Task runner:

```bash
task build:go
```

Or with plain Go:

```bash
go build -o dependabot-generator .
```

## Usage

### Basic usage

Generate a Dependabot configuration for the current directory:

```bash
dependabot-generator run . > .github/dependabot.yml
```

Generate for a different directory:

```bash
dependabot-generator run /path/to/other/repo > .github/dependabot.yml
```

### Custom header comment

Add an inline header comment to the generated file:

```bash
dependabot-generator run . --header "Managed by dependabot-generator. Do not edit."
```

Or point to a file containing the header text:

```bash
dependabot-generator run . --header-file .depgen-header.txt
```

The header can also be set via the `DEPGEN_HEADER` environment variable.

### Configuration file

The tool optionally reads a `.depgen.toml` file from the scan path for additional configuration. This file supports:

```toml
# Custom header comment prepended to the generated YAML.
header = """
# Managed by dependabot-generator.
# See https://docs.github.com/...
"""

# Directories to exclude from scanning (glob patterns matched against base names).
ignore-dirs = ["node_modules", "vendor", ".venv", ".*"]

# Per-ecosystem default fields injected into every update entry.
[ecosystems._default]
"schedule.interval" = "monthly"
"insecure-external-code-execution" = "deny"
```

Without a config file, the tool applies built-in defaults that exclude common vendored directories and set a monthly schedule.

### Priority resolution for header text

When the header is specified in multiple places, the highest-priority source wins:

1. `--header` / `--header-file` CLI flags
2. `DEPGEN_HEADER` environment variable
3. `.depgen.toml` config file

### Version information

```bash
dependabot-generator version
```

## Supported ecosystems

| Ecosystem      | Identifier       | Detection files                                |
|----------------|------------------|------------------------------------------------|
| Bazel          | `bazel`          | `BUILD.bazel` or `BUILD`                       |
| Bun            | `bun`            | `bunfig.toml` or (`package.json` + `bun.lock`) |
| Bundler        | `bundler`        | `gems.rb`, `Gemfile`, or `*.gemspec`           |
| Cargo          | `cargo`          | `Cargo.toml`                                   |
| Composer       | `composer`       | `composer.json`                                |
| Conda          | `conda`          | `environment.yaml` or `environment.yml`        |
| Deno           | `deno`           | `deno.json` or `deno.jsonc`                    |
| Dev containers | `devcontainers`  | `.devcontainer.json` or nested variants        |
| Docker         | `docker`         | `Dockerfile*` or `Containerfile*`              |
| Docker Compose | `docker-compose` | `compose*.yml` or `docker-compose*.yml`        |
| .NET SDK       | `dotnet-sdk`     | `global.json`                                  |
| Elm            | `elm`            | `elm-package.json`                             |
| GitHub Actions | `github-actions` | `action.yml` or `.github/workflows/*.yml`      |
| Git submodule  | `gitsubmodule`   | `.gitmodules`                                  |
| Go modules     | `gomod`          | `go.mod`                                       |
| Gradle         | `gradle`         | `build.gradle` or `build.gradle.kts`           |
| Helm           | `helm`           | `Chart.lock`                                   |
| Julia          | `julia`          | `Project.toml` or `JuliaProject.toml`          |
| Maven          | `maven`          | `pom.xml`                                      |
| Mix (Elixir)   | `mix`            | `mix.exs`                                      |
| Nix            | `nix`            | `flake.nix`                                    |
| npm            | `npm`            | `package.json`                                 |
| NuGet          | `nuget`          | `NuGet.Config`                                 |
| OpenTofu       | `opentofu`       | `.terraform.lock.hcl` + `*.tofu`               |
| pip            | `pip`            | `requirements*.txt`, `setup.py`, etc.          |
| pre-commit     | `pre-commit`     | `.pre-commit-config.yaml`                      |
| pub (Dart)     | `pub`            | `pubspec.yaml`                                 |
| Rust toolchain | `rust-toolchain` | `rust-toolchain` or `rust-toolchain.toml`      |
| sbt            | `sbt`            | `build.sbt`                                    |
| Swift          | `swift`          | `Package.swift`                                |
| Terraform      | `terraform`      | `.terraform.lock.hcl` + `*.tf`                 |
| uv             | `uv`             | `uv.lock`                                      |
| vcpkg          | `vcpkg`          | `vcpkg-configuration.json`                     |

### Precedence rules

When both a specific tool and its generic counterpart are detected in the same directory, the more specific tool wins:

| Winner     | Suppresses  | Rationale                       |
|------------|-------------|---------------------------------|
| `bun`      | `npm`       | Bun manages the `package.json`  |
| `uv`       | `pip`       | uv replaces pip for the project |
| `opentofu` | `terraform` | OpenTofu is a drop-in fork      |

## Running tests

### Full test suite

```bash
go test ./... -count=1
```

### Scanner package only

```bash
go test ./lib/scanner/ -count=1
```

### Config package only

```bash
go test ./lib/config/ -count=1
```

### How tests work

The test suite combines three testing strategies:

* **Unit tests** (`scanner_test.go`, `loader_test.go`) verify error handling, path validation, and individual function behavior using standard Go `testing`.
* **Fixture-based integration tests** scan real directory structures under `src/` (one per ecosystem) to confirm end-to-end detection against actual filesystem layouts.
* **Property-based tests** (`scanner_property_test.go`, `config_property_test.go`) use [`pgregory.net/rapid`](https://pkg.go.dev/pgregory.net/rapid) to generate random inputs and verify invariants like sort stability, precedence correctness, and round-trip encoding across thousands of generated cases.

### Linting

```bash
golangci-lint run --fix ./...
```

Or via Task:

```bash
task lint
```

## Troubleshooting

### "root path does not exist"

The path argument does not point to a valid location on disk. Verify the path exists and check for typos.

### "root path is not a directory"

You pointed the tool at a file instead of a directory. Provide the directory containing your project files.

### "root path is not accessible"

The current user does not have read permission on the target directory. Check filesystem permissions with `ls -la`.

### Ecosystem not detected

The scanner relies on specific file names to identify ecosystems. Verify that the expected files exist in the target directory. For ecosystems that require multiple files (like OpenTofu needing both `.terraform.lock.hcl` and `*.tofu`), all required files must be present in the same directory.

### Unwanted matches from vendored directories

By default, the tool excludes `node_modules`, `vendor`, `.venv`, `venv`, and all hidden directories (`.*`). If you still see unwanted matches, add custom ignore patterns to `.depgen.toml`:

```toml
ignore-dirs = ["node_modules", "vendor", ".venv", "venv", ".*", "third_party"]
```

### Generated file missing `schedule` field

If no `.depgen.toml` exists and no built-in defaults apply, the output may omit the `schedule` field that Dependabot requires. Create a `.depgen.toml` with ecosystem defaults or add schedule configuration manually after generation.

### Bun not suppressing npm

If both `package.json` and `bun.lock` (or `bunfig.toml`) exist in the same directory, only `bun` should appear. If npm still shows up, verify that the Bun detection files are in the same directory as `package.json`.

## License

Apache License 2.0. See the license header in source files for details.
