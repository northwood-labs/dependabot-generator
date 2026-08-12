# Deep Architecture Audit

## Entry points

The application has a single binary entry point:

| File      | Function | Role                                   |
|-----------|----------|----------------------------------------|
| `main.go` | `main()` | Calls `cmd.Execute()` and nothing else |

The `main.go` file is intentionally minimal. All logic lives in the `cmd`, `lib/config`, and `lib/scanner` packages so that the CLI is testable without executing a binary.

The scanner package exposes two public entry points for programmatic use:

| Function                              | Purpose                                     |
|---------------------------------------|---------------------------------------------|
| `scanner.Scan(path, ignoreDirs)`      | Walk a directory tree and detect ecosystems |
| `scanner.Generate(results, *GenOpts)` | Convert scan results to YAML                |
| `scanner.FormatComment(raw)`          | Format raw text into YAML comment lines     |

The config package exposes:

| Function                   | Purpose                                             |
|----------------------------|-----------------------------------------------------|
| `config.LoadConfig(*Opts)` | Resolve config from all sources with priority merge |
| `config.Validate(*Config)` | Check ignore patterns are well-formed               |

## CLI startup and initialization flow

### Package initialization order

Go guarantees `init()` functions run before `main()`. The CLI's startup sequence is:

```text
1. cmd/root.go init()    → registers --verbose/-v persistent flag
2. cmd/run.go init()     → registers --header, --header-file flags; adds runCmd to rootCmd
3. cmd/version.go init() → registers versionCmd as a subcommand of rootCmd
4. main.go main()        → calls cmd.Execute()
```

### Execute function

```text
cmd.Execute()
  → fang.Execute(context.Background(), rootCmd)
    → detects terminal color capabilities
    → calls rootCmd.Execute() (Cobra dispatch)
  → on error: os.Exit(1)
```

The `fang` wrapper ensures the CLI renders correctly on both interactive terminals and piped output. Cobra handles argument parsing, flag binding, help generation, and subcommand dispatch internally.

### Root command

`rootCmd` has no `RunE` function. Invoking the binary without a subcommand falls through to Cobra's automatic help display. This is intentional because the tool requires an explicit subcommand (`run` or `version`).

### Shared state

The `cmd` package declares these package-level variables shared across commands:

| Variable      | Type     | Purpose                                       |
|---------------|----------|-----------------------------------------------|
| `fVerbose`    | `int`    | Count flag for progressive verbosity (-v/-vv) |
| `fHeader`     | `string` | Inline header text from --header flag         |
| `fHeaderFile` | `string` | Path to header file from --header-file flag   |

The `fVerbose` count supports `-v`, `-vv`, and `-vvv` levels, matching conventions from tools like `ssh` and `curl`.

## Command-specific flows

### The `run` command

This is the primary user-facing command. Its lifecycle:

```text
RunE:
  1. path = args[0] or "." (default)
  2. Guard: --header and --header-file are mutually exclusive
  3. If --header-file set → read file contents (or return validation error)
  4. Read DEPGEN_HEADER environment variable
  5. config.LoadConfig(opts) → layered TOML resolution
  6. config.Validate(cfg) → verify ignore patterns
  7. Guard: header size ≤ 8192 bytes
  8. scanner.Scan(path, cfg.IgnoreDirs) → ecosystem detection
  9. Convert cfg.EcosystemDefaults → scanner.EcosystemSettings
  10. scanner.Generate(results, genOpts) → YAML output
  11. fmt.Fprint(os.Stdout, output)
```

**Arguments**: Accepts 0 or 1 positional args (`cobra.RangeArgs(0, 1)`). Defaults to `"."` when no argument is provided.

**Flags**:

| Flag            | Type   | Purpose                        |
|-----------------|--------|--------------------------------|
| `--header`      | string | Inline header comment text     |
| `--header-file` | string | Path to file containing header |

These are mutually exclusive. Both set at once returns `ErrFlagsMutuallyExclusive`.

**Error handling**: All errors are wrapped with user-facing descriptions before returning to Cobra, which prints the error and triggers the `os.Exit(1)` in `Execute()`. Sentinel errors from `lib/config` are translated to CLI-layer sentinels via `mapConfigError`.

**Output strategy**: YAML goes to stdout so users can pipe or redirect. No file writing logic exists in the command itself.

### The `version` command

Entirely delegated to `clihelpers.VersionScreen()` from the shared `go.nwlabs.dev/cli-helpers/v2` package. No custom logic. All Northwood Labs CLI tools share a consistent version display format.

## File and module responsibilities

### `cmd/` package

| File         | Exports           | Responsibility                              |
|--------------|-------------------|---------------------------------------------|
| `doc.go`     | (package comment) | Documents the cmd package's purpose         |
| `root.go`    | `Execute()`       | Root command, verbose flag, Cobra lifecycle |
| `run.go`     | (none)            | Run subcommand orchestration                |
| `version.go` | (none)            | Version subcommand registration             |
| `errors.go`  | `Err*` sentinels  | All CLI-layer sentinel error definitions    |

### `lib/config/` package

| File                      | Exports / Key Symbols    | Responsibility                             |
|---------------------------|--------------------------|--------------------------------------------|
| `doc.go`                  | (package comment)        | Documents the config package               |
| `config.go`               | Types, defaults, errors  | Type definitions and compiled-in defaults  |
| `loader.go`               | `LoadConfig`, `Validate` | TOML file loading, merge logic, validation |
| `loader_test.go`          | (tests)                  | Unit tests for config loading              |
| `config_property_test.go` | (tests)                  | Property-based tests for config layer      |

### `lib/scanner/` package

| File                       | Exports / Key Symbols     | Responsibility                   |
|----------------------------|---------------------------|----------------------------------|
| `doc.go`                   | (package comment)         | Documents the scanner package    |
| `scanner.go`               | `Scan`, `Generate`, types | Core logic and YAML encoding     |
| `rules.go`                 | `ecosystemRules`          | Detection rule table (32 rules)  |
| `comment.go`               | `FormatComment`           | Header comment formatting        |
| `scanner_test.go`          | (tests)                   | Unit tests for scan logic        |
| `scanner_property_test.go` | (tests)                   | Property-based fuzz tests        |
| `generate_test.go`         | (tests)                   | YAML generation tests            |
| `integration_test.go`      | (tests)                   | Fixture-based integration tests  |
| `comment_test.go`          | (tests)                   | Unit tests for comment formatter |
| `comment_property_test.go` | (tests)                   | Property-based comment tests     |

### `src/` directory (test fixtures)

Contains directories (one per ecosystem) each with filesystem structures that match the corresponding detection rule. Used by integration tests to verify end-to-end scanning against real file layouts.

### Key types

```text
lib/config:
  Config              — fully-resolved config after merging all sources
  EcosystemConfig     — per-ecosystem field overrides (dotted-path keys)
  FileConfig          — TOML file structure before merging
  LoadOptions         — inputs needed for config resolution

lib/scanner:
  EcosystemRule       — detection pattern: Identifier + Files (OR-of-AND groups)
  ScanResult          — detected ecosystem entry: Directory + Ecosystem
  precedenceRule      — winner/loser relationship between ecosystems
  GenerateOptions     — CommentText + EcosystemDefaults for generation
  EcosystemSettings   — per-ecosystem fields passed to the generator
  dependabotUpdate    — single updates[] entry (internal)
```

## Decision points and side effects

### Configuration loading (layered merge)

The `LoadConfig` function loads TOML files from three filesystem locations in order, each overriding the previous:

| Priority | Path                                                | Scope        |
|----------|-----------------------------------------------------|--------------|
| Lowest   | `/etc/dependabot-generator/config.toml`             | Organization |
| Middle   | `$XDG_CONFIG_HOME/dependabot-generator/config.toml` | User         |
| Highest  | `<scan-path>/.depgen.toml`                          | Repository   |

After file-based resolution, environment variables and CLI flags override in ascending priority. Non-existent files are silently skipped. Files that exist but cannot be read or parsed return errors immediately.

**Merge semantics**:

* `header`: last non-empty value wins (simple replacement).
* `ignore-dirs`: last non-empty slice wins (full replacement, not append).
* `ecosystems`: per-key merge. Each ecosystem key in a higher-priority file replaces that key entirely, but other keys from lower-priority files are preserved.

**Built-in defaults** (applied when no config overrides):

* `ignore-dirs`: `["node_modules", ".venv", "venv", "vendor", ".*"]`
* `ecosystems._default`: monthly schedule, deny insecure code execution, 7-day cooldown, batch all patterns.
* `header`: URL to GitHub's Dependabot configuration docs.

### Path validation (3-step)

The `Scan` function validates the root path in three distinct steps, each producing a different sentinel error:

| Step              | Check                 | Sentinel             | User guidance                  |
|-------------------|-----------------------|----------------------|--------------------------------|
| `os.Stat`         | Path exists           | `ErrRootNotExist`    | "directory not found"          |
| `info.IsDir()`    | Path is a directory   | `ErrRootNotDir`      | "expected directory, got file" |
| `os.Open`/`Close` | Directory is readable | `ErrRootNotReadable` | "check permissions"            |

### Directory exclusion

During the walk, each directory's base name is tested against the `ignoreDirs` slice using `filepath.Match` semantics. The root directory (`.`) is never excluded. Matching directories are skipped entirely via `fs.SkipDir`, preventing descent into their subtrees.

### Rule evaluation

For each directory in the tree, every rule in `ecosystemRules` is evaluated:

```text
for each rule:
  for each AND-group (outer = OR):
    for each pattern (inner = AND):
      → fileglob.Glob(fullPattern, fileglob.MaybeRootFS)
      → filterMatchesByDepth() to prevent false positives
    if ALL patterns matched → rule matches (short-circuit OR)
  if NO group matched → rule does not match
```

**Side effects**: None. Rule evaluation is pure. It reads the filesystem but does not modify any state.

### Depth filtering

When `fileglob` encounters a pattern that resolves to a directory name, it can recurse into that directory and return files from within. The `filterMatchesByDepth` function retains only matches whose relative path segment count equals the pattern's segment count.

**Exception**: Patterns containing `**` bypass this filter because recursive matching is intentional.

### Precedence resolution

A post-processing pass suppresses generic ecosystems when a more specific tool is detected in the same directory:

| Winner     | Loser       | Rationale                      |
|------------|-------------|--------------------------------|
| `bun`      | `npm`       | Bun wraps npm's `package.json` |
| `opentofu` | `terraform` | OpenTofu is a drop-in fork     |
| `uv`       | `pip`       | uv wraps pip's manifests       |

**Algorithm**: Two-pass approach. First pass builds a map of directories where each winner was found. Second pass filters out losers from those same directories. Two passes are necessary because a winner and loser may be discovered in either order.

### YAML generation

**Sorting**: Results are copied and sorted before encoding (directory ascending, ecosystem ascending within each directory). This guarantees deterministic output.

**Extra fields**: Each update entry looks up its ecosystem in `EcosystemDefaults`. If no specific override exists, it falls back to the `_default` key. The fields map uses dotted paths (`schedule.interval`) that are expanded into nested YAML structures.

**Node tree construction**: The generator builds a `yaml.Node` tree manually rather than relying on struct marshaling. This allows inserting arbitrary extra fields per update entry in a controlled, deterministic key order (package-ecosystem, directory, then extra fields sorted alphabetically).

**Header comment**: When `CommentText` is non-empty, `FormatComment` is called to convert raw text into `#`-prefixed YAML comment lines, which are inserted between the `---` separator and the YAML body.

**Side effects**: None. Generates a string; does not write to disk.

### Header file validation

When `--header-file` is specified, the `run` command performs a two-phase approach:

1. Attempt to read the file (`readHeaderFile`). Returns content on success, empty string on failure.
2. If empty, call `validateHeaderFilePath` to determine the specific failure mode (invalid path, not found, not readable) and return the appropriate sentinel error.

This separation means the happy path (file reads successfully) avoids redundant stat calls, while the error path provides precise diagnostics.

### Error mapping

Errors from `lib/config` are translated at the CLI boundary via `mapConfigError`, which wraps `config.ErrConfigParse` and `config.ErrConfigRead` into their CLI-layer equivalents (`ErrConfigSyntax`, `ErrConfigNotReadable`). This decouples the config package's error semantics from the user-facing error messages.

## Risks, gaps, and follow-up inspections

### No merge with existing configuration

The tool always generates a fresh `dependabot.yml`. It does not read an existing file, meaning customizations (schedules, reviewers, labels, ignore rules, groups) are lost on regeneration.

* **Impact**: Users who customize their Dependabot config cannot safely re-run the tool without losing manual changes.
* **Mitigation**: Users must manually merge or use the tool only for initial generation.

### Glob library edge cases

`fileglob` can produce unexpected results when:

* A directory name exactly matches a non-glob pattern (mitigated by `filterMatchesByDepth`).
* Symlinks create cycles or cross filesystem boundaries.
* Patterns with special characters interact with OS-specific path rules.
* **Impact**: Potential false positives or missed detections.
* **Follow-up**: Audit symlink handling behavior in the `fileglob` library.

### Ecosystem rule staleness

The 32 rules in `rules.go` are derived from Dependabot's source code. Dependabot may add new ecosystems, deprecate existing ones, or change detection patterns without notice.

* **Impact**: Generated configs may miss new ecosystems or include deprecated ones.
* **Mitigation**: The `docs/ecosystem.md` file documents the mapping. Periodic review against Dependabot's upstream source is needed.

### No output validation

The tool does not validate the generated YAML against GitHub's Dependabot schema. Malformed output (from bugs in rule matching or encoding) would only be caught when GitHub rejects the file.

* **Impact**: Silent failures. The user commits an invalid config.
* **Follow-up**: Consider adding schema validation as a post-generation step.

### Config ignore-dirs replacement semantics

When a higher-priority config file specifies `ignore-dirs`, it _replaces_ the entire list rather than appending to the lower-priority list. This means a local `.depgen.toml` that adds one custom pattern must also re-declare all the built-in defaults.

* **Impact**: Users may accidentally remove default excludes like `node_modules` or `vendor`.
* **Follow-up**: Consider a merge strategy or an `ignore-dirs-append` key.

### Header size limit

The 8 KiB cap (`maxHeaderSize = 8192`) on resolved header text is an arbitrary guard to prevent accidental inclusion of large files. There is no override mechanism.

* **Impact**: Users with large header requirements get a hard error.
* **Follow-up**: Evaluate whether a higher limit or a `--force` flag is warranted.

### Ecosystem defaults full-key replacement

When a specific ecosystem key exists in `EcosystemDefaults`, it replaces that ecosystem's settings entirely. There is no merge between `_default` and a specific ecosystem key.

* **Impact**: Users overriding one field for a specific ecosystem must redeclare all desired fields, not just the delta.
* **Follow-up**: Consider deep-merging specific ecosystem settings with `_default`.

## Design rationale

### Convention over configuration

The tool requires zero setup by default. Point it at a directory and it produces output. The optional `.depgen.toml` adds flexibility without breaking the zero-config path. This trade favors simplicity and reliability for the common case.

### Data-driven architecture

The entire detection engine is driven by the `ecosystemRules` table. Benefits:

* **Extensibility**: Adding a new ecosystem is a one-line struct addition.
* **Auditability**: The full detection surface is visible in a single file.
* **Testability**: Rules can be tested in isolation without filesystem setup.
* **Separation of concerns**: Detection logic (`evaluateRule`) never changes when rules change.

### Explicit sentinel errors

Each failure mode has a distinct sentinel error type. This enables the CLI to present targeted user guidance rather than generic "scan failed" messages. The sentinels span both packages:

* Scanner: `ErrRootNotExist`, `ErrRootNotDir`, `ErrRootNotReadable`, `ErrGlobEval`, `ErrYAMLMarshal`
* Config: `ErrConfigParse`, `ErrConfigRead`
* CLI: `ErrFlagsMutuallyExclusive`, `ErrHeaderFilePathInvalid`, `ErrHeaderFileNotFound`, `ErrHeaderFileNotReadable`, `ErrHeaderTooLarge`, `ErrConfigSyntax`, `ErrConfigNotReadable`, `ErrIgnorePatternInvalid`

### Layered configuration

The three-tier file lookup (global, user, local) mirrors conventions from tools like Git, rustfmt, and EditorConfig. This lets organizations set defaults centrally while repositories opt into specific overrides.

### Scanner isolation from CLI

The `lib/scanner` package has no dependency on Cobra, Fang, or any terminal library. This means:

* Tests run without CLI bootstrapping.
* The scanner can be imported as a library by other tools.
* CI scripts could call `Scan` and `Generate` directly via a thin wrapper.

### Config isolation from scanner

The `lib/config` package handles TOML parsing and merge logic without knowing about scanning or YAML generation. The `cmd` layer is the only place that connects config output to scanner input, keeping both libraries independently testable and reusable.

### Deterministic output

Sorting results before YAML encoding and using a Node tree with sorted keys guarantees that identical inputs always produce identical output. This is critical because:

* Users commit `dependabot.yml` to version control.
* PRs show diffs. Non-deterministic ordering creates review noise.
* CI pipelines can compare current vs generated to detect drift.

### Unix philosophy (stdout output)

Writing to stdout rather than directly to a file respects the Unix composability model:

* Pipe to `diff` to see what changed.
* Redirect to any path the user chooses.
* Combine with other tools in shell scripts.
* Preview output before committing.

### Property-based testing

The test suite uses `pgregory.net/rapid` for property-based testing alongside traditional unit tests and fixture-based integration tests. This provides:

* Coverage of edge cases that hand-written tests miss.
* Verification of invariants (sort stability, precedence correctness, round-trip encoding) across random inputs.
* Confidence that the system handles arbitrary ecosystem combinations correctly.
