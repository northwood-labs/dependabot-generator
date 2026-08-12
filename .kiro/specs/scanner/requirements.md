# Requirements Document

## Introduction

This feature implements a directory scanner and Dependabot configuration generator for the `dependabot-generator` CLI tool. The scanner walks a project directory tree, matches file patterns against a known table of Dependabot ecosystem identifiers, and produces a slice of results mapping directories to ecosystems. The generator consumes those results and outputs a valid Dependabot v2 YAML configuration file. A single directory may match multiple ecosystems (e.g., a directory containing both `go.mod` and `Dockerfile`), so results must be stored as a slice of structs rather than a simple map.

## Glossary

* **Scanner**: The component that walks a directory tree and identifies ecosystem matches based on file patterns.
* **Generator**: The component that accepts scan results and produces a valid Dependabot v2 YAML configuration file.
* **Ecosystem**: A package manager or build system recognized by GitHub Dependabot (e.g., `gomod`, `npm`, `docker`).
* **Identifier**: The string value used in the `package-ecosystem` field of a Dependabot configuration (e.g., `"github-actions"`, `"cargo"`).
* **EcosystemRule**: A data structure describing one ecosystem's detection logic: its identifier and the set of file glob patterns (with AND/OR logic) that indicate its presence in a directory.
* **ScanResult**: A data structure pairing a directory path with a matched ecosystem identifier.
* **Dependabot_Config**: The output YAML file conforming to Dependabot version 2 schema with a `version` key and an `updates` array.

## Requirements

### Requirement 1: Ecosystem rule definition

**User Story:** As a developer, I want ecosystem detection rules defined as structured data, so that adding or modifying ecosystem support requires only data changes, not logic changes.

#### Acceptance criteria

1. THE Scanner SHALL define each ecosystem rule as a struct containing an identifier string and a slice of file glob patterns.
2. THE Scanner SHALL support both simple rules (any single pattern match) and compound rules (all patterns in a group must match within the same directory).
3. THE Scanner SHALL define rules for exactly the 33 ecosystems in the project's ecosystem table (Bazel, Bun, Bundler, Cargo, Composer, Conda, Deno, Dev containers, Docker, Docker Compose, .NET SDK, elm-package, git submodule, GitHub Actions, Go modules, Gradle, Helm Charts, Hex, Julia, Maven, Nix flakes, npm/pnpm/yarn, NuGet, OpenTofu, Python (not uv), pre-commit, pub, Rust toolchain, sbt, Swift, Terraform, uv, vcpkg).
4. THE Scanner SHALL restrict ecosystem rules to exactly these 33 ecosystems. No mechanism for adding additional custom patterns SHALL be provided.

### Requirement 2: Directory scanning

**User Story:** As a user, I want the scanner to walk my project directory and detect which package ecosystems are present, so that I get accurate Dependabot coverage.

#### Acceptance criteria

1. WHEN a root path is provided, THE Scanner SHALL recursively walk the directory tree and evaluate ecosystem rules against each directory.
2. WHEN a directory contains files matching an ecosystem rule, THE Scanner SHALL record that directory and ecosystem identifier as a ScanResult.
3. WHEN a single directory matches multiple ecosystem rules, THE Scanner SHALL produce one ScanResult per match (not collapse them into a single entry).
4. THE Scanner SHALL use the `github.com/goreleaser/fileglob` library for all glob pattern matching.
5. WHEN the root path does not exist or is not a directory, THE Scanner SHALL return a descriptive error.
6. WHEN the root path exists but is not readable or accessible, THE Scanner SHALL return a descriptive error before beginning the directory walk.
7. THE Scanner SHALL store results in a slice of ScanResult structs (not a map), preserving the ability to have multiple entries per directory.
8. THE Scanner SHALL express directory paths in results relative to the provided root path, using forward-slash (`/`) prefix notation (e.g., `/`, `/subdir/nested`).

### Requirement 3: Compound pattern matching

**User Story:** As a developer, I want ecosystems that require multiple files to co-exist (e.g., `package.json` + `bun.lock` for Bun) to only match when all required files are present in the same directory, so that false positives are avoided.

#### Acceptance criteria

1. WHEN an ecosystem rule uses AND logic (multiple required patterns), THE Scanner SHALL match only when all required patterns resolve to at least one file in the directory.
2. WHEN an ecosystem rule uses OR logic (alternative patterns), THE Scanner SHALL match when any single alternative resolves to at least one file in the directory.
3. WHEN an ecosystem rule combines AND and OR logic, THE Scanner SHALL evaluate the expression correctly (e.g., `bunfig.toml` OR (`package.json` AND `bun.lock`) for Bun; `.terraform.lock.hcl` AND `*.tofu` for OpenTofu; `.terraform.lock.hcl` AND `*.tf` for Terraform).

### Requirement 4: Ecosystem precedence

**User Story:** As a user, I want the scanner to resolve ambiguity when multiple ecosystems claim the same lock file, so that only the correct ecosystem is reported for a given directory.

#### Acceptance criteria

1. WHEN a directory contains `.terraform.lock.hcl` along with both `*.tofu` and `*.tf` files, THE Scanner SHALL produce a ScanResult for `opentofu` only and SHALL NOT produce a ScanResult for `terraform` in that directory.
2. WHEN a directory contains `.terraform.lock.hcl` along with `*.tf` files but no `*.tofu` files, THE Scanner SHALL produce a ScanResult for `terraform`.
3. WHEN a directory contains `.terraform.lock.hcl` along with `*.tofu` files but no `*.tf` files, THE Scanner SHALL produce a ScanResult for `opentofu`.

### Requirement 5: YAML generation

**User Story:** As a user, I want the generator to produce a valid Dependabot v2 YAML file from scan results, so that I can use it directly in my repository.

#### Acceptance criteria

1. WHEN scan results are provided, THE Generator SHALL produce YAML output with `version: 2` and an `updates` array.
2. WHEN producing an update entry, THE Generator SHALL include `package-ecosystem` and `directory`. All other values can be added by the user later.
3. THE Generator SHALL sort update entries first by directory path ascending, then by ecosystem identifier ascending within the same directory.
4. WHEN scan results are empty, THE Generator SHALL produce a valid YAML document with `version: 2` and an empty `updates` array.
5. THE Generator SHALL produce YAML that starts with a `---` document separator.

### Requirement 6: Scan function signature

**User Story:** As a developer integrating the scanner into the CLI, I want `Scan` to accept a path and return structured results with proper error handling, so that the calling command can act on results or report errors.

#### Acceptance criteria

1. THE Scan function SHALL accept a single string parameter (the root path) and return a slice of ScanResult and an error.
2. IF an error occurs during directory traversal or glob evaluation, THEN THE Scan function SHALL return the error wrapped with context describing the failure.

### Requirement 7: Generate function signature

**User Story:** As a developer integrating the generator into the CLI, I want `Generate` to accept scan results and return the YAML string with proper error handling, so that the calling command can write the output or report errors.

#### Acceptance criteria

1. THE Generate function SHALL accept a slice of ScanResult and return a string (the YAML content) and an error.
2. IF YAML marshaling fails, THEN THE Generate function SHALL return the error wrapped with context describing the failure.

### Requirement 8: Serialization round-trip consistency

**User Story:** As a developer, I want the YAML output to faithfully represent all scan results, so that no detected ecosystems are lost or duplicated in the generated configuration.

#### Acceptance criteria

1. FOR ALL valid slices of ScanResult, THE Generator SHALL produce a YAML document whose `updates` array contains exactly one entry per ScanResult.
2. FOR ALL valid slices of ScanResult, parsing the generated YAML back into a structured form SHALL yield the same set of directory-ecosystem pairs as the original input (after accounting for sort order).
