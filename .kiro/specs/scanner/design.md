# Design Document

## Overview

This design describes the `lib/scanner` package which provides two public functions: `Scan` and `Generate`. `Scan` recursively walks a directory tree, evaluates a table of ecosystem detection rules against each directory, and returns a slice of `ScanResult` structs pairing directory paths with ecosystem identifiers. `Generate` accepts that slice and produces a Dependabot v2 YAML configuration string.

The architecture is deliberately simple: a single package with no external state, no concurrency, and deterministic output for a given input. The ecosystem rule table is defined as package-level data — adding a new ecosystem means adding a struct literal, not changing control flow.

## Architecture

```mermaid
flowchart TD
    CLI[cmd/run.go] -->|path string| Scan
    Scan -->|[]ScanResult, error| CLI
    CLI -->|[]ScanResult| Generate
    Generate -->|string, error| CLI

    subgraph lib/scanner
        Scan --> RuleTable[ecosystemRules]
        Scan --> Walker[fs.WalkDir]
        Walker --> Matcher[fileglob.Glob]
        Generate --> Sorter[slices.SortFunc]
        Generate --> Marshaler[yaml.Marshal]
    end
```

The CLI command (`cmd/run.go`) orchestrates the two-phase workflow:

1. Call `Scan(path)` to get detection results.
2. Call `Generate(results)` to get YAML output.
3. Write the YAML string to stdout or a file.

## Components and interfaces

### Types

```go
// EcosystemRule defines detection logic for a single ecosystem.
type EcosystemRule struct {
    Identifier string
    // Files contains OR-groups. Each inner slice is an AND-group:
    // all patterns in an AND-group must match for that group to
    // succeed. The rule matches if ANY AND-group succeeds.
    Files [][]string
}

// ScanResult pairs a directory (relative to root) with a matched
// ecosystem identifier.
type ScanResult struct {
    Directory string
    Ecosystem string
}
```

### Functions

```go
// Scan walks the directory tree rooted at path and returns all
// ecosystem matches found.
func Scan(path string) ([]ScanResult, error)

// Generate produces a Dependabot v2 YAML configuration string
// from the provided scan results.
func Generate(results []ScanResult) (string, error)
```

### Internal helpers

* `evaluateRule(fsys fs.FS, dir string, rule EcosystemRule) bool` — evaluates a single rule against a directory using `fileglob.Glob` with `fileglob.WithFs`.
* `resolvePrecedence(results []ScanResult) []ScanResult` — applies the OpenTofu-over-Terraform precedence rule per directory.

### Package-level data

* `ecosystemRules []EcosystemRule` — the full table of 33 ecosystem detection rules, defined as a `var` block of struct literals.

## Data models

### EcosystemRule encoding

Each ecosystem rule maps its identifier to a set of file patterns using a two-dimensional slice (`[][]string`). The outer slice represents OR-alternatives. Each inner slice represents an AND-group where all patterns must match within the same directory.

Examples:

| Ecosystem | Identifier  | Files encoding                                    |
|-----------|-------------|---------------------------------------------------|
| Bazel     | `bazel`     | `[["BUILD.bazel"], ["BUILD"]]`                    |
| Bun       | `bun`       | `[["bunfig.toml"], ["package.json", "bun.lock"]]` |
| Cargo     | `cargo`     | `[["Cargo.toml"]]`                                |
| OpenTofu  | `opentofu`  | `[[".terraform.lock.hcl", "*.tofu"]]`             |
| Terraform | `terraform` | `[[".terraform.lock.hcl", "*.tf"]]`               |

### ScanResult

A flat struct with two string fields. A slice of these is the exchange format between `Scan` and `Generate`. The `Directory` field uses forward-slash prefix notation relative to the scan root (e.g., `/`, `/subdir/nested`).

### YAML output structure

The generated YAML follows the Dependabot v2 schema:

```yaml
---
version: 2
updates:
  - package-ecosystem: gomod
    directory: /
  - package-ecosystem: github-actions
    directory: /
```

Each entry contains only `package-ecosystem` and `directory`. Additional fields (schedule, reviewers, etc.) are left for the user to add manually.

## Correctness properties

_A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees._

### Property 1: Serialization round-trip

_For any_ valid slice of `ScanResult`, generating YAML via `Generate` and parsing it back into a structured form SHALL yield the same set of directory-ecosystem pairs as the original input (after accounting for sort order).

**Validates: Requirements 8.1, 8.2.**

### Property 2: One entry per ScanResult

_For any_ valid slice of `ScanResult` of length N, the `updates` array in the generated YAML SHALL contain exactly N entries.

**Validates: Requirements 8.1.**

### Property 3: Output is sorted

_For any_ valid slice of `ScanResult`, the `updates` array in the generated YAML SHALL be sorted first by `directory` ascending, then by `package-ecosystem` ascending within the same directory.

**Validates: Requirements 5.3.**

### Property 4: Empty input produces valid empty document

_For any_ empty slice of `ScanResult`, `Generate` SHALL produce a valid YAML document with `version: 2` and an empty `updates` array, starting with a `---` document separator.

**Validates: Requirements 5.4, 5.5.**

### Property 5: Compound AND-matching requires all patterns

_For any_ directory containing a strict subset of the patterns in an AND-group for an ecosystem rule, that ecosystem SHALL NOT appear in the scan results for that directory.

**Validates: Requirements 3.1.**

### Property 6: OR-matching succeeds on any alternative

_For any_ directory containing files matching at least one complete AND-group of an ecosystem rule, that ecosystem SHALL appear in the scan results for that directory.

**Validates: Requirements 3.2.**

### Property 7: OpenTofu precedence over Terraform

_For any_ directory containing `.terraform.lock.hcl` along with both `*.tofu` and `*.tf` files, the scan results SHALL contain `opentofu` but NOT `terraform` for that directory.

**Validates: Requirements 4.1.**

### Property 8: Multiple ecosystems per directory

_For any_ directory matching N distinct ecosystem rules (after precedence resolution), the scan results SHALL contain exactly N `ScanResult` entries for that directory.

**Validates: Requirements 2.3.**

### Property 9: Relative path format

_For any_ `ScanResult` produced by `Scan`, the `Directory` field SHALL start with a `/` and SHALL use forward-slash separators, representing the path relative to the provided root.

**Validates: Requirements 2.8.**

## Error handling

| Condition                      | Behavior                                                                                |
|--------------------------------|-----------------------------------------------------------------------------------------|
| Root path does not exist       | `Scan` returns error: `"root path does not exist: <path>"`                              |
| Root path is not a directory   | `Scan` returns error: `"root path is not a directory: <path>"`                          |
| Root path is not readable      | `Scan` returns error: `"root path is not accessible: <path>: <os error>"`               |
| Glob evaluation fails mid-walk | `Scan` returns error wrapped with context: `"glob evaluation failed in <dir>: <inner>"` |
| YAML marshaling fails          | `Generate` returns error: `"failed to marshal YAML: <inner>"`                           |
| Empty scan results             | `Generate` succeeds with empty `updates` array                                          |
| Nil slice passed to Generate   | Treated as empty; `Generate` succeeds                                                   |

All errors are wrapped with `fmt.Errorf` using `%w` to support `errors.Is` and `errors.As` by callers. Sentinel errors are defined in the scanner package for testability:

```go
var (
    ErrRootNotExist    = errors.New("root path does not exist")
    ErrRootNotDir      = errors.New("root path is not a directory")
    ErrRootNotReadable = errors.New("root path is not accessible")
    ErrGlobEval        = errors.New("glob evaluation failed")
    ErrYAMLMarshal     = errors.New("failed to marshal YAML")
)
```

## Testing strategy

### Property-based tests

The feature's core logic — pattern matching, result collection, YAML generation, and round-tripping — is well suited to property-based testing. The input space (directory structures, file names, ecosystem combinations) is large and varied, making PBT effective at finding edge cases.

**Library:** [`pgregory.net/rapid`](https://github.com/flyingmutant/rapid) — a Go property-based testing library with built-in shrinking.

**Configuration:**

* Minimum 100 iterations per property test.
* Each test tagged with a comment referencing the design property.
* Tag format: `// Feature: scanner, Property {N}: {title}`

**Properties to implement:**

1. Serialization round-trip (Property 1)
2. One entry per ScanResult (Property 2)
3. Output is sorted (Property 3)
4. Empty input produces valid document (Property 4)
5. Compound AND-matching requires all patterns (Property 5)
6. OR-matching succeeds on any alternative (Property 6)
7. OpenTofu precedence (Property 7)
8. Multiple ecosystems per directory (Property 8)
9. Relative path format (Property 9)

### Unit tests

Unit tests complement PBT by covering specific examples and integration points:

* Each of the 33 ecosystem rules resolves correctly against the existing `src/<ecosystem>/` fixture directories.
* Error cases: non-existent path, file-as-path, unreadable directory.
* Precedence: OpenTofu vs Terraform specific directory layouts.
* Edge cases: deeply nested directories, symlinks (if followed), empty directories.

### Test fixtures

The existing `src/` directory contains 32 fixture directories (one per ecosystem, some with multiple sub-variants like `bazel/a` and `bazel/b`). These serve as integration fixtures for verifying the full `Scan` function against real directory structures.

### Running tests

```bash
go test ./lib/scanner/ -count=1 -v
```
