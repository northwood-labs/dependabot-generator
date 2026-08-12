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

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// LoadConfig resolves configuration from all sources in priority order,
// applying built-in defaults for any unset values. The resolution order is: CLI
// flag > env var > local config > user config > global config > built-in
// defaults. If both CLIHeader and CLIHeaderFile are non-empty, CLIHeader takes
// precedence.
func LoadConfig(opts *LoadOptions) (*Config, error) {
	cfg := &Config{
		HeaderComment:     DefaultHeaderURL,
		IgnoreDirs:        copyStringSlice(DefaultIgnoreDirs),
		EcosystemDefaults: copyEcosystemDefaults(DefaultEcosystemSettings),
	}

	// Load config files from lowest to highest priority. Each layer overrides
	// the previous.
	globalPath := filepath.Join("etc", "dependabot-generator", "config.toml")

	loadErr := applyFileConfig(cfg, globalPath)
	if loadErr != nil {
		return nil, fmt.Errorf("loading global config: %w", loadErr)
	}

	userPath := userConfigPath()

	loadErr = applyFileConfig(cfg, userPath)
	if loadErr != nil {
		return nil, fmt.Errorf("loading user config: %w", loadErr)
	}

	localPath := filepath.Join(opts.ScanPath, ".depgen.toml")

	loadErr = applyFileConfig(cfg, localPath)
	if loadErr != nil {
		return nil, fmt.Errorf("loading local config: %w", loadErr)
	}

	// Apply environment variable (overrides config files).
	if opts.EnvHeader != "" {
		cfg.HeaderComment = opts.EnvHeader
	}

	// Apply CLI flags (highest priority). CLIHeader takes precedence over
	// CLIHeaderFile when both are set.
	if opts.CLIHeaderFile != "" {
		cfg.HeaderComment = opts.CLIHeaderFile
	}

	if opts.CLIHeader != "" {
		cfg.HeaderComment = opts.CLIHeader
	}

	return cfg, nil
}

// Validate checks that the resolved config is internally consistent. It
// verifies that all ignore-dir patterns are well-formed according to
// [filepath.Match] semantics.
func Validate(cfg *Config) error {
	for _, pattern := range cfg.IgnoreDirs {
		_, matchErr := filepath.Match(pattern, "test")
		if matchErr != nil {
			return fmt.Errorf("%w: %s", ErrConfigParse, pattern)
		}
	}

	return nil
}

// userConfigPath returns the path to the user-level config file. It respects
// $XDG_CONFIG_HOME and falls back to $HOME/.config per the XDG Base Directory
// Specification.
func userConfigPath() string {
	xdgHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgHome == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return ""
		}

		xdgHome = filepath.Join(home, ".config")
	}

	return filepath.Join(
		xdgHome, "dependabot-generator", "config.toml",
	)
}

// applyFileConfig reads and parses a TOML config file at the given path, then
// applies its values to cfg. If the file does not exist, it is silently
// skipped. If the file exists but cannot be read or parsed, an error is
// returned.
func applyFileConfig(cfg *Config, path string) error {
	if path == "" {
		return nil
	}

	data, readErr := os.ReadFile(path) // lint:allow_dynamic_filename
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("%w: %s", ErrConfigRead, path)
	}

	fc := &FileConfig{}

	parseErr := toml.Unmarshal(data, fc)
	if parseErr != nil {
		return fmt.Errorf("%w: %s", ErrConfigParse, path)
	}

	if fc.Header != "" {
		cfg.HeaderComment = fc.Header
	}

	if len(fc.IgnoreDirs) > 0 {
		cfg.IgnoreDirs = fc.IgnoreDirs
	}

	if len(fc.Ecosystems) > 0 {
		for name, fields := range fc.Ecosystems {
			cfg.EcosystemDefaults[name] = EcosystemConfig{
				Fields: fields,
			}
		}
	}

	return nil
}

// copyStringSlice returns a shallow copy of a string slice to prevent mutation
// of the built-in defaults.
func copyStringSlice(src []string) []string {
	dst := make([]string, len(src))
	copy(dst, src)

	return dst
}

// copyEcosystemDefaults returns a deep copy of the ecosystem defaults map to
// prevent mutation of the built-in defaults.
func copyEcosystemDefaults(src map[string]EcosystemConfig) map[string]EcosystemConfig {
	dst := make(map[string]EcosystemConfig, len(src))

	for k, v := range src {
		fields := make(map[string]any, len(v.Fields))
		maps.Copy(fields, v.Fields)

		dst[k] = EcosystemConfig{Fields: fields}
	}

	return dst
}
