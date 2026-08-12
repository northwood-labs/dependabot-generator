# Implementation Plan: Conditional Ecosystem Defaults

## Overview

This plan implements a compiled-in conditional defaults mechanism for the scanner package. The feature adds a new data table (`conditionalDefaults`) that injects ecosystem-specific Dependabot fields during YAML generation, sitting between the builtin `_default` and user `.depgen.toml` overrides in the merge precedence. The implementation is contained entirely within `lib/scanner/` — no config package changes required.

## Tasks

* [x] 1. Create ConditionalDefault type and data table
  * [x] 1.1 Create `lib/scanner/conditional_defaults.go` with type, table, sentinel error, and validation
    * Add Apache 2.0 license header (year 2026, Northwood Labs, LLC)
    * Define `ConditionalDefault` struct with `FieldKey string`, `FieldValue any`, `Ecosystems []string`
    * Define `conditionalDefaults` package-level slice with the initial entry: `insecure-external-code-execution` = `"deny"` for `bundler`, `mix`, `pip`
    * Define `ErrConditionalDefaultInvalid` sentinel error
    * Implement `validateConditionalDefaults()` function that rejects empty Ecosystems, empty FieldKey, and `_default` in Ecosystems list
    * _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 4.4, 4.5, 5.1, 5.4_

* [x] 2. Extract resolveFields helper and integrate conditional defaults into Generate
  * [x] 2.1 Add `resolveFields` helper function in `lib/scanner/scanner.go`
    * Implement the four-layer merge: (1) builtin `_default` from `EcosystemDefaults`, (2) conditional defaults matched by ecosystem, (3) user ecosystem-specific override
    * Use `slices.Contains` for ecosystem membership check
    * Return `map[string]any` merged result
    * _Requirements: 2.1, 2.2, 2.3, 2.6, 3.1, 3.2, 3.3, 3.4, 3.5, 3.6_

  * [x] 2.2 Replace existing field resolution in `Generate` with `resolveFields` call
    * Remove the current two-level lookup loop (ecosystem-specific → `_default`)
    * Replace with: `updates[i].ExtraFields = resolveFields(eco, opts.EcosystemDefaults)`
    * Handle nil `opts` and nil `opts.EcosystemDefaults` gracefully
    * _Requirements: 2.3, 2.4, 2.5, 4.1, 4.2, 4.3, 5.2, 5.3, 6.1, 6.2, 6.3_

* [x] 3. Checkpoint — Verify build and existing tests pass
  * Ensure all tests pass, ask the user if questions arise.

* [x] 4. Unit tests for conditional defaults
  * [x] 4.1 Create `lib/scanner/conditional_defaults_test.go` with unit test scenarios
    * Add Apache 2.0 license header
    * Test: `validateConditionalDefaults` passes for the production table
    * Test: `validateConditionalDefaults` rejects entry with `_default` ecosystem identifier
    * Test: `validateConditionalDefaults` rejects entry with empty Ecosystems slice
    * Test: `validateConditionalDefaults` rejects entry with empty FieldKey
    * Test: bundler gets `insecure-external-code-execution: deny` via `resolveFields`
    * Test: mix gets `insecure-external-code-execution: deny` via `resolveFields`
    * Test: pip gets `insecure-external-code-execution: deny` via `resolveFields`
    * Test: gomod does NOT get `insecure-external-code-execution` field
    * Test: user `.depgen.toml` override for bundler wins over conditional default
    * Test: empty conditional table produces unchanged output (identity behavior)
    * Test: dead ecosystem identifier in table has no effect on output
    * _Requirements: 1.3, 1.4, 2.1, 2.2, 2.4, 2.5, 3.2, 3.6, 4.1, 4.2, 4.3, 4.4_

* [x] 5. Property-based tests for correctness properties
  * [x] 5.1 Write property test for merge precedence
    * **Property 1: Merge precedence**
    * Generate random field keys/values at multiple priority layers; verify highest-priority layer always wins
    * Tag: `Feature: conditional-ecosystem-defaults, Property 1: Merge precedence`
    * **Validates: Requirements 2.3, 2.4, 3.1, 3.2, 3.6, 6.3**

  * [x] 5.2 Write property test for conditional application
    * **Property 2: Conditional application**
    * Generate random ecosystems present in a random conditional defaults table; verify field appears in output unless overridden
    * Tag: `Feature: conditional-ecosystem-defaults, Property 2: Conditional application`
    * **Validates: Requirements 2.1, 5.2, 5.3**

  * [x] 5.3 Write property test for non-interference
    * **Property 3: Non-interference for non-matching ecosystems**
    * Generate scan results with ecosystems NOT in conditional defaults; verify output is identical with and without conditional defaults
    * Tag: `Feature: conditional-ecosystem-defaults, Property 3: Non-interference for non-matching ecosystems`
    * **Validates: Requirements 2.2, 4.1, 4.2, 4.3**

  * [x] 5.4 Write property test for field preservation
    * **Property 4: Field preservation from lower-priority sources**
    * Generate random fields at lower-priority layers with keys not present in higher layers; verify they appear unchanged in merged result
    * Tag: `Feature: conditional-ecosystem-defaults, Property 4: Field preservation from lower-priority sources`
    * **Validates: Requirements 3.3, 3.4**

  * [x] 5.5 Write property test for empty table identity
    * **Property 5: Empty table identity**
    * With empty `conditionalDefaults` slice, verify output matches pre-feature behavior exactly
    * Tag: `Feature: conditional-ecosystem-defaults, Property 5: Empty table identity`
    * **Validates: Requirements 2.5**

  * [x] 5.6 Write property test for output determinism
    * **Property 6: Output determinism**
    * Call `Generate` multiple times with identical inputs (including conditional defaults); verify byte-for-byte identical output
    * Tag: `Feature: conditional-ecosystem-defaults, Property 6: Output determinism`
    * **Validates: Requirements 6.1**

  * [x] 5.7 Write property test for alphabetical field ordering
    * **Property 7: Alphabetical field ordering**
    * Generate update entries with multiple extra fields from various sources; verify top-level keys appear in strict alphabetical order after `directory`
    * Tag: `Feature: conditional-ecosystem-defaults, Property 7: Alphabetical field ordering`
    * **Validates: Requirements 2.6, 6.2**

* [x] 6. Final checkpoint — Verify all tests pass and lint is clean
  * Run `go test ./lib/scanner/ -count=1` to confirm all tests pass
  * Run `golangci-lint run --fix ./...` to confirm zero diagnostics
  * Run `go vet ./...` to confirm clean build
  * Ensure all tests pass, ask the user if questions arise.

## Notes

* Tasks marked with `*` are optional and can be skipped for faster MVP
* Each task references specific requirements for traceability
* Checkpoints ensure incremental validation
* Property tests validate universal correctness properties from the design document
* Unit tests validate specific scenarios and edge cases
* The `resolveFields` helper is the key integration point — it encapsulates the four-layer merge and keeps `Generate` focused on orchestration
* All code must pass `golangci-lint` (Zero Diagnostics Policy) before completion
* Property tests use `pgregory.net/rapid` which is already a project dependency

## Task dependency graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["2.1"] },
    { "id": 2, "tasks": ["2.2"] },
    { "id": 3, "tasks": ["4.1"] },
    { "id": 4, "tasks": ["5.1", "5.2", "5.3", "5.4", "5.5", "5.6", "5.7"] }
  ]
}
```
