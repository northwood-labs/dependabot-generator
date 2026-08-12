// Copyright 2026, Northwood Labs, LLC <license@northwood-labs.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import "errors"

const (
	// DefaultHeaderURL is the default comment text inserted into documentation
	// for Dependabot configuration options. generated Dependabot configuration
	// files when no user-provided header is specified. It points to the
	// official GitHub.
	DefaultHeaderURL = "https://docs.github.com/github/administering" +
		"-a-repository/configuration-options-for-dependency-updates"

	// DefaultEcosystemKey is the key used in EcosystemDefaults to store the
	// fallback configuration that applies to all ecosystems unless a more
	// specific override exists.
	DefaultEcosystemKey = "_default"
)

var (
	// ErrConfigParse indicates that a configuration file could not be parsed
	// due to invalid syntax or structure.
	ErrConfigParse = errors.New("failed to parse config file")

	// ErrConfigRead indicates that a configuration file could not be read from
	// the filesystem.
	ErrConfigRead = errors.New("failed to read config file")

	// DefaultIgnoreDirs is the default set of directory patterns excluded from
	// scanning. These patterns use [filepath.Match] name starting with a dot
	// (hidden directories). semantics: exact names match literally, and ".*"
	// matches any.
	DefaultIgnoreDirs = []string{
		"node_modules",
		".venv",
		"venv",
		"vendor",
		".*",
	}

	// DefaultEcosystemSettings provides the built-in per-ecosystem
	// configuration that applies to all ecosystems via the _default key. These
	// settings are compiled into the binary and used when no configuration file
	// overrides them.
	DefaultEcosystemSettings = map[string]EcosystemConfig{
		DefaultEcosystemKey: {
			Fields: map[string]any{
				"insecure-external-code-execution": "deny",
				"schedule.interval":                "monthly",
				"cooldown.default-days":            7,
				"groups.monthly-batch.patterns":    []string{"*"},
			},
		},
	}
)

type (
	// Config holds the fully-resolved configuration after merging all sources
	// according to priority rules.
	Config struct {
		EcosystemDefaults map[string]EcosystemConfig
		HeaderComment     string
		IgnoreDirs        []string
	}

	// EcosystemConfig holds additional Dependabot v2 fields for a specific
	// ecosystem. The Fields map is keyed by dotted path (e.g.,
	// "schedule.interval") and maps to arbitrary YAML values.
	EcosystemConfig struct {
		Fields map[string]any
	}

	// FileConfig represents the structure of a single TOML config file before
	// merging. This maps directly to the TOML schema.
	FileConfig struct {
		Ecosystems map[string]map[string]any `toml:"ecosystems"`
		Header     string                    `toml:"header"`
		IgnoreDirs []string                  `toml:"ignore-dirs"`
	}

	// LoadOptions bundles the inputs needed to resolve configuration.
	LoadOptions struct {
		CLIHeader     string
		CLIHeaderFile string
		EnvHeader     string
		ScanPath      string
	}
)
