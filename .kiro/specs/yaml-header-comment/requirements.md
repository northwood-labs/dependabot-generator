# Requirements Document

## Introduction

This feature adds optional header comment injection to the `dependabot-generator` CLI tool. Users can provide custom text that the Generator inserts as YAML comments between the opening document separator (`---`) and the `version: 2` key in the generated output. Comment text is sourced through a layered configuration system with defined precedence: CLI flags, environment variables, and configuration files at local, user, and global levels. The tool enforces text wrapping at 80 characters and an 8,192 byte size limit on comment content.

## Glossary

* **Generator**: The component that accepts scan results and produces a valid Dependabot v2 YAML configuration file (the `Generate` function in `lib/scanner/scanner.go`).
* **Header_Comment**: One or more lines of YAML comment text (prefixed with `#`) placed between the `---` document separator and the `version` key in the generated output.
* **Comment_Source**: The mechanism through which a user provides raw comment text — via a CLI flag, an environment variable, or a configuration file entry.
* **Comment_Text**: The raw text content provided by the user, before any formatting or `#` prefix normalization.
* **Run_Command**: The `run` subcommand of the CLI that invokes scanning and generation.
* **Config_File**: A TOML or YAML configuration file that provides settings for the tool, including comment header text and directories to ignore during scanning.
* **Local_Config**: A configuration file located in the project directory (e.g., `.dependabot-generator.toml` in the repository root being scanned).
* **User_Config**: A configuration file located in the user's home directory (e.g., `~/.config/dependabot-generator/config.toml`).
* **Global_Config**: A configuration file located at the system-wide configuration path (e.g., `/etc/dependabot-generator/config.toml`).
* **Input_Priority**: The precedence order for resolving configuration values: CLI flag, then environment variable, then Local_Config, then User_Config, then Global_Config.
* **Wrap_Limit**: The 80-character hard line length limit for prose text in comment output.

## Requirements

### Requirement 1: Inline header comment via CLI flag

**User Story:** As a user, I want to provide a short header comment directly on the command line, so that I can add simple annotations without creating a separate file.

#### Acceptance criteria

1. WHEN the `--header` flag is provided with a non-empty string value, THE Generator SHALL use the value as Comment_Text.
2. WHEN the `--header` flag is provided, THE Run_Command SHALL use it as the highest-priority Comment_Source, overriding all other sources.
3. THE Run_Command SHALL register a `--header` string flag with an empty string default value.

### Requirement 2: File-based header comment via CLI flag

**User Story:** As a user, I want to point the tool at a file containing my header comment text, so that I can maintain multi-line comments without complex shell quoting.

#### Acceptance criteria

1. WHEN the `--header-file` flag is provided with a valid file path, THE Run_Command SHALL read the file contents and use them as Comment_Text at the same priority level as `--header`.
2. WHEN the `--header-file` flag value is not a well-formed file path, THE Run_Command SHALL return a descriptive error indicating the path format is invalid.
3. WHEN the `--header-file` flag points to a file that does not exist, THE Run_Command SHALL return a descriptive error indicating the file was not found.
4. WHEN the `--header-file` flag points to a file that is not readable, THE Run_Command SHALL return a descriptive error indicating the file could not be read.
5. THE Run_Command SHALL validate the `--header-file` path for well-formedness before attempting file access.
6. THE Run_Command SHALL register a `--header-file` string flag with an empty string default value.

### Requirement 3: CLI flag mutual exclusivity

**User Story:** As a user, I want the tool to reject ambiguous input when both inline and file-based CLI flags are provided, so that I do not accidentally get unexpected output.

#### Acceptance criteria

1. WHEN both `--header` and `--header-file` flags are provided with non-empty values, THE Run_Command SHALL return a descriptive error and stop execution without producing output.
2. WHEN neither CLI flags, environment variables, nor configuration files provide Comment_Text, THE Generator SHALL produce output with no Header_Comment.

### Requirement 4: Environment variable comment source

**User Story:** As a user, I want to set the header comment via an environment variable, so that I can configure it in CI pipelines or shell profiles without modifying files.

#### Acceptance criteria

1. WHEN the designated environment variable is set to a non-empty value, THE Run_Command SHALL use the value as Comment_Text.
2. WHEN both a CLI flag and the environment variable provide Comment_Text, THE Run_Command SHALL use the CLI flag value and ignore the environment variable.
3. WHEN the environment variable is set and no CLI flag is provided, THE Run_Command SHALL use the environment variable value, overriding any configuration file values.

### Requirement 5: Configuration file support

**User Story:** As a user, I want to specify settings in configuration files at local, user, and global levels, so that I can maintain persistent defaults without repeating CLI flags.

#### Acceptance criteria

1. THE Run_Command SHALL search for configuration files at three levels: Local_Config in the scanned project directory, User_Config in the user home configuration directory, and Global_Config at the system-wide path.
2. WHEN a Local_Config file exists and contains a header comment setting, THE Run_Command SHALL use it as Comment_Text if no higher-priority source provides a value.
3. WHEN a User_Config file exists and contains a header comment setting, THE Run_Command SHALL use it as Comment_Text if no higher-priority source provides a value.
4. WHEN a Global_Config file exists and contains a header comment setting, THE Run_Command SHALL use it as Comment_Text if no higher-priority source provides a value.
5. WHEN a Config_File specifies directories to ignore, THE Run_Command SHALL exclude those directories from scanning.
6. IF exclusion logic for directory ignore patterns from a Config_File fails, THEN THE Run_Command SHALL abort the scan and return a descriptive error.
7. IF a Config_File contains invalid syntax, THEN THE Run_Command SHALL return a descriptive error indicating the file path and the nature of the syntax error.

### Requirement 6: Input priority chain

**User Story:** As a user, I want a clear precedence order for configuration sources, so that I can predictably override defaults at the appropriate level.

#### Acceptance criteria

1. THE Run_Command SHALL resolve Comment_Text using the following priority order (highest to lowest): CLI flag (`--header` or `--header-file`), environment variable, Local_Config, User_Config, Global_Config.
2. WHEN a higher-priority source provides a non-empty Comment_Text value, THE Run_Command SHALL ignore all lower-priority sources for that setting.
3. WHEN multiple configuration files exist at different levels with non-empty settings, THE Run_Command SHALL apply priority logic considering all config sources (CLI, environment variables, Local_Config, User_Config, Global_Config) and use only the value from the highest-priority source.
4. WHEN no configuration source provides a non-empty Comment_Text value, THE Run_Command SHALL fall back to Global_Config as the default source; IF no source at any level provides a value, THE Generator SHALL produce output with no Header_Comment.

### Requirement 7: Comment text wrapping

**User Story:** As a user, I want the tool to wrap comment text at 80 characters, so that the generated output is readable in standard terminal widths without manual formatting.

#### Acceptance criteria

1. THE Generator SHALL wrap prose lines of Comment_Text to a maximum of 80 characters per line (measured after the `#` prefix is applied).
2. WHEN a line of Comment_Text contains a URL, THE Generator SHALL keep the URL intact on a single line even if it exceeds 80 characters.
3. WHEN Comment_Text contains a line that is 80 characters or shorter (measured after the `#` prefix), THE Generator SHALL preserve that line as-is without joining it to adjacent lines.
4. WHEN Comment_Text contains a line longer than 80 characters that does not contain a URL, THE Generator SHALL reflow the text to fit within the 80-character limit.
5. THE Generator SHALL fill lines up to the 80-character limit rather than wrapping early at a shorter length.

### Requirement 8: Comment text size limit

**User Story:** As a user, I want the tool to enforce a size limit on comment text, so that accidentally large input does not produce an unwieldy output file.

#### Acceptance criteria

1. THE Run_Command SHALL enforce a maximum size of 8,192 bytes (inclusive) for Comment_Text input.
2. IF Comment_Text exceeds 8,192 bytes, THEN THE Run_Command SHALL return a descriptive error indicating the input exceeds the maximum allowed size.
3. THE Run_Command SHALL measure the size limit against the raw input before any formatting or wrapping is applied.

### Requirement 9: Comment text formatting

**User Story:** As a user, I want the tool to handle both raw text and pre-formatted comment text, so that I do not need to worry about whether my input already contains `#` prefixes.

#### Acceptance criteria

1. WHEN a line of Comment_Text does not start with `#`, THE Generator SHALL prepend `#` (hash followed by a space) to that line before insertion.
2. WHEN a line of Comment_Text already starts with `#`, THE Generator SHALL insert that line unchanged.
3. WHEN Comment_Text contains multiple lines, THE Generator SHALL process each line independently according to the formatting rules.
4. WHEN Comment_Text contains a trailing newline, THE Generator SHALL strip the trailing newline before processing to avoid inserting a blank comment line at the end.

### Requirement 10: Header comment placement in output

**User Story:** As a user, I want the header comment placed immediately after `---` and before `version: 2`, so that the output matches the standard YAML document structure I expect.

#### Acceptance criteria

1. WHEN Comment_Text is provided, THE Generator SHALL place the formatted Header_Comment on the line immediately following the `---` separator.
2. WHEN Comment_Text is provided, THE Generator SHALL place a blank line between the last Header_Comment line and the `version` key.
3. THE Generator SHALL produce a valid YAML document regardless of whether Comment_Text is provided.
4. IF the Generator encounters malformed input or internal errors, THEN THE Generator SHALL return an error rather than producing partial or invalid YAML output.

### Requirement 11: Generate function signature extension

**User Story:** As a developer integrating the header comment feature, I want the Generate function to accept comment text alongside scan results, so that the feature composes cleanly with the existing architecture.

#### Acceptance criteria

1. THE Generate function SHALL accept Comment_Text as a string parameter in addition to the existing slice of ScanResult.
2. WHEN Comment_Text is an empty string, THE Generate function SHALL produce output identical to the current behavior (no Header_Comment).
3. THE Generate function SHALL continue to return a string and an error.

### Requirement 12: Round-trip consistency with header comments

**User Story:** As a developer, I want the generated YAML to remain valid and parseable regardless of the header comment content, so that downstream tools can still consume the output.

#### Acceptance criteria

1. FOR ALL valid Comment_Text strings and valid slices of ScanResult, parsing the generated YAML output SHALL yield a valid Dependabot v2 configuration with the correct `version` and `updates` entries.
2. FOR ALL valid Comment_Text strings, the Header_Comment lines in the output SHALL all begin with `#` (ensuring they are valid YAML comments that do not affect parsing).

### Requirement 13: Empty and whitespace-only input handling

**User Story:** As a user, I want the tool to gracefully handle edge cases like empty strings or whitespace-only input, so that accidental blank flags do not produce malformed output.

#### Acceptance criteria

1. WHEN Comment_Text consists entirely of whitespace characters (spaces, tabs, newlines), THE Generator SHALL treat it as empty and produce no Header_Comment.
2. WHEN Comment_Text contains leading or trailing blank lines (after stripping the final newline), THE Generator SHALL preserve internal blank lines as empty comment lines (`#`).
