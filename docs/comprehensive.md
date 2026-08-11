# Deep Architecture Audit

## Entry points

The application has a single binary entry point:

| File      | Function | Role                                   |
|-----------|----------|----------------------------------------|
| `main.go` | `main()` | Calls `cmd.Execute()` and nothing else |

The `main.go` file is intentionally minimal. All logic lives in the `cmd` and `lib/scanner` packages so that the CLI is testable without executing a binary.

The scanner package exposes two public entry points for programmatic use:

| Function                            | Purpose                                     |
|-------------------------------------|---------------------------------------------|
| `scanner.Scan(logger, p)`           | Walk a directory tree and detect ecosystems |
| `scanner.Generate(logger, results)` | Convert scan results to YAML                |

## CLI startup and initialization flow

### Package initialization order

Go guarantees `init()` functions run before `main()`. The CLI's startup sequence is:

```text
1. cmd/root.go init()    → registers --verbose/-v persistent flag
2. cmd/run.go init()     → registers runCmd as a subcommand of rootCmd
3. cmd/version.go init() → registers versionCmd as a subcommand of rootCmd
4. main.go main()        → calls cmd.Execute()
```

### Execute function (`cmd/root.go`)

```text
cmd.Execute()
  → fang.Execute(context.Background(), rootCmd)
    → detects terminal color capabilities
    → calls rootCmd.Execute() (Cobra dispatch)
  → on error: os.Exit(1)
```

The `fang` wrapper ensures the CLI renders correctly on both interactive terminals and piped output. Cobra handles argument parsing, flag binding, help generation, and subcommand dispatch internally.

### Root command

`rootCmd` has no `RunE` function. Invoking the binary without a subcommand falls through to Cobra's automatic help display. This is intentional — the tool requires an explicit subcommand (`run` or `version`).

### Shared state

The `cmd` package declares three package-level variables shared across commands:

| Variable   | Type              | Purpose                                   |
|------------|-------------------|-------------------------------------------|
| `fVerbose` | `int`             | Count flag for progressive verbosity      |
| `ctx`      | `context.Context` | Background context for structured logging |
| `logger`   | `*slog.Logger`    | Initialized in PersistentPreRunE          |

The `fVerbose` count supports `-v`, `-vv`, and `-vvv` levels, matching conventions from tools like `ssh` and `curl`.

## Command-specific flows

### `run` command (`cmd/run.go`)

This is the primary user-facing command. Its lifecycle:

```text
PersistentPreRunE:
  → logger = logutils.GetDefaultLogger(fVerbose)

RunE:
  → path = args[0] or "." (default)
  → results, err = scanner.Scan(logger, path)
  → output, err = scanner.Generate(logger, results)
  → fmt.Fprint(os.Stdout, output)
```

**Arguments**: Accepts 0 or 1 positional args (`cobra.RangeArgs(0, 1)`). Defaults to `"."` when no argument is provided.

**Error handling**: Both `Scan` and `Generate` errors are wrapped with user-facing descriptions before returning to Cobra, which prints the error and triggers the `os.Exit(1)` in `Execute()`.

**Output strategy**: YAML goes to stdout so users can pipe or redirect. No file writing logic exists in the command itself.

### `version` command (`cmd/version.go`)

Entirely delegated to `clihelpers.VersionScreen()` from the shared `go.nwlabs.dev/cli-helpers/v2` package. No custom logic — all Northwood Labs CLI tools share a consistent version display format.

## File and module responsibilities

### `cmd/` package

| File         | Exports           | Responsibility                       |
|--------------|-------------------|--------------------------------------|
| `doc.go`     | (package comment) | Documents the cmd package's purpose  |
| `root.go`    | `Execute()`       | Root command, flags, Cobra lifecycle |
| `run.go`     | (none)            | Run subcommand orchestration         |
| `version.go` | (none)            | Version subcommand registration      |

### `lib/scanner/` package

| File                       | Exports / Key Symbols     | Responsibility                |
|----------------------------|---------------------------|-------------------------------|
| `doc.go`                   | (package comment)         | Documents the scanner package |
| `scanner.go`               | `Scan`, `Generate`, types | Core logic and YAML encoding  |
| `rules.go`                 | `ecosystemRules`          | Detection rule table          |
| `scanner_test.go`          | (tests)                   | Unit and integration tests    |
| `scanner_property_test.go` | (tests)                   | Property-based fuzz tests     |
| `generate_test.go`         | (tests)                   | YAML generation tests         |

### `src/` directory (test fixtures)

Contains 32 directories — one per ecosystem — each with filesystem structures that match the corresponding detection rule. Used by integration tests to verify end-to-end scanning against real file layouts.

### Key types in `lib/scanner/`

```text
EcosystemRule        — detection pattern: Identifier + Files (OR-of-AND groups)
ScanResult           — detected ecosystem entry: Directory + Ecosystem
precedenceRule       — winner/loser relationship between ecosystems
dependabotConfig     — top-level Dependabot YAML structure (internal)
dependabotUpdate     — single updates[] entry (internal)
```

## Decision points and side effects

### Path validation (3-step)

The `Scan` function validates the root path in three distinct steps, each producing a different sentinel error:

| Step              | Check                 | Sentinel             | User guidance                  |
|-------------------|-----------------------|----------------------|--------------------------------|
| `os.Stat`         | Path exists           | `ErrRootNotExist`    | "directory not found"          |
| `info.IsDir()`    | Path is a directory   | `ErrRootNotDir`      | "expected directory, got file" |
| `os.Open`/`Close` | Directory is readable | `ErrRootNotReadable` | "check permissions"            |

### Rule evaluation (`evaluateRule`)

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

**Side effects**: None. Rule evaluation is pure — it reads the filesystem but does not modify any state.

### Precedence resolution (`resolvePrecedence`)

A post-processing pass that suppresses generic ecosystems when a more specific tool is detected in the same directory:

| Winner     | Loser       | Rationale                      |
|------------|-------------|--------------------------------|
| `bun`      | `npm`       | Bun wraps npm's `package.json` |
| `opentofu` | `terraform` | OpenTofu is a drop-in fork     |
| `uv`       | `pip`       | uv wraps pip's manifests       |

**Algorithm**: Two-pass approach.

1. First pass builds a map of directories where each winner was found.
2. Second pass filters out losers from those same directories.

Two passes are necessary because a winner and loser may be discovered in either order depending on filesystem layout and rule-table ordering.

### YAML generation (`Generate`)

**Decision**: Sort results before encoding. This guarantees deterministic output so that identical repository states always produce identical YAML. Users commit this file and review diffs in PRs — non-deterministic ordering would create noise.

**Sort order**: Directory ascending (primary), ecosystem ascending within each directory (secondary). Uses `slices.SortFunc` with `strings.Compare`.

**Side effects**: None. Generates a string; does not write to disk.

### Depth filtering (`filterMatchesByDepth`)

When `fileglob` encounters a pattern that resolves to a directory name, it can recurse into that directory and return files from within. This function retains only matches whose relative path segment count equals the pattern's segment count.

**Exception**: Patterns containing `**` bypass this filter because recursive matching is intentional.

## Risks, gaps, and follow-up inspections

### No directory exclusion mechanism

The scanner walks the entire tree. There is no `.dependabotignore` or `--exclude` flag. Large repositories with vendored dependencies (`vendor/`, `node_modules/`, `.terraform/`) may produce unwanted matches from third-party code.

* **Impact**: False positives in the generated config.
* **Mitigation**: None currently. Users must manually edit the output.

### No merge with existing configuration

The tool always generates a fresh `dependabot.yml`. It does not read an existing file, meaning customizations (schedules, reviewers, labels, ignore rules, groups) are lost on regeneration.

* **Impact**: Users who customize their Dependabot config cannot safely
re-run the tool without losing manual changes.
* **Mitigation**: Users must manually merge or use the tool only for initial generation.

### Glob library edge cases

`fileglob` can produce unexpected results when:

* A directory name exactly matches a non-glob pattern (mitigated by `filterMatchesByDepth`).
* Symlinks create cycles or cross filesystem boundaries.
* Patterns with special characters interact with OS-specific path rules.

* **Impact**: Potential false positives or missed detections.
* **Follow-up**: Audit symlink handling behavior in the `fileglob` library.

### Ecosystem rule staleness

The 32 rules in `rules.go` are derived from Dependabot's source code (as noted in the file's header comment). Dependabot may add new ecosystems, deprecate existing ones, or change detection patterns without notice.

* **Impact**: Generated configs may miss new ecosystems or include deprecated ones.
* **Mitigation**: The `docs/ecosystem.md` file documents the mapping. Periodic review against Dependabot's upstream source is needed.

### Single-schedule assumption

The generated YAML includes only `package-ecosystem` and `directory` per entry. It omits the required `schedule` field, which Dependabot mandates.

* **Impact**: The generated file may not be immediately valid without adding schedule configuration.
* **Follow-up**: Verify whether Dependabot has a default schedule when none is specified, or whether the tool should emit a default `schedule.interval`.

### No output validation

The tool does not validate the generated YAML against GitHub's Dependabot schema. Malformed output (from bugs in rule matching or encoding) would only be caught when GitHub rejects the file.

* **Impact**: Silent failures — the user commits an invalid config.
* **Follow-up**: Consider adding schema validation as a post-generation step.

## Design rationale

### Convention over configuration

The tool requires zero setup. Point it at a directory and it produces output. This decision trades flexibility (no custom schedules, no ignore patterns) for simplicity and reliability. The tool does one thing well: detect ecosystems and produce a starting-point config.

### Data-driven architecture

The entire detection engine is driven by the `ecosystemRules` table. This has several benefits:

* **Extensibility**: Adding a new ecosystem is a one-line struct addition.
* **Auditability**: The full detection surface is visible in a single file.
* **Testability**: Rules can be tested in isolation without filesystem setup.
* **Separation of concerns**: Detection logic (`evaluateRule`) never changes when rules change.

### Explicit sentinel errors

Each failure mode has a distinct sentinel error type. This enables the CLI to present targeted user guidance rather than generic "scan failed" messages. The sentinels are:

* `ErrRootNotExist` — path doesn't exist
* `ErrRootNotDir` — path is a file, not a directory
* `ErrRootNotReadable` — permission denied
* `ErrGlobEval` — glob engine infrastructure failure
* `ErrYAMLMarshal` — serialization bug

### Scanner isolation from CLI

The `lib/scanner` package has no dependency on Cobra, Fang, or any terminal library. This means:

* Tests run without CLI bootstrapping.
* The scanner can be imported as a library by other tools.
* CI scripts could call `Scan` and `Generate` directly via a thin wrapper.

### Deterministic output

Sorting results before YAML encoding guarantees that identical inputs always produce identical output. This is critical because:

* Users commit `dependabot.yml` to version control.
* PRs show diffs — non-deterministic ordering creates review noise.
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
* Verification of invariants (sort stability, precedence correctness round-trip encoding) across random inputs.
* Confidence that the system handles arbitrary ecosystem combinations correctly.
