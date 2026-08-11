# Implementation Plan: Scanner and Generator

## Overview

Implement the `lib/scanner` package providing two public functions: `Scan` (directory walking and ecosystem detection) and `Generate` (Dependabot v2 YAML output). The implementation follows a bottom-up approach: define types and rule data first, implement pattern matching, then scanning, then generation, and finally wire into the CLI.

## Tasks

* [x] 1. Define types, sentinel errors, and ecosystem rule table
  * [x] 1.1 Define types and sentinel errors in `lib/scanner/scanner.go`
    * Define `EcosystemRule` struct with `Identifier string` and `Files [][]string`
    * Define `ScanResult` struct with `Directory string` and `Ecosystem string`
    * Define sentinel errors: `ErrRootNotExist`, `ErrRootNotDir`, `ErrRootNotReadable`, `ErrGlobEval`, `ErrYAMLMarshal`
    * _Requirements: 1.1, 2.7, 6.1, 6.2, 7.1, 7.2_

  * [x] 1.2 Define the ecosystem rule table in a new `lib/scanner/rules.go` file
    * Create a package-level `var ecosystemRules []EcosystemRule` containing all 33 ecosystem rules
    * Encode each rule using the `[][]string` OR-of-AND pattern from the design
    * Reference the ecosystem table in `docs/ecosystem.md` for exact patterns
    * _Requirements: 1.1, 1.2, 1.3, 1.4_

* [x] 2. Implement the Scan function
  * [x] 2.1 Implement root path validation in `lib/scanner/scanner.go`
    * Check path exists, is a directory, and is readable before walking
    * Return wrapped sentinel errors with context on failure
    * _Requirements: 2.5, 2.6, 6.2_

  * [x] 2.2 Implement directory walking and rule evaluation in `lib/scanner/scanner.go`
    * Use `fs.WalkDir` to traverse the directory tree
    * For each directory, evaluate all ecosystem rules using `fileglob.Glob` with `fileglob.MaybeRootFS`
    * Implement `evaluateRule` helper that evaluates AND/OR logic for a single rule against a directory
    * Record each match as a `ScanResult` with directory path in forward-slash relative notation
    * _Requirements: 2.1, 2.2, 2.3, 2.4, 2.8, 3.1, 3.2, 3.3_

  * [x] 2.3 Implement precedence resolution in `lib/scanner/scanner.go`
    * Implement `resolvePrecedence` helper that applies OpenTofu-over-Terraform rule
    * When a directory has both `opentofu` and `terraform` results, remove the `terraform` entry
    * Call precedence resolution after collecting all results
    * _Requirements: 4.1, 4.2, 4.3_

  * [x] 2.4 Write property tests for compound AND-matching (Property 5)
    * **Property 5: Compound AND-matching requires all patterns**
    * **Validates: Requirements 3.1**

  * [x] 2.5 Write property tests for OR-matching (Property 6)
    * **Property 6: OR-matching succeeds on any alternative**
    * **Validates: Requirements 3.2**

  * [x] 2.6 Write property tests for OpenTofu precedence (Property 7)
    * **Property 7: OpenTofu precedence over Terraform**
    * **Validates: Requirements 4.1**

  * [x] 2.7 Write property tests for multiple ecosystems per directory (Property 8)
    * **Property 8: Multiple ecosystems per directory**
    * **Validates: Requirements 2.3**

  * [x] 2.8 Write property tests for relative path format (Property 9)
    * **Property 9: Relative path format**
    * **Validates: Requirements 2.8**

* [x] 3. Checkpoint - Ensure Scan function works
  * Ensure all tests pass, ask the user if questions arise.

* [x] 4. Implement the Generate function
  * [x] 4.1 Implement YAML generation in `lib/scanner/scanner.go`
    * Accept `[]ScanResult` and return `(string, error)`
    * Sort results by directory ascending, then by ecosystem ascending using `slices.SortFunc`
    * Marshal to YAML with `version: 2` and `updates` array containing `package-ecosystem` and `directory` fields
    * Prepend `---` document separator
    * Handle nil/empty input by producing valid empty document
    * Return wrapped `ErrYAMLMarshal` on marshaling failure
    * _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 7.1, 7.2_

  * [x] 4.2 Write property test for serialization round-trip (Property 1)
    * **Property 1: Serialization round-trip**
    * **Validates: Requirements 8.1, 8.2**

  * [x] 4.3 Write property test for one entry per ScanResult (Property 2)
    * **Property 2: One entry per ScanResult**
    * **Validates: Requirements 8.1**

  * [x] 4.4 Write property test for sorted output (Property 3)
    * **Property 3: Output is sorted**
    * **Validates: Requirements 5.3**

  * [x] 4.5 Write property test for empty input (Property 4)
    * **Property 4: Empty input produces valid empty document**
    * **Validates: Requirements 5.4, 5.5**

* [x] 5. Checkpoint - Ensure Generate function works
  * Ensure all tests pass, ask the user if questions arise.

* [x] 6. Write unit tests using fixture directories
  * [x] 6.1 Write unit tests for `Scan` against `src/` fixture directories in `lib/scanner/scanner_test.go`
    * Test each of the 33 ecosystem rules resolves correctly against `src/<ecosystem>/a` fixtures
    * Test error cases: non-existent path, file-as-path, unreadable directory
    * Test precedence: OpenTofu vs Terraform specific directory layouts
    * Test compound pattern matching (Bun AND-group via `src/bun/b`)
    * Test multiple ecosystems in same directory produce separate entries
    * Test relative path format for nested directories
    * _Requirements: 1.3, 2.1, 2.2, 2.3, 2.5, 2.6, 2.8, 3.1, 3.2, 4.1, 4.2, 4.3_

  * [x] 6.2 Write unit tests for `Generate` in `lib/scanner/generate_test.go`
    * Test YAML output structure: `---` prefix, `version: 2`, sorted `updates` array
    * Test empty and nil results produce valid empty document
    * Test multiple ecosystems in same directory produce multiple entries
    * Test document separator is exactly `---\n`
    * _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 8.1, 8.2_

* [x] 7. Wire into CLI and finalize
  * [x] 7.1 Update `cmd/run.go` to call `Scan` and `Generate`
    * Uncomment and update the existing commented-out code in `RunE`
    * Determine path from args (default to `"."` when no arg provided)
    * Call `scanner.Scan(path)` and return wrapped error on failure
    * Call `scanner.Generate(results)` and return wrapped error on failure
    * Print the generated YAML string to stdout using `strings.Builder` and `fmt.Fprint`
    * Add import for `"fmt"` and the `lib/scanner` package
    * _Requirements: 6.1, 7.1_

* [x] 8. Final checkpoint - Ensure all tests pass
  * Ensure all tests pass, ask the user if questions arise.

## Notes

* Each task references specific requirements for traceability
* Checkpoints ensure incremental validation
* Property tests use `pgregory.net/rapid` with minimum 100 iterations per property
* Unit tests use the existing `src/` fixture directories for integration-style verification
* All code must comply with the project's `golangci-lint` configuration (zero diagnostics policy)
* The `Generate` unit tests live in `lib/scanner/generate_test.go` (separate from `scanner_test.go`)

## Task dependency graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2"] },
    { "id": 2, "tasks": ["2.1"] },
    { "id": 3, "tasks": ["2.2"] },
    { "id": 4, "tasks": ["2.3"] },
    { "id": 5, "tasks": ["2.4", "2.5", "2.6", "2.7", "2.8"] },
    { "id": 6, "tasks": ["4.1"] },
    { "id": 7, "tasks": ["4.2", "4.3", "4.4", "4.5"] },
    { "id": 8, "tasks": ["6.1", "6.2"] },
    { "id": 9, "tasks": ["7.1"] }
  ]
}
```
