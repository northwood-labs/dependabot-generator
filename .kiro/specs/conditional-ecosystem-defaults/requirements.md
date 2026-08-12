# Requirements Document

## Introduction

The conditional-ecosystem-defaults feature adds a compiled-in mechanism for applying Dependabot configuration fields conditionally, based on the matched ecosystem identifier. Unlike the existing `.depgen.toml` ecosystem defaults (which are user-configurable), these defaults are built into the binary and applied automatically during YAML generation. The initial use case is injecting `insecure-external-code-execution: deny` for the `bundler`, `mix`, and `pip` ecosystems only. The design must support easy future extension to other ecosystems and other conditional fields.

## Glossary

* **Generator**: The `scanner.Generate` function that converts scan results into a Dependabot v2 YAML configuration string.
* **Conditional_Default**: A key-value pair that is injected into a generated update entry only when the entry's ecosystem identifier matches a predefined set of ecosystems.
* **Ecosystem_Identifier**: The string name of a package ecosystem as defined in the `ecosystemRules` table (e.g., `"bundler"`, `"pip"`, `"npm"`).
* **Builtin_Defaults**: The compiled-in `DefaultEcosystemSettings` map in `lib/config/config.go` that provides fallback configuration for all ecosystems.
* **Conditional_Defaults_Table**: A compiled-in data structure that maps conditional field key-value pairs to the set of ecosystem identifiers where they apply.
* **Field_Merge**: The process of combining Builtin_Defaults, Conditional_Defaults, and user-configured `.depgen.toml` overrides into a final set of fields for a generated update entry.

## Requirements

### Requirement 1: Conditional defaults table definition

**User Story:** As a maintainer, I want conditional defaults defined as a compiled-in data table, so that adding new conditional fields or ecosystems requires only a single-location code change.

#### Acceptance criteria

1. THE Conditional_Defaults_Table SHALL define each conditional default as a struct containing: a field key (dotted-path string matching the format used in `EcosystemConfig.Fields`), a field value (of type `any`, consistent with the existing `map[string]any` pattern), and a non-empty list of Ecosystem_Identifiers to which the default applies.
2. THE Conditional_Defaults_Table SHALL be defined in a Go source file within the `lib/scanner` package as a package-level variable, adjacent to the existing `ecosystemRules` table.
3. EACH Ecosystem_Identifier in the Conditional_Defaults_Table SHALL correspond to a valid `Identifier` field value in the `ecosystemRules` table; identifiers not present in `ecosystemRules` SHALL be treated as dead entries (no effect) without causing a compile or runtime error.
4. THE Conditional_Defaults_Table SHALL include the entry: field `"insecure-external-code-execution"` with value `"deny"` for ecosystems `"bundler"`, `"mix"`, and `"pip"`.
5. WHEN a new conditional default is needed, THE Conditional_Defaults_Table SHALL allow the addition by appending a single struct literal to the slice without modifying existing logic or other table entries.

### Requirement 2: Conditional default application during generation

**User Story:** As a user, I want ecosystem-specific defaults applied automatically when generating Dependabot configuration, so that ecosystems requiring `insecure-external-code-execution` get the correct setting without manual configuration.

#### Acceptance criteria

1. WHEN the Generator produces an update entry for an Ecosystem_Identifier that appears in the Conditional_Defaults_Table, THE Generator SHALL include each field-value pair from that ecosystem's Conditional_Defaults_Table row in the generated update entry, using the same dotted-key expansion as Builtin_Defaults (e.g., `"insecure-external-code-execution" = "deny"` produces `insecure-external-code-execution: deny` in the YAML output).
2. WHEN the Generator produces an update entry for an Ecosystem_Identifier that does not appear in the Conditional_Defaults_Table, THE Generator SHALL omit all conditional default fields from that entry's output.
3. THE Generator SHALL merge field sources in the following precedence order (highest wins): user-configured `.depgen.toml` ecosystem overrides > Conditional_Defaults_Table values > Builtin_Defaults (`DefaultEcosystemSettings`), so that a field present at a higher-priority layer replaces the same field from any lower-priority layer.
4. WHEN a user-configured `.depgen.toml` override specifies a value for the same dotted-key field as a conditional default, THE Generator SHALL use the user-configured value and omit the conditional default value for that field in the generated output.
5. IF the Conditional_Defaults_Table contains zero entries, THEN THE Generator SHALL produce output identical to the current behavior (Builtin_Defaults merged with user-configured `.depgen.toml` overrides only).
6. WHEN the Generator applies a conditional default field to an update entry, THE Generator SHALL position the conditional default field in alphabetical order among the other extra fields in the YAML output, consistent with existing field-ordering behavior for Builtin_Defaults.

### Requirement 3: Merge order integrity

**User Story:** As a user, I want my `.depgen.toml` settings to always override built-in conditional defaults, so that I retain full control over my generated configuration.

#### Acceptance criteria

1. THE Field_Merge SHALL apply sources in the following priority order (highest to lowest): user-configured `.depgen.toml` ecosystem-specific override, user-configured `.depgen.toml` `_default` override, Conditional_Defaults_Table entry matched by ecosystem identifier, Builtin_Defaults `_default`.
2. WHEN a field (identified by its dotted-path key in the Fields map) is defined at multiple priority levels, THE Field_Merge SHALL use the value from the highest-priority source only, discarding all lower-priority values for that key.
3. THE Field_Merge SHALL preserve all fields from lower-priority sources whose dotted-path keys are not present in any higher-priority source.
4. IF a priority source defines an ecosystem key but its Fields map is empty or nil, THEN THE Field_Merge SHALL treat that source as contributing zero fields, without suppressing fields from lower-priority sources.
5. WHEN resolving fields for a scan result, THE Field_Merge SHALL select the Conditional_Defaults_Table entry whose key matches the scan result's ecosystem identifier; IF no matching entry exists, THE Field_Merge SHALL skip the conditional defaults layer entirely and proceed to the next lower-priority source.
6. THE Field_Merge SHALL enforce explicit constraints: WHEN a conditional default is applied for a matching ecosystem, THE merged result for that field SHALL equal the conditional default value unless a higher-priority source overrides it; test assertions SHALL validate the exact source of each merged field value.

### Requirement 4: No impact on Non-Matching ecosystems

**User Story:** As a user with projects using ecosystems outside the conditional set, I want my generated output to be unchanged by this feature, so that existing behavior is preserved.

#### Acceptance criteria

1. WHEN scanning a repository that contains only ecosystems not listed in the Conditional_Defaults_Table (e.g., `"gomod"`, `"npm"`, `"cargo"`), THE Generator SHALL produce byte-for-byte identical YAML output to the output produced by the same Generator version without conditional defaults applied.
2. THE Conditional_Defaults_Table SHALL NOT merge entries into, overwrite, or shadow the Builtin_Defaults `_default` entry; the `_default` entry SHALL remain the sole source of fields applied to ecosystems that have no ecosystem-specific or conditional override.
3. WHEN the Generator resolves per-ecosystem fields for an ecosystem not listed in the Conditional_Defaults_Table, THE Generator SHALL apply only the `_default` entry from EcosystemDefaults, with no additional fields injected by the conditional defaults mechanism.
4. IF a Conditional_Defaults_Table entry uses the key `_default`, THEN THE Generator SHALL reject the configuration and report an error indicating that the reserved key `_default` cannot be used as a conditional defaults identifier.
5. WHEN loading the Conditional_Defaults_Table during initialization, THE system SHALL validate that no entry's ecosystem list contains identifiers that would cause field injection into non-matching ecosystems; IF validation detects an entry that could shadow or interfere with the `_default` fallback path, THE system SHALL reject the table with a descriptive error.

### Requirement 5: Binary-Embedded implementation

**User Story:** As a maintainer, I want conditional defaults compiled into the binary rather than loaded from external configuration, so that the feature works without any config file changes and behaves identically across all installations.

#### Acceptance criteria

1. THE Conditional_Defaults_Table SHALL be defined as a Go package-level variable initialized with a composite literal at compile time (not read from a file, environment variable, or CLI flag at runtime).
2. THE Conditional_Defaults_Table SHALL not require any entry in `.depgen.toml` to activate or modify its behavior; user `.depgen.toml` configuration SHALL only override individual field values via the standard Field_Merge precedence, never enable or disable the conditional defaults mechanism itself.
3. WHEN the binary is executed without any `.depgen.toml` file present, THE Generator SHALL still apply conditional defaults to matching ecosystems, producing the same output as when a `.depgen.toml` exists but contains no overrides for the conditional fields.
4. THE Conditional_Defaults_Table SHALL NOT be modifiable at runtime; no CLI flag, environment variable, or configuration directive SHALL add, remove, or reorder entries in the table.

### Requirement 6: Deterministic output ordering

**User Story:** As a user who commits the generated `dependabot.yml`, I want the output to remain deterministic regardless of the conditional defaults, so that diffs are minimal and reviewable.

#### Acceptance criteria

1. THE Generator SHALL produce byte-for-byte identical YAML output for identical scan results and configuration, regardless of the internal iteration order of the Conditional_Defaults_Table.
2. WHEN conditional default fields are emitted in the YAML, THE Generator SHALL position them in alphabetical order by top-level key name after the `directory` field, interleaved with any static ecosystem default fields using the same alphabetical sort.
3. IF a conditional default defines a field key that is also defined by the static ecosystem default for the same entry, THEN THE Generator SHALL use the conditional default value, discarding the static ecosystem default value for that key, without affecting output determinism.
