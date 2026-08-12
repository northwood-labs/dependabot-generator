# Quick Flow Summary

## Entrypoint

The application starts at `main.go`, which does exactly one thing: call `cmd.Execute()`. This keeps the entrypoint trivial and pushes all CLI wiring into the `cmd` package where it can be tested and extended independently.

```text
main.go → cmd.Execute() → fang.Execute(ctx, rootCmd) → Cobra dispatch
```

## Primary flow

The `run` command is the only user-facing action. It implements a multi-stage pipeline:

```text
1. Parse CLI args (default path: ".")
2. Validate mutual exclusivity of --header / --header-file
3. Read --header-file contents (if specified)
4. Resolve DEPGEN_HEADER environment variable
5. config.LoadConfig(opts)             → layered TOML merge + priority resolution
6. config.Validate(cfg)                → check ignore patterns are well-formed
7. Enforce header size limit (8 KiB)
8. scanner.Scan(path, cfg.IgnoreDirs)  → walk directory tree, match ecosystem rules
9. Convert config ecosystem defaults → scanner.EcosystemSettings
10. scanner.Generate(results, genOpts) → sort results, encode to YAML with extras
11. Print YAML to stdout
```

The user captures output via shell redirection:

```bash
dependabot-generator run . > .github/dependabot.yml
```

### Configuration resolution

Before scanning, the `run` command resolves configuration from multiple sources. The priority order (highest wins):

```text
CLI flags (--header, --header-file)
  ↓ overrides
Environment variable (DEPGEN_HEADER)
  ↓ overrides
Local config file (.depgen.toml in scan path)
  ↓ overrides
User config file ($XDG_CONFIG_HOME/dependabot-generator/config.toml)
  ↓ overrides
Global config file (/etc/dependabot-generator/config.toml)
  ↓ overrides
Built-in defaults (compiled into the binary)
```

The config layer resolves three concerns: header comment text, directory ignore patterns, and per-ecosystem field defaults.

### Scan stage

* Validates the root path (exists, is a directory, is readable).
* Resolves to an absolute path (required by the glob library).
* Walks the directory tree using `fs.WalkDir` over `os.DirFS`.
* Skips directories matching any pattern from `cfg.IgnoreDirs`.
* For each directory, evaluates all 32 ecosystem rules from the rules table.
* Each rule uses OR-of-AND pattern matching: the outer slice is OR (any group matching is sufficient), each inner slice is AND (all patterns must match).
* After the walk, applies precedence rules to suppress generic ecosystems when a more specific tool is detected (e.g., `bun` suppresses `npm`, `uv` suppresses `pip`).

### Generate stage

* Copies and sorts results deterministically (directory ascending, then ecosystem ascending).
* Looks up per-ecosystem extra fields from `EcosystemDefaults` (falls back to `_default` key).
* Builds a YAML Node tree for deterministic key ordering per update entry.
* Encodes via `gopkg.in/yaml.v3` with 2-space indentation.
* Prepends the `---` YAML document separator.
* Inserts formatted header comment (if configured) between the separator and the body.

### Comment formatting

When a header comment is configured, `FormatComment` processes the raw text:

* Strips trailing newlines and returns empty for whitespace-only input.
* Lines already prefixed with `#` are preserved unchanged.
* Short lines (within 78 characters) get a `# ` prefix.
* Lines containing URLs are preserved intact regardless of length.
* Long lines without URLs are reflowed via `WrapLine` to fit within the 78-character content limit.
* Internal blank lines become bare `#` lines.

## Module roles

| Module                   | Responsibility                                              |
|--------------------------|-------------------------------------------------------------|
| `main.go`                | Trivial entrypoint, calls `cmd.Execute()`                   |
| `cmd/`                   | Cobra command definitions, flag registration, CLI lifecycle |
| `cmd/root.go`            | Root command (help-only), verbose flag, `Execute()`         |
| `cmd/run.go`             | Primary command: config resolution, Scan, Generate, stdout  |
| `cmd/version.go`         | Version subcommand (delegated to shared cli-helpers)        |
| `cmd/errors.go`          | Sentinel error definitions for the CLI layer                |
| `lib/config/`            | Layered TOML config loading, merging, and validation        |
| `lib/config/config.go`   | Types, defaults, and sentinel errors for config             |
| `lib/config/loader.go`   | `LoadConfig()`, `Validate()`, file resolution               |
| `lib/scanner/`           | Core detection and generation logic, decoupled from CLI     |
| `lib/scanner/scanner.go` | `Scan()`, `Generate()`, pattern evaluation, precedence      |
| `lib/scanner/rules.go`   | Data-driven ecosystem detection table (32 rules)            |
| `lib/scanner/comment.go` | `FormatComment()`, `WrapLine()` for YAML header comments    |
| `src/`                   | Test fixture directories (one per ecosystem)                |
| `docs/`                  | Project documentation                                       |

## Design decisions

### Convention over configuration

The tool requires zero mandatory setup. Point it at a directory and it produces output. The optional `.depgen.toml` config adds flexibility (custom schedules, ignore patterns, header text) without breaking the zero-config default path.

### Data-driven rules table

Adding a new ecosystem requires only a single struct entry in `rules.go`. No procedural code changes, no new functions, no conditional branches. The table is sorted alphabetically for human readability, and runtime evaluation checks every rule independently regardless of order.

### OR-of-AND pattern matching

Ecosystem detection often requires expressing "file A exists" OR "files B AND C both exist." The two-level `[][]string` encoding covers every real Dependabot detection pattern without needing a custom expression language or DSL.

### Post-processing precedence

A winner (e.g., `bun`) and its loser (e.g., `npm`) may be discovered in any order during the walk. Resolving precedence after the full walk guarantees correctness regardless of discovery order and keeps the walk logic simple.

### Layered configuration

The config system mirrors established conventions (global, user, local, env, CLI) so that teams can set organization defaults in `/etc/` or user preferences in `$XDG_CONFIG_HOME` while individual repositories override via `.depgen.toml`.

### Stdout-only output

Following Unix conventions, the tool writes to stdout so users can pipe, redirect, or compose with other tools. No `--output` flag is needed.

### Deterministic output

Sorting results before YAML encoding guarantees that identical inputs always produce identical output. Users commit `dependabot.yml` and review diffs in PRs, so non-deterministic ordering would create noise.

### Scanner isolation from CLI

The `lib/scanner` package has no dependency on Cobra, Fang, or any terminal library. Tests run without CLI bootstrapping, and the scanner can be imported as a library by other tools.

### Config isolation from scanner

The `lib/config` package handles TOML parsing and merge logic without knowing about scanning or YAML generation. The `cmd` layer is the only place that connects config output to scanner input, keeping both libraries independently testable and reusable.

### Property-based testing

The test suite uses `pgregory.net/rapid` for property-based testing alongside traditional unit tests and fixture-based integration tests. This provides coverage of edge cases that hand-written tests miss, and verifies invariants (sort stability, precedence correctness, round-trip encoding) across random inputs.

## Risks and unknowns

* **No merge with existing configuration.** The tool always generates a fresh `dependabot.yml`. Manual customizations (reviewers, labels, ignore rules, groups) are lost on regeneration.
* **Glob library edge cases.** `fileglob` can produce unexpected results when directory names match non-glob patterns. The `filterMatchesByDepth` function mitigates this, but symlink cycles or OS-specific path issues could still cause false positives.
* **Ecosystem rule staleness.** The 32 rules are derived from Dependabot's source code. If Dependabot adds new ecosystems or changes detection patterns, the tool needs a manual update.
* **No output validation.** The generated YAML is not validated against GitHub's Dependabot schema. Malformed output from bugs would only surface when GitHub rejects the file.
* **Header size limit.** The 8 KiB cap on resolved header text is an arbitrary guard. If a user hits this limit, the error message explains it, but there is no override mechanism.
* **Config ignore-dirs replacement semantics.** When a higher-priority config file specifies `ignore-dirs`, it replaces the entire list rather than appending. A local `.depgen.toml` that adds one custom pattern must also re-declare all the built-in defaults.
