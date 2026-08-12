# Design Document

## Overview

This design extends the `dependabot-generator` CLI with three capabilities:

1. **Header comment injection** — inserting user-provided comment text between `---` and `version: 2` in the generated YAML output.
2. **Directory exclusion** — skipping specified directories (and their children) during the filesystem scan.
3. **Per-ecosystem Dependabot fields** — merging additional configuration values (schedule, cooldown, groups) into each update entry based on the matched ecosystem.

All three capabilities are driven by a layered configuration system using TOML files at local, user, and global levels, plus CLI flags and environment variables. The system provides sensible built-in defaults that apply when no configuration is provided.

The design preserves backward compatibility: when no configuration is present, the tool produces identical output to the current behavior.

## Architecture

```mermaid
flowchart TD
    CLI[cmd/run.go] -->|flags, env, path| ConfigLoader
    ConfigLoader -->|Config| CLI
    CLI -->|path, Config.IgnoreDirs| Scan
    Scan -->|[]ScanResult, error| CLI
    CLI -->|[]ScanResult, Config| Generate
    Generate -->|string, error| CLI

    subgraph lib/config
        ConfigLoader[LoadConfig]
        ConfigLoader --> TOMLParser[BurntSushi/toml]
        ConfigLoader --> Merger[MergeDefaults]
        ConfigLoader --> Validator[Validate]
    end

    subgraph lib/scanner
        Scan --> Walker[fs.WalkDir]
        Scan --> SkipDir[IgnoreFilter]
        Walker --> Matcher[fileglob.Glob]
        Generate --> CommentFormatter[FormatComment]
        Generate --> Sorter[slices.SortFunc]
        Generate --> Marshaler[yaml.Marshal]
        Generate --> EcosystemMerger[MergeEcosystemConfig]
    end
```

The workflow becomes:

1. `LoadConfig` resolves the layered configuration (CLI flags > env var > local > user > global > built-in defaults).
2. `Scan(path, ignoreDirs)` walks the tree, skipping excluded directories.
3. `Generate(results, config)` produces YAML with optional header comment and per-ecosystem fields.

## Components and interfaces

### New package: `lib/config`

```go
// Config holds the fully-resolved configuration after merging all
// sources according to priority rules.
type Config struct {
    HeaderComment     string
    IgnoreDirs        []string
    EcosystemDefaults map[string]EcosystemConfig
}

// EcosystemConfig holds additional Dependabot v2 fields for a
// specific ecosystem. The Fields map is keyed by dotted path
// (e.g., "schedule.interval") and maps to arbitrary YAML values.
type EcosystemConfig struct {
    Fields map[string]any
}

// FileConfig represents the structure of a single TOML config file
// before merging. This maps directly to the TOML schema.
type FileConfig struct {
    Header     string                       `toml:"header"`
    IgnoreDirs []string                     `toml:"ignore-dirs"`
    Ecosystems map[string]map[string]any    `toml:"ecosystems"`
}

// LoadConfig resolves configuration from all sources in priority
// order, applying built-in defaults for any unset values.
func LoadConfig(opts *LoadOptions) (*Config, error)

// LoadOptions bundles the inputs needed to resolve configuration.
type LoadOptions struct {
    CLIHeader     string
    CLIHeaderFile string
    EnvHeader     string
    ScanPath      string
}

// Validate checks that the resolved config is internally consistent
// (e.g., ignore patterns are well-formed).
func Validate(cfg *Config) error
```

### Modified: `lib/scanner`

```go
// Scan now accepts ignore patterns to exclude directories from the
// walk. Passing nil or an empty slice preserves current behavior.
func Scan(path string, ignoreDirs []string) ([]ScanResult, error)

// Generate now accepts a GenerateOptions struct containing comment
// text and per-ecosystem configuration alongside scan results.
func Generate(results []ScanResult, opts *GenerateOptions) (string, error)

// GenerateOptions holds all inputs for YAML generation beyond the
// scan results themselves.
type GenerateOptions struct {
    CommentText       string
    EcosystemDefaults map[string]EcosystemConfig
}

// FormatComment applies prefix normalization, text wrapping, and
// trailing newline stripping to raw comment text. Returns the
// formatted comment block ready for insertion (each line prefixed
// with #). Returns empty string for whitespace-only input.
func FormatComment(raw string) string

// WrapLine wraps a single line of text to the 80-character limit,
// preserving URLs intact. Returns a slice of wrapped lines.
func WrapLine(line string, limit int) []string
```

### Modified: `cmd/run.go`

The `run` command gains:

* `--header` flag (string, empty default)
* `--header-file` flag (string, empty default)
* Environment variable lookup (`DEPGEN_HEADER`)
* Config file discovery and loading via `lib/config`
* Mutual exclusivity validation for `--header` / `--header-file`
* Size limit enforcement (8,192 bytes) on resolved comment text

### Extended: `dependabotUpdate` struct

```go
// dependabotUpdate represents a single entry in the Dependabot
// updates array. ExtraFields holds per-ecosystem configuration
// values that get merged into the YAML output.
type dependabotUpdate struct {
    PackageEcosystem string         `yaml:"package-ecosystem"`
    Directory        string         `yaml:"directory"`
    ExtraFields      map[string]any `yaml:"-"`
}
```

The `ExtraFields` map is excluded from automatic marshaling (`yaml:"-"`) and instead merged manually during YAML node construction so that dotted keys (e.g., `schedule.interval`) produce nested YAML structures.

## Data models

### Configuration file schema (TOML)

The application ships with sensible built-in defaults compiled into the binary (see the "Built-in defaults" table below). A configuration file only needs to specify values that differ from those defaults. Any setting not present in a config file falls through to the built-in default value.

```toml
# .depgen.toml (local) or config.toml (user/global)
# Only specify values you want to override. Omitted settings
# use the built-in defaults compiled into the binary.

# Override the default schedule interval for npm to weekly.
[ecosystems.npm]
"schedule.interval" = "weekly"

# Add a custom header comment instead of the default URL.
header = "Managed by dependabot-generator — do not edit by hand."
```

### Configuration file locations

| Level  | Path                                                |
|--------|-----------------------------------------------------|
| Local  | `<scan-path>/.depgen.toml`                          |
| User   | `$XDG_CONFIG_HOME/dependabot-generator/config.toml` |
| Global | `/etc/dependabot-generator/config.toml`             |

When `$XDG_CONFIG_HOME` is not set, it defaults to `$HOME/.config` per the XDG Base Directory Specification.

### Built-in default values

These values are compiled into the application binary as Go constants and variables. They apply automatically when no configuration file exists at any level and no CLI/env source overrides them:

| Setting                                                | Default value                                                                                            |
|--------------------------------------------------------|----------------------------------------------------------------------------------------------------------|
| `header`                                               | `https://docs.github.com/github/administering-a-repository/configuration-options-for-dependency-updates` |
| `ignore-dirs`                                          | `["node_modules", ".venv", "venv", "vendor", ".*"]`                                                      |
| `ecosystems._default.insecure-external-code-execution` | `"deny"`                                                                                                 |
| `ecosystems._default.schedule.interval`                | `"monthly"`                                                                                              |
| `ecosystems._default.cooldown.default-days`            | `7`                                                                                                      |
| `ecosystems._default.groups.monthly-batch.patterns`    | `["*"]`                                                                                                  |

### Input priority chain

Resolution order for each setting (highest wins):

```text
CLI flag → Environment variable → Local config → User config → Global config → Built-in default
```

For `header` specifically:

1. `--header` flag value (non-empty)
2. `--header-file` file contents (non-empty)
3. `DEPGEN_HEADER` env var (non-empty)
4. Local config `header` key
5. User config `header` key
6. Global config `header` key
7. Built-in default

For `ignore-dirs` and `ecosystems`, config files merge with built-in defaults (config file values override defaults, not append).

### Generated YAML output structure

With configuration applied:

```yaml
---
# https://docs.github.com/github/administering-a-repository/configuration-options-for-dependency-updates

version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    insecure-external-code-execution: deny
    schedule:
      interval: monthly
    cooldown:
      default-days: 7
    groups:
      monthly-batch:
        patterns:
          - "*"
```

### Comment text wrapping rules

The `FormatComment` function applies these rules in order:

1. Strip trailing newline from input.
2. If input is whitespace-only after stripping, return empty (no comment).
3. Split into lines.
4. For each line:
   * If it already starts with `#`, keep it unchanged.
   * If it does not start with `#`:
     * If it is ≤78 characters (80 minus `# ` prefix), prepend `# ` and emit.
     * If it contains a URL, prepend `# ` and emit unchanged (URL exception).
     * If it exceeds 78 characters with no URL, reflow/wrap to fit within the 78-character content limit (80 total with `# ` prefix), filling lines optimally.
5. Internal blank lines become bare `#` lines.

### Directory exclusion logic

The `Scan` function's `WalkDir` callback checks each directory name against the ignore patterns before descending. Pattern matching uses `filepath.Match` semantics:

* `node_modules` — exact name match
* `.*` — any name starting with `.` (hidden directories)
* `vendor` — exact name match

When a directory matches an ignore pattern, `fs.SkipDir` is returned to prevent descending into it or any of its children.

### Per-ecosystem field merging

During `Generate`, each `dependabotUpdate` entry is enriched:

1. Look up the ecosystem identifier in `EcosystemDefaults`.
2. If a specific ecosystem config exists, use it.
3. Otherwise, use `_default` config.
4. Convert dotted keys to nested YAML structure (e.g., `"schedule.interval": "monthly"` becomes `schedule:\n  interval: monthly`).
5. Merge fields into the YAML node after `directory`.

#### Dotted-key expansion

Each period (`.`) in a setting key becomes a level of YAML nesting. The final segment is the leaf key, and the value becomes that leaf's value.

| TOML Setting Key                   | Value       | YAML Output                                              |
|------------------------------------|-------------|----------------------------------------------------------|
| `insecure-external-code-execution` | `"deny"`    | `insecure-external-code-execution: deny` (flat, no dots) |
| `schedule.interval`                | `"monthly"` | `schedule:\n  interval: monthly`                         |
| `cooldown.default-days`            | `7`         | `cooldown:\n  default-days: 7`                           |
| `groups.monthly-batch.patterns`    | `["*"]`     | `groups:\n  monthly-batch:\n    patterns:\n      - "*"`  |

**TOML input (from config or built-in defaults):**

```toml
[ecosystems._default]
insecure-external-code-execution = "deny"
"schedule.interval" = "monthly"
"cooldown.default-days" = 7
"groups.monthly-batch.patterns" = ["*"]
```

**Generated YAML output (per update entry):**

```yaml
  - package-ecosystem: gomod
    directory: /
    insecure-external-code-execution: deny
    schedule:
      interval: monthly
    cooldown:
      default-days: 7
    groups:
      monthly-batch:
        patterns:
          - "*"
```

## Correctness properties

_A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees._

### Property 1: Priority resolution selects highest-priority source

_For any_ combination of configuration sources (CLI flag, environment variable, local config, user config, global config) where at least one provides a non-empty header value, the resolved Comment_Text SHALL equal the value from the highest-priority non-empty source.

**Validates: Requirements 1.2, 4.2, 4.3, 5.2, 5.3, 5.4, 6.1, 6.2, 6.3**

### Property 2: Directory exclusion prevents results from ignored paths

_For any_ directory tree and any set of ignore patterns, scanning with those patterns SHALL produce no `ScanResult` entries whose `Directory` field matches or is a child of an ignored directory.

**Validates: Requirements 5.5**

### Property 3: Wrapped lines respect 80-character limit

_For any_ prose string that does not contain a URL, all output lines produced by `FormatComment` SHALL be 80 characters or fewer (including the `# ` prefix).

**Validates: Requirements 7.1, 7.4**

### Property 4: URLs are preserved intact during wrapping

_For any_ input line containing a URL, the formatted output SHALL contain that URL on a single line without splitting or truncation.

**Validates: Requirements 7.2**

### Property 5: Short lines are preserved and wrapping fills optimally

_For any_ input line that is already ≤78 characters (content, excluding prefix), `FormatComment` SHALL emit that line unchanged (with only the `# ` prefix added). Additionally, no line SHALL wrap early if the next word would fit within the 80-character limit.

**Validates: Requirements 7.3, 7.5**

### Property 6: Size limit enforcement on raw input

_For any_ string of N bytes, when N ≤ 8192 the size validation SHALL pass, and when N > 8192 the size validation SHALL return an error. The measurement is performed on the raw input before formatting.

**Validates: Requirements 8.1, 8.3**

### Property 7: Comment prefix normalization is idempotent

_For any_ line of text, applying the prefix normalization function ensures the line starts with `#`. Applying the function a second time to the already-normalized output SHALL produce the same result (idempotence).

**Validates: Requirements 9.1, 9.2, 9.3**

### Property 8: Trailing newline does not produce empty trailing comment

_For any_ non-whitespace string with one or more trailing newline characters, `FormatComment` SHALL produce output whose last line is not a bare `#` that resulted from the trailing newline.

**Validates: Requirements 9.4**

### Property 9: Comment placement structure

_For any_ non-empty Comment_Text and any valid scan results, the generated output SHALL have the comment block immediately after `---\n` and SHALL have a blank line separating the last comment line from `version: 2`.

**Validates: Requirements 10.1, 10.2**

### Property 10: YAML round-trip validity

_For any_ valid Comment_Text string (including empty) and any valid slice of `ScanResult`, parsing the generated YAML output SHALL yield a valid Dependabot v2 configuration with `version: 2` and an `updates` array whose length equals the input slice length, with matching ecosystem and directory values.

**Validates: Requirements 10.3, 11.2, 12.1**

### Property 11: All header lines are valid YAML comments

_For any_ non-empty Comment_Text, every line between `---` and the blank line before `version:` in the generated output SHALL begin with `#`.

**Validates: Requirements 12.2**

### Property 12: Whitespace-only input produces no comment

_For any_ string consisting entirely of whitespace characters (spaces, tabs, newlines), `FormatComment` SHALL return an empty string, and `Generate` SHALL produce output with no comment lines.

**Validates: Requirements 13.1**

### Property 13: Internal blank lines become bare comment markers

_For any_ Comment_Text containing internal blank lines (empty lines between non-empty lines), those blank lines SHALL appear as bare `#` lines in the formatted output.

**Validates: Requirements 13.2**

## Error handling

| Condition                               | Behavior                                                                     |
|-----------------------------------------|------------------------------------------------------------------------------|
| Both `--header` and `--header-file` set | Return error: `"--header and --header-file are mutually exclusive"`          |
| `--header-file` path malformed          | Return error: `"header file path is invalid: <path>"`                        |
| `--header-file` not found               | Return error: `"header file not found: <path>"`                              |
| `--header-file` not readable            | Return error: `"header file could not be read: <path>: <os error>"`          |
| Comment text exceeds 8,192 bytes        | Return error: `"header comment exceeds maximum size (8192 bytes)"`           |
| Config file has invalid TOML syntax     | Return error: `"invalid config file <path>: <parse error>"`                  |
| Ignore pattern is malformed             | Return error: `"invalid ignore pattern in config: <pattern>: <match error>"` |
| Config file not readable (permissions)  | Return error: `"config file not readable: <path>: <os error>"`               |

All errors use sentinel definitions wrapped with context via `fmt.Errorf("%w: %s", ErrSentinel, detail)`. New sentinels in `cmd/errors.go`:

```go
var (
    ErrFlagsMutuallyExclusive = errors.New(
        "--header and --header-file are mutually exclusive")
    ErrHeaderFilePathInvalid  = errors.New("header file path is invalid")
    ErrHeaderFileNotFound     = errors.New("header file not found")
    ErrHeaderFileNotReadable  = errors.New("header file could not be read")
    ErrHeaderTooLarge         = errors.New(
        "header comment exceeds maximum size")
    ErrConfigSyntax           = errors.New("invalid config file")
    ErrConfigNotReadable      = errors.New("config file not readable")
    ErrIgnorePatternInvalid   = errors.New(
        "invalid ignore pattern in config")
)
```

New sentinels in `lib/config/`:

```go
var (
    ErrConfigParse = errors.New("failed to parse config file")
    ErrConfigRead  = errors.New("failed to read config file")
)
```

## Testing strategy

### Property-based tests

The feature's core logic — priority resolution, comment formatting, text wrapping, directory exclusion, and YAML round-tripping — consists of pure functions with large input spaces, making PBT highly effective.

**Library:** [`pgregory.net/rapid`](https://github.com/flyingmutant/rapid) — already a project dependency.

**Configuration:**

* Minimum 100 iterations per property test.
* Each test tagged with a comment referencing the design property.
* Tag format: `// Feature: yaml-header-comment, Property {N}: {title}`

**Properties to implement:**

1. Priority resolution selects highest-priority source (Property 1)
2. Directory exclusion prevents results from ignored paths (Property 2)
3. Wrapped lines respect 80-character limit (Property 3)
4. URLs are preserved intact during wrapping (Property 4)
5. Short lines are preserved and wrapping fills optimally (Property 5)
6. Size limit enforcement on raw input (Property 6)
7. Comment prefix normalization is idempotent (Property 7)
8. Trailing newline does not produce empty trailing comment (Property 8)
9. Comment placement structure (Property 9)
10. YAML round-trip validity (Property 10)
11. All header lines are valid YAML comments (Property 11)
12. Whitespace-only input produces no comment (Property 12)
13. Internal blank lines become bare comment markers (Property 13)

### Unit tests

Unit tests complement PBT by covering specific examples and integration points:

* Mutual exclusivity of `--header` and `--header-file` flags.
* File reading from `--header-file` (valid, missing, unreadable).
* Config file discovery at each level (local, user, global).
* Config file parsing with valid TOML and syntax errors.
* Size limit boundary (8,192 exactly passes, 8,193 fails).
* Per-ecosystem field merging (specific ecosystem override vs `_default`).
* Dotted-key expansion (`"schedule.interval"` → nested YAML).
* Backward compatibility: empty config produces identical output to current.
* URL detection edge cases (URLs mid-line, multiple URLs, URL-like strings).

### Integration tests

* End-to-end: create a fixture tree with a `.depgen.toml`, run the full CLI pipeline, verify the output YAML contains expected header, exclusions worked, and per-ecosystem fields are present.
* Priority chain: set values at multiple levels, verify correct override behavior.

### Running tests

```bash
# Config package tests
go test ./lib/config/ -count=1 -v

# Scanner package tests (includes new Generate tests)
go test ./lib/scanner/ -count=1 -v

# Full project
go test ./... -count=1 -v
```
