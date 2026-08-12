# Implementation Plan: YAML Header Comment

## Overview

This plan implements layered configuration, header comment injection, directory exclusion, and per-ecosystem field merging for `dependabot-generator`. Tasks are ordered: foundational types/interfaces first, then config loading, scanner modifications, CLI integration, and finally tests.

## Tasks

- [x] 1. Create `lib/config` package with types and built-in defaults
  - [x] 1.1 Create `lib/config/doc.go` with package documentation
    - Add the Apache 2.0 license header and `// Package config ...` doc comment
    - _Requirements: N/A (project convention)_
  - [x] 1.2 Create `lib/config/config.go` with core types and built-in defaults
    - Define `Config`, `EcosystemConfig`, `FileConfig`, `LoadOptions` structs
    - Define built-in default constants/variables: default header URL, default ignore-dirs slice, default ecosystem settings (`_default` with `insecure-external-code-execution`, `schedule.interval`, `cooldown.default-days`, `groups.monthly-batch.patterns`)
    - Define `ErrConfigParse` and `ErrConfigRead` sentinel errors
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 6.1, 6.4_
  - [x] 1.3 Create `lib/config/loader.go` with `LoadConfig` and `Validate` functions
    - Implement `LoadConfig(opts *LoadOptions) (*Config, error)` that resolves the priority chain: CLI flag > env var > local > user > global > built-in defaults
    - Implement config file discovery at local (`<scan-path>/.depgen.toml`), user (`$XDG_CONFIG_HOME/dependabot-generator/config.toml`), and global (`/etc/dependabot-generator/config.toml`) paths
    - Parse TOML files using `github.com/BurntSushi/toml`
    - Implement `Validate(cfg *Config) error` to check ignore patterns are well-formed via `filepath.Match`
    - Handle mutual exclusivity of `CLIHeader` and `CLIHeaderFile` within LoadOptions
    - _Requirements: 1.1, 1.2, 2.1, 3.1, 4.1, 4.2, 4.3, 5.1, 5.2, 5.3, 5.4, 5.5, 5.7, 6.1, 6.2, 6.3, 6.4_

- [x] 2. Checkpoint — Ensure config package compiles
  - Ensure all tests pass, ask the user if questions arise.

- [x] 3. Add comment formatting and text wrapping to `lib/scanner`
  - [x] 3.1 Create `lib/scanner/comment.go` with `FormatComment` and `WrapLine` functions
    - Implement `FormatComment(raw string) string` — strip trailing newline, handle whitespace-only input, split lines, apply prefix normalization (`# ` prefix for non-`#` lines), preserve already-prefixed lines, convert internal blank lines to bare `#`
    - Implement `WrapLine(line string, limit int) []string` — wrap prose to 80-char limit (78 content + 2 for `# ` prefix), detect and preserve URLs intact on a single line, fill lines optimally
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 9.1, 9.2, 9.3, 9.4, 13.1, 13.2_

- [x] 4. Modify `lib/scanner` Generate function and Scan function
  - [x] 4.1 Add `GenerateOptions` struct and `EcosystemConfig` import, update `Generate` signature
    - Define `GenerateOptions` struct with `CommentText string` and `EcosystemDefaults map[string]EcosystemConfig`
    - Change `Generate(results []ScanResult)` to `Generate(results []ScanResult, opts *GenerateOptions) (string, error)`
    - When `opts` is nil or `CommentText` is empty, produce identical output to current behavior (backward compat)
    - When `CommentText` is non-empty, call `FormatComment`, insert result after `---\n` with a blank line before `version: 2`
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 11.1, 11.2, 11.3, 12.1, 12.2, 13.1_
  - [x] 4.2 Implement per-ecosystem field merging in `Generate`
    - Extend `dependabotUpdate` with `ExtraFields map[string]any` (tagged `yaml:"-"`)
    - Implement dotted-key expansion: convert `"schedule.interval"` to nested YAML map structure
    - Look up ecosystem in `EcosystemDefaults`; fall back to `_default` key
    - Merge expanded fields into YAML output after `directory`
    - _Requirements: 5.2, 5.3, 5.4_
  - [x] 4.3 Update `Scan` to accept `ignoreDirs []string` parameter
    - Change signature to `Scan(path string, ignoreDirs []string) ([]ScanResult, error)`
    - In the `WalkDir` callback, check each directory name against ignore patterns using `filepath.Match` semantics
    - When a directory matches, return `fs.SkipDir` to exclude it and all children
    - Passing nil or empty slice preserves current behavior
    - _Requirements: 5.5, 5.6_

- [x] 5. Checkpoint — Ensure scanner package compiles and existing tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. CLI integration in `cmd/`
  - [x] 6.1 Create `cmd/errors.go` with sentinel errors for the CLI layer
    - Define `ErrFlagsMutuallyExclusive`, `ErrHeaderFilePathInvalid`, `ErrHeaderFileNotFound`, `ErrHeaderFileNotReadable`, `ErrHeaderTooLarge`, `ErrConfigSyntax`, `ErrConfigNotReadable`, `ErrIgnorePatternInvalid` in a single `var()` block with individual doc comments
    - _Requirements: 2.2, 2.3, 2.4, 3.1, 8.2_
  - [x] 6.2 Update `cmd/run.go` with new flags, env var lookup, config loading, and validation
    - Register `--header` string flag (empty default)
    - Register `--header-file` string flag (empty default)
    - Add mutual exclusivity check: if both flags are non-empty, return `ErrFlagsMutuallyExclusive`
    - Add `--header-file` path validation: well-formedness check, existence check, readability check
    - Add `DEPGEN_HEADER` environment variable lookup
    - Add size limit enforcement: if resolved comment text exceeds 8,192 bytes, return `ErrHeaderTooLarge`
    - Call `config.LoadConfig` with appropriate `LoadOptions`
    - Call `config.Validate` on the resolved config
    - Pass `config.IgnoreDirs` to `scanner.Scan`
    - Pass `GenerateOptions` (with `CommentText` and `EcosystemDefaults`) to `scanner.Generate`
    - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 3.1, 3.2, 4.1, 4.2, 4.3, 5.5, 5.6, 5.7, 6.1, 6.2, 6.3, 6.4, 8.1, 8.2, 8.3_

- [x] 7. Checkpoint — Ensure full project compiles and existing tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 8. Unit tests for `lib/config`
  - [x] 8.1 Create `lib/config/loader_test.go` with unit tests
    - Test config file discovery at each level (local, user, global)
    - Test TOML parsing with valid config and syntax errors
    - Test priority chain resolution (CLI > env > local > user > global > built-in)
    - Test mutual exclusivity validation
    - Test `Validate` with well-formed and malformed ignore patterns
    - Test XDG_CONFIG_HOME fallback behavior
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.7, 6.1, 6.2, 6.3, 6.4_

- [x] 9. Unit tests for comment formatting and scanner changes
  - [x] 9.1 Create `lib/scanner/comment_test.go` with unit tests for `FormatComment` and `WrapLine`
    - Test prefix normalization (lines with/without `#`)
    - Test text wrapping at 80-char boundary
    - Test URL preservation (mid-line URLs, long URLs, URL-like strings)
    - Test whitespace-only input returns empty
    - Test trailing newline stripping
    - Test internal blank lines become bare `#`
    - Test size limit boundary (8,192 bytes exactly passes, 8,193 fails)
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 8.1, 8.3, 9.1, 9.2, 9.3, 9.4, 13.1, 13.2_
  - [x] 9.2 Update `lib/scanner/scanner_test.go` for updated `Scan` and `Generate` signatures
    - Update existing test calls to pass `nil` for `ignoreDirs` and `nil` for `GenerateOptions` (backward compat)
    - Add tests for directory exclusion with ignore patterns
    - Add tests for `Generate` with comment text (placement, blank line separator)
    - Add tests for per-ecosystem field merging (specific override, `_default` fallback, dotted-key expansion)
    - _Requirements: 5.5, 10.1, 10.2, 10.3, 11.1, 11.2, 11.3, 12.1, 12.2_

- [x] 10. Property-based tests
  - [x] 10.1 Write property test for priority resolution (Property 1)
    - **Property 1: Priority resolution selects highest-priority source**
    - **Validates: Requirements 1.2, 4.2, 4.3, 5.2, 5.3, 5.4, 6.1, 6.2, 6.3**
  - [x] 10.2 Write property test for directory exclusion (Property 2)
    - **Property 2: Directory exclusion prevents results from ignored paths**
    - **Validates: Requirements 5.5**
  - [x] 10.3 Write property test for wrap limit (Property 3)
    - **Property 3: Wrapped lines respect 80-character limit**
    - **Validates: Requirements 7.1, 7.4**
  - [x] 10.4 Write property test for URL preservation (Property 4)
    - **Property 4: URLs are preserved intact during wrapping**
    - **Validates: Requirements 7.2**
  - [x] 10.5 Write property test for short line preservation (Property 5)
    - **Property 5: Short lines are preserved and wrapping fills optimally**
    - **Validates: Requirements 7.3, 7.5**
  - [x] 10.6 Write property test for size limit (Property 6)
    - **Property 6: Size limit enforcement on raw input**
    - **Validates: Requirements 8.1, 8.3**
  - [x] 10.7 Write property test for prefix idempotence (Property 7)
    - **Property 7: Comment prefix normalization is idempotent**
    - **Validates: Requirements 9.1, 9.2, 9.3**
  - [x] 10.8 Write property test for trailing newline handling (Property 8)
    - **Property 8: Trailing newline does not produce empty trailing comment**
    - **Validates: Requirements 9.4**
  - [x] 10.9 Write property test for comment placement (Property 9)
    - **Property 9: Comment placement structure**
    - **Validates: Requirements 10.1, 10.2**
  - [x] 10.10 Write property test for YAML round-trip (Property 10)
    - **Property 10: YAML round-trip validity**
    - **Validates: Requirements 10.3, 11.2, 12.1**
  - [x] 10.11 Write property test for comment lines validity (Property 11)
    - **Property 11: All header lines are valid YAML comments**
    - **Validates: Requirements 12.2**
  - [x] 10.12 Write property test for whitespace-only input (Property 12)
    - **Property 12: Whitespace-only input produces no comment**
    - **Validates: Requirements 13.1**
  - [x] 10.13 Write property test for internal blank lines (Property 13)
    - **Property 13: Internal blank lines become bare comment markers**
    - **Validates: Requirements 13.2**

- [x] 11. Integration tests
  - [x] 11.1 Create integration test for end-to-end CLI pipeline
    - Create a fixture tree with `.depgen.toml`, run the full pipeline, verify output YAML contains expected header, exclusions worked, and per-ecosystem fields are present
    - Test priority chain: set values at multiple levels, verify correct override behavior
    - Test backward compatibility: empty config produces identical output to current behavior
    - _Requirements: 1.1, 3.2, 5.5, 6.1, 10.1, 10.2, 10.3, 11.2_

- [x] 12. Final checkpoint — Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

* Tasks marked with `*` are optional and can be skipped for faster MVP
* Each task references specific requirements for traceability
* Checkpoints ensure incremental validation
* Property tests validate universal correctness properties from the design document using `pgregory.net/rapid`
* Unit tests validate specific examples and edge cases
* The `github.com/BurntSushi/toml` dependency must be added via `go get` before task 1.3
* Existing `scanner_test.go` calls to `Scan` and `Generate` must be updated for the new signatures in task 9.2

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2", "6.1"] },
    { "id": 1, "tasks": ["1.3", "3.1"] },
    { "id": 2, "tasks": ["4.1", "4.3"] },
    { "id": 3, "tasks": ["4.2"] },
    { "id": 4, "tasks": ["6.2"] },
    { "id": 5, "tasks": ["8.1", "9.1"] },
    { "id": 6, "tasks": ["9.2"] },
    { "id": 7, "tasks": ["10.1", "10.2", "10.3", "10.4", "10.5", "10.6", "10.7", "10.8", "10.12", "10.13"] },
    { "id": 8, "tasks": ["10.9", "10.10", "10.11", "11.1"] }
  ]
}
```
