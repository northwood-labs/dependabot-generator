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
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfig_BuiltInDefaults verifies that when no config files exist and
// no CLI/env values are provided, the built-in defaults are returned.
func TestLoadConfig_BuiltInDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cfg, err := LoadConfig(&LoadOptions{
		ScanPath: dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeaderComment != DefaultHeaderURL {
		t.Fatalf(
			"expected default header %q, got %q",
			DefaultHeaderURL, cfg.HeaderComment,
		)
	}

	if len(cfg.IgnoreDirs) != len(DefaultIgnoreDirs) {
		t.Fatalf(
			"expected %d ignore dirs, got %d",
			len(DefaultIgnoreDirs), len(cfg.IgnoreDirs),
		)
	}

	for i, d := range cfg.IgnoreDirs {
		if d != DefaultIgnoreDirs[i] {
			t.Fatalf(
				"ignore dir [%d]: expected %q, got %q",
				i, DefaultIgnoreDirs[i], d,
			)
		}
	}

	defaultEco, ok := cfg.EcosystemDefaults[DefaultEcosystemKey]
	if !ok {
		t.Fatal("expected _default ecosystem config")
	}

	interval, ok := defaultEco.Fields["schedule.interval"]
	if !ok || interval != "monthly" {
		t.Fatalf(
			"expected schedule.interval=monthly, got %v", interval,
		)
	}
}

// TestLoadConfig_LocalConfig verifies that a .depgen.toml in the scan path
// overrides built-in defaults.
func TestLoadConfig_LocalConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	content := []byte(`header = "local header value"` + "\n")

	writeErr := os.WriteFile(
		filepath.Join(dir, ".depgen.toml"), content, 0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}

	cfg, err := LoadConfig(&LoadOptions{
		ScanPath: dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeaderComment != "local header value" {
		t.Fatalf(
			"expected local header, got %q", cfg.HeaderComment,
		)
	}
}

// TestLoadConfig_UserConfig verifies that setting XDG_CONFIG_HOME to a temp
// directory with a config file overrides built-in defaults.
func TestLoadConfig_UserConfig(t *testing.T) {
	dir := t.TempDir()
	scanDir := t.TempDir()

	xdgDir := filepath.Join(dir, "dependabot-generator")

	mkdirErr := os.MkdirAll(xdgDir, 0o0755)
	if mkdirErr != nil {
		t.Fatalf("failed to create xdg dir: %v", mkdirErr)
	}

	content := []byte(`header = "user header value"` + "\n")

	writeErr := os.WriteFile(
		filepath.Join(xdgDir, "config.toml"), content, 0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}

	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg, err := LoadConfig(&LoadOptions{
		ScanPath: scanDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeaderComment != "user header value" {
		t.Fatalf(
			"expected user header, got %q", cfg.HeaderComment,
		)
	}
}

// TestLoadConfig_GlobalConfig verifies that the function does not error when
// the global config path does not exist (the common case).
func TestLoadConfig_GlobalConfig(t *testing.T) {
	dir := t.TempDir()

	// Ensure no user config interferes.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := LoadConfig(&LoadOptions{
		ScanPath: dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to built-in default.
	if cfg.HeaderComment != DefaultHeaderURL {
		t.Fatalf(
			"expected default header, got %q", cfg.HeaderComment,
		)
	}
}

// TestLoadConfig_PriorityChain verifies the full priority chain: CLI > env >
// local > user > global > built-in.
func TestLoadConfig_PriorityChain(t *testing.T) {
	// Set up user config.
	xdgDir := t.TempDir()
	appDir := filepath.Join(xdgDir, "dependabot-generator")

	mkdirErr := os.MkdirAll(appDir, 0o0755)
	if mkdirErr != nil {
		t.Fatalf("failed to create app dir: %v", mkdirErr)
	}

	writeErr := os.WriteFile(
		filepath.Join(appDir, "config.toml"),
		[]byte(`header = "user level"`+"\n"),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write user config: %v", writeErr)
	}

	// Set up local config.
	localDir := t.TempDir()

	writeErr = os.WriteFile(
		filepath.Join(localDir, ".depgen.toml"),
		[]byte(`header = "local level"`+"\n"),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write local config: %v", writeErr)
	}

	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	// Local beats user.
	cfg, err := LoadConfig(&LoadOptions{
		ScanPath: localDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeaderComment != "local level" {
		t.Fatalf("expected local level, got %q", cfg.HeaderComment)
	}

	// Env beats local.
	cfg, err = LoadConfig(&LoadOptions{
		ScanPath:  localDir,
		EnvHeader: "env level",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeaderComment != "env level" {
		t.Fatalf("expected env level, got %q", cfg.HeaderComment)
	}

	// CLI header-file beats env.
	cfg, err = LoadConfig(&LoadOptions{
		ScanPath:      localDir,
		EnvHeader:     "env level",
		CLIHeaderFile: "cli-file level",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeaderComment != "cli-file level" {
		t.Fatalf(
			"expected cli-file level, got %q", cfg.HeaderComment,
		)
	}

	// CLI header beats cli-file.
	cfg, err = LoadConfig(&LoadOptions{
		ScanPath:      localDir,
		EnvHeader:     "env level",
		CLIHeaderFile: "cli-file level",
		CLIHeader:     "cli level",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeaderComment != "cli level" {
		t.Fatalf("expected cli level, got %q", cfg.HeaderComment)
	}
}

// TestLoadConfig_CLIHeaderOverridesAll verifies that CLIHeader takes precedence
// over all other sources.
func TestLoadConfig_CLIHeaderOverridesAll(t *testing.T) {
	// Set up user config.
	xdgDir := t.TempDir()
	appDir := filepath.Join(xdgDir, "dependabot-generator")

	mkdirErr := os.MkdirAll(appDir, 0o0755)
	if mkdirErr != nil {
		t.Fatalf("failed to create app dir: %v", mkdirErr)
	}

	writeErr := os.WriteFile(
		filepath.Join(appDir, "config.toml"),
		[]byte(`header = "user"`+"\n"),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write user config: %v", writeErr)
	}

	// Set up local config.
	localDir := t.TempDir()

	writeErr = os.WriteFile(
		filepath.Join(localDir, ".depgen.toml"),
		[]byte(`header = "local"`+"\n"),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write local config: %v", writeErr)
	}

	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	cfg, err := LoadConfig(&LoadOptions{
		ScanPath:      localDir,
		EnvHeader:     "env",
		CLIHeaderFile: "cli-file",
		CLIHeader:     "cli wins",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeaderComment != "cli wins" {
		t.Fatalf("expected cli wins, got %q", cfg.HeaderComment)
	}
}

// TestLoadConfig_CLIHeaderFileOverridesEnv verifies that CLIHeaderFile takes
// precedence over the environment variable.
func TestLoadConfig_CLIHeaderFileOverridesEnv(t *testing.T) {
	dir := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := LoadConfig(&LoadOptions{
		ScanPath:      dir,
		EnvHeader:     "env value",
		CLIHeaderFile: "file value",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeaderComment != "file value" {
		t.Fatalf("expected file value, got %q", cfg.HeaderComment)
	}
}

// TestLoadConfig_EnvOverridesConfig verifies that the env var overrides local
// config.
func TestLoadConfig_EnvOverridesConfig(t *testing.T) {
	localDir := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	writeErr := os.WriteFile(
		filepath.Join(localDir, ".depgen.toml"),
		[]byte(`header = "local value"`+"\n"),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}

	cfg, err := LoadConfig(&LoadOptions{
		ScanPath:  localDir,
		EnvHeader: "env value",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeaderComment != "env value" {
		t.Fatalf("expected env value, got %q", cfg.HeaderComment)
	}
}

// TestLoadConfig_InvalidTOML verifies that a malformed TOML file returns
// ErrConfigParse.
func TestLoadConfig_InvalidTOML(t *testing.T) {
	dir := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	writeErr := os.WriteFile(
		filepath.Join(dir, ".depgen.toml"),
		[]byte("this is not valid [[[toml\n"),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}

	_, err := LoadConfig(&LoadOptions{
		ScanPath: dir,
	})
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}

	if !errors.Is(err, ErrConfigParse) {
		t.Fatalf("expected ErrConfigParse, got: %v", err)
	}
}

// TestLoadConfig_UnreadableFile verifies that a file with no read permissions
// returns ErrConfigRead.
func TestLoadConfig_UnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping unreadable test when running as root")
	}

	dir := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	configPath := filepath.Join(dir, ".depgen.toml")

	writeErr := os.WriteFile(
		configPath, []byte(`header = "test"`+"\n"), 0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}

	chmodErr := os.Chmod(configPath, 0o0000)
	if chmodErr != nil {
		t.Fatalf("failed to chmod: %v", chmodErr)
	}

	t.Cleanup(func() {
		restoreErr := os.Chmod(configPath, 0o0666) // lint:allow_666
		if restoreErr != nil {
			t.Logf("cleanup chmod failed: %v", restoreErr)
		}
	})

	_, err := LoadConfig(&LoadOptions{
		ScanPath: dir,
	})
	if err == nil {
		t.Fatal("expected error for unreadable file, got nil")
	}

	if !errors.Is(err, ErrConfigRead) {
		t.Fatalf("expected ErrConfigRead, got: %v", err)
	}
}

// TestValidate_WellFormedPatterns verifies that valid [filepath.Match] patterns
// pass validation.
func TestValidate_WellFormedPatterns(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		IgnoreDirs: []string{
			"node_modules",
			".venv",
			"venv",
			"vendor",
			".*",
		},
	}

	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// TestValidate_MalformedPattern verifies that a malformed [filepath.Match]
// pattern returns an error.
func TestValidate_MalformedPattern(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		IgnoreDirs: []string{"["},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for malformed pattern, got nil")
	}

	if !errors.Is(err, ErrConfigParse) {
		t.Fatalf("expected ErrConfigParse, got: %v", err)
	}
}

// TestLoadConfig_XDGConfigHomeFallback verifies that when XDG_CONFIG_HOME is
// unset, the fallback to $HOME/.config works.
func TestLoadConfig_XDGConfigHomeFallback(t *testing.T) {
	homeDir := t.TempDir()
	scanDir := t.TempDir()

	// Clear XDG_CONFIG_HOME to trigger fallback.
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", homeDir)

	appDir := filepath.Join(
		homeDir, ".config", "dependabot-generator",
	)

	mkdirErr := os.MkdirAll(appDir, 0o0755)
	if mkdirErr != nil {
		t.Fatalf("failed to create app dir: %v", mkdirErr)
	}

	content := []byte(`header = "fallback header"` + "\n")

	writeErr := os.WriteFile(
		filepath.Join(appDir, "config.toml"), content, 0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}

	cfg, err := LoadConfig(&LoadOptions{
		ScanPath: scanDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeaderComment != "fallback header" {
		t.Fatalf(
			"expected fallback header, got %q",
			cfg.HeaderComment,
		)
	}
}

// TestLoadConfig_EcosystemOverrides verifies that a config file with
// ecosystem-specific settings overrides the _default.
func TestLoadConfig_EcosystemOverrides(t *testing.T) {
	dir := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	content := []byte(`[ecosystems.npm]
"schedule.interval" = "weekly"
`)

	writeErr := os.WriteFile(
		filepath.Join(dir, ".depgen.toml"), content, 0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}

	cfg, err := LoadConfig(&LoadOptions{
		ScanPath: dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	npmCfg, ok := cfg.EcosystemDefaults["npm"]
	if !ok {
		t.Fatal("expected npm ecosystem config")
	}

	interval, ok := npmCfg.Fields["schedule.interval"]
	if !ok || interval != "weekly" {
		t.Fatalf("expected schedule.interval=weekly, got %v", interval)
	}

	// Verify _default is still present.
	_, ok = cfg.EcosystemDefaults[DefaultEcosystemKey]
	if !ok {
		t.Fatal("expected _default ecosystem config to remain")
	}
}

// TestLoadConfig_IgnoreDirsOverride verifies that a config file with custom
// ignore-dirs replaces (not appends to) the defaults.
func TestLoadConfig_IgnoreDirsOverride(t *testing.T) {
	dir := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	content := []byte(`ignore-dirs = ["custom_dir", "another"]` + "\n")

	writeErr := os.WriteFile(
		filepath.Join(dir, ".depgen.toml"), content, 0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}

	cfg, err := LoadConfig(&LoadOptions{
		ScanPath: dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.IgnoreDirs) != 2 {
		t.Fatalf("expected 2 ignore dirs, got %d", len(cfg.IgnoreDirs))
	}

	if cfg.IgnoreDirs[0] != "custom_dir" {
		t.Fatalf(
			"expected custom_dir, got %q", cfg.IgnoreDirs[0],
		)
	}

	if cfg.IgnoreDirs[1] != "another" {
		t.Fatalf("expected another, got %q", cfg.IgnoreDirs[1])
	}
}
