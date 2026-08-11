# Quick Flow Summary

## Entrypoint

The application starts at `main.go`, which does exactly one thing: call `cmd.Execute()`. This keeps the entrypoint trivial and pushes all CLI wiring into the `cmd` package where it can be tested and extended independently.

```text
main.go → cmd.Execute() → fang.Execute(ctx, rootCmd) → Cobra dispatch
```

## Primary flow

The `run` command is the only user-facing action. It implements a two-stage pipeline:

```text
1. Parse CLI args (default path: ".")
2. scanner.Scan(logger, path)        → walk directory tree, match ecosystem rules
3. scanner.Generate(logger, results) → sort results, encode to YAML
4. Print YAML to stdout
```

The user captures output via shell redirection:

```bash
dependabot-generator run . > .github/dependabot.yml
```

### Scan stage

* Validates the root path (exists, is a directory, is readable).
* Resolves to an absolute path (required by the glob library).
* Walks the directory tree using `fs.WalkDir` over `os.DirFS`.
* For each directory, evaluates all 32 ecosystem rules from the rules table.
* Each rule uses OR-of-AND pattern matching: the outer slice is OR (any group matching is sufficient), each inner slice is AND (all patterns must match).
* After the walk, applies precedence rules to suppress generic ecosystems when a more specific tool is detected (e.g., `bun` suppresses `npm`, `uv` suppresses `pip`).

### Generate stage

* Copies and sorts results deterministically (directory ascending, then ecosystem ascending).
* Maps results to Dependabot v2 YAML structs.
* Encodes via `gopkg.in/yaml.v3` with 2-space indentation.
* Prepends the `---` YAML document separator.

## Module roles

| Module                   | Responsibility                                               |
|--------------------------|--------------------------------------------------------------|
| `main.go`                | Trivial entrypoint — calls `cmd.Execute()`                   |
| `cmd/`                   | Cobra command definitions, flag registration, CLI lifecycle  |
| `cmd/root.go`            | Root command (help-only), verbose flag, `Execute()` function |
| `cmd/run.go`             | Primary command — orchestrates Scan → Generate → stdout      |
| `cmd/version.go`         | Version subcommand (delegated to shared cli-helpers)         |
| `lib/scanner/`           | Core detection and generation logic, decoupled from CLI      |
| `lib/scanner/scanner.go` | `Scan()`, `Generate()`, pattern evaluation, precedence       |
| `lib/scanner/rules.go`   | Data-driven ecosystem detection table (32 rules)             |
| `src/`                   | Test fixtures — one directory per ecosystem with variants    |
| `docs/`                  | Project documentation                                        |

## Design decisions

### Why a data-driven rules table?

Adding a new ecosystem requires only a single struct entry in `rules.go`. No procedural code changes, no new functions, no conditional branches. The table is sorted alphabetically for human readability — runtime evaluation checks every rule independently regardless of order.

### Why OR-of-AND pattern matching?

Ecosystem detection often requires expressing "file A exists" OR "files B AND C both exist." The two-level `[][]string` encoding covers every real Dependabot detection pattern without needing a custom expression language or DSL.

### Why post-processing precedence instead of inline checks?

A winner (e.g., `bun`) and its loser (e.g., `npm`) may be discovered in any order during the walk. Resolving precedence after the full walk guarantees correctness regardless of discovery order and keeps the walk logic simple.

### Why stdout-only output?

Following Unix conventions, the tool writes to stdout so users can pipe, redirect, or compose with other tools. No `--output` flag is needed — shell redirection handles all cases.

### Why no user configuration file?

The tool is purely convention-based. Detection is driven entirely by the presence of well-known files in the repository tree. This eliminates configuration drift and means the tool works correctly on any repository without setup.

### Why is the scanner a separate package?

Decoupling detection logic from CLI concerns (flags, exit codes, terminal formatting) allows the scanner to be invoked programmatically — from tests, CI tooling, or potential future library consumers — without pulling in Cobra or terminal dependencies.

## Risks and unknowns

* **Glob library behavior**: `fileglob` can recurse into directories whose names match non-glob patterns (e.g., a directory named `rust-toolchain`). The `filterMatchesByDepth` function mitigates this, but edge cases with unusual filesystem layouts may still produce false positives.
* **No ignore patterns**: The scanner walks the entire tree with no mechanism to exclude directories (e.g., `vendor/`, `node_modules/`). Large repositories with vendored dependencies may produce unwanted matches.
* **Ecosystem rule maintenance**: The rules table must be manually kept in sync with Dependabot's evolving ecosystem support. If Dependabot adds a new ecosystem or changes detection patterns, the tool needs a corresponding update.
* **No merge with existing config**: The tool generates a fresh `dependabot.yml` every time. It does not read or merge with an existing configuration, meaning any manual customizations (schedules, reviewers, ignore rules) would be overwritten.
