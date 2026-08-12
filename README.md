# Dependabot Generator

Keeping `.github/dependabot.yml` in sync with your actual project structure is tedious. Every time you add a new language, framework, or subdirectory, you have to remember to update the config. Miss one and your dependencies drift silently.

`dependabot-generator` eliminates that maintenance burden. Point it at any repository and it produces a complete, correct Dependabot configuration by detecting what package ecosystems actually exist on disk. It handles 32 ecosystems, resolves conflicts between overlapping tools (Bun vs npm, uv vs pip, OpenTofu vs Terraform), and outputs deterministic YAML you can commit directly.

```bash
dependabot-generator run . > .github/dependabot.yml
```

No config files required. No manual inventory. One command, always up to date.

## Prerequisites

If you just want to run the pre-built binary, only the compiled `dependabot-generator` executable is needed — no runtime dependencies.

To build from source:

* **Go 1.26.5+** — required to compile the binary.

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

Preview what would be generated without writing a file:

```bash
dependabot-generator run .
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

The tool optionally reads `.depgen.toml` for additional configuration. It searches three locations in priority order (highest wins):

1. `<scan-path>/.depgen.toml` (repository-local)
2. `$XDG_CONFIG_HOME/dependabot-generator/config.toml` (user-level)
3. `/etc/dependabot-generator/config.toml` (organization-level)

Example `.depgen.toml`:

```toml
# Custom header comment prepended to the generated YAML.
header = """
# Managed by dependabot-generator.
# See https://docs.github.com/github/administering-a-repository/configuration-options-for-dependency-updates
"""

# Directories to exclude from scanning (glob patterns matched against base names).
ignore-dirs = [
  "node_modules",
  "vendor",
  ".venv",
  "venv",
  ".*",
]

# Per-ecosystem default fields injected into every update entry.
[ecosystems._default]
"schedule.interval" = "monthly"
"cooldown.default-days" = 3
"groups.batch.patterns" = ["*"]
```

Without a config file, the tool applies built-in defaults that exclude common vendored directories and set a monthly schedule with batch grouping.

### Priority resolution for header text

When the header is specified in multiple places, the highest-priority source wins:

1. `--header` / `--header-file` CLI flags
2. `DEPGEN_HEADER` environment variable
3. Config file (`header` key)

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

* **Unit tests** (`scanner_test.go`, `generate_test.go`, `comment_test.go`, `loader_test.go`) verify error handling, path validation, YAML generation, comment formatting, and config loading using standard Go `testing`.

* **Fixture-based integration tests** (`integration_test.go`) scan real directory structures under `src/` (one per ecosystem) to confirm end-to-end detection against actual filesystem layouts.

* **Property-based tests** (`scanner_property_test.go`, `comment_property_test.go`, `config_property_test.go`) use [`pgregory.net/rapid`](https://pkg.go.dev/pgregory.net/rapid) to generate random inputs and verify invariants like sort stability, precedence correctness, comment formatting guarantees, and priority resolution across thousands of generated cases.

## Troubleshooting

### "root path does not exist"

The path argument does not point to a valid location on disk. Verify the path exists and check for typos.

### "root path is not a directory"

You pointed the tool at a file instead of a directory. Provide the directory containing your project files.

### "root path is not accessible"

The current user does not have read permission on the target directory. Check filesystem permissions with `ls -la`.

### "header comment exceeds maximum size"

The resolved header text (from `--header`, `--header-file`, environment variable, or config file) exceeds the 8 KiB safety limit. Shorten the header content or use a more concise message.

### "--header and --header-file are mutually exclusive"

Both flags were provided at the same time. Use one or the other, not both. If you need the header in a file, use only `--header-file`.

### Ecosystem not detected

The scanner relies on specific file names to identify ecosystems. Verify that the expected files exist in the target directory. For ecosystems that require multiple files (like OpenTofu needing both `.terraform.lock.hcl` and `*.tofu`), all required files must be present in the same directory.

### Unwanted matches from vendored directories

By default, the tool excludes `node_modules`, `vendor`, `.venv`, `venv`, and all hidden directories (`.*`). If you still see unwanted matches, add custom ignore patterns to `.depgen.toml`:

```toml
ignore-dirs = [
  "node_modules",
  "vendor",
  ".venv",
  "venv",
  ".*",
  "third_party",
]
```

Note that specifying `ignore-dirs` replaces the entire default list, so include the built-in patterns you want to keep.

### Bun not suppressing npm

If both `package.json` and `bun.lock` (or `bunfig.toml`) exist in the same directory, only `bun` should appear. If npm still shows up, verify that the Bun detection files are in the same directory as `package.json`.

### Generated file missing expected fields

If the generated YAML lacks `schedule` or other fields that Dependabot requires, create a `.depgen.toml` with ecosystem defaults:

```toml
[ecosystems._default]
"schedule.interval" = "monthly"
```

The built-in defaults include a monthly schedule, but if a config file overrides the `_default` ecosystem key, it replaces the defaults entirely.

## License

Apache License 2.0. See the license header in source files for details.
