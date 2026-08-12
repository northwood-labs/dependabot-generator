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

// Integration tests wire together config.LoadConfig + scanner.Scan +
// scanner.Generate — the same pipeline as cmd/run.go — without Cobra.
// They verify end-to-end behavior: config loading, directory
// exclusion, header comment injection, per-ecosystem field merging,
// priority chain resolution, and backward compatibility.

package scanner // lint:allow_naming_conflict_stdlib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.nwlabs.dev/dependabot-generator/lib/config"
)

// TestIntegration_FullPipelineWithConfig creates a fixture tree with
// a .depgen.toml containing a custom header and default ignore-dirs,
// then runs LoadConfig → Scan → Generate and verifies that the
// output contains the custom header comment, node_modules is
// excluded, and per-ecosystem fields are present.
func TestIntegration_FullPipelineWithConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	setupFullPipelineFixture(t, root)

	// Load and validate config.
	cfg, loadErr := config.LoadConfig(&config.LoadOptions{
		ScanPath: root,
	})
	if loadErr != nil {
		t.Fatalf("LoadConfig failed: %v", loadErr)
	}

	validateErr := config.Validate(cfg)
	if validateErr != nil {
		t.Fatalf("Validate failed: %v", validateErr)
	}

	// Scan with ignore dirs from config.
	results, scanErr := Scan(root, cfg.IgnoreDirs)
	if scanErr != nil {
		t.Fatalf("Scan failed: %v", scanErr)
	}

	// Verify exclusion and detection.
	assertNoResultContains(t, results, "node_modules")
	assertEcosystemAtDir(t, results, "gomod", "/")

	// Generate output with config applied.
	output := generateWithConfig(t, results, cfg)

	// Verify output structure.
	assertOutputHasPrefix(t, output, "---\n")
	assertOutputContains(t, output, "# Custom integration test header")
	assertOutputContains(t, output, "schedule:")
	assertOutputContains(t, output, "interval: monthly")
	assertOutputContains(t, output, "version: 2")
	assertOutputContains(t, output, "updates:")
	assertOutputContains(t, output, "package-ecosystem: gomod")
}

// setupFullPipelineFixture creates the fixture tree for the full
// pipeline test: go.mod at root, node_modules with package.json,
// and a .depgen.toml config file.
func setupFullPipelineFixture(t *testing.T, root string) {
	t.Helper()

	writeErr := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module test\n"),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write go.mod: %v", writeErr)
	}

	nmDir := filepath.Join(root, "node_modules")

	mkErr := os.MkdirAll(nmDir, 0o0755)
	if mkErr != nil {
		t.Fatalf("failed to create node_modules: %v", mkErr)
	}

	writeErr = os.WriteFile(
		filepath.Join(nmDir, "package.json"),
		[]byte("{}"),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write package.json: %v", writeErr)
	}

	tomlContent := `header = "Custom integration test header"
ignore-dirs = ["node_modules"]

[ecosystems._default]
"schedule.interval" = "monthly"
`

	writeErr = os.WriteFile(
		filepath.Join(root, ".depgen.toml"),
		[]byte(tomlContent),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write .depgen.toml: %v", writeErr)
	}
}

// TestIntegration_BackwardCompatibility scans a directory with a
// go.mod, calls Generate with nil opts, and verifies the output is
// valid YAML with `---`, `version: 2`, an `updates` array, and no
// comment lines — matching the original behavior before the header
// comment feature.
func TestIntegration_BackwardCompatibility(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Create go.mod at root.
	writeErr := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module test\n"),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write go.mod: %v", writeErr)
	}

	// Scan with nil ignoreDirs (original behavior).
	results, scanErr := Scan(root, nil)
	if scanErr != nil {
		t.Fatalf("Scan failed: %v", scanErr)
	}

	// Generate with nil opts (original behavior).
	output, genErr := Generate(results, nil)
	if genErr != nil {
		t.Fatalf("Generate failed: %v", genErr)
	}

	// Must start with ---.
	if !strings.HasPrefix(output, "---\n") {
		t.Fatal("expected output to start with ---")
	}

	// Must contain version: 2.
	if !strings.Contains(output, "version: 2") {
		t.Fatal("expected version: 2 in output")
	}

	// Must contain updates array.
	if !strings.Contains(output, "updates:") {
		t.Fatal("expected updates: in output")
	}

	// Must NOT contain any comment lines.
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		if i == 0 {
			continue
		}

		if strings.HasPrefix(line, "#") {
			t.Fatalf(
				"expected no comment lines with nil opts, found: %q",
				line,
			)
		}
	}
}

// TestIntegration_PriorityChainEndToEnd uses config.LoadConfig with
// multiple sources set and verifies the correct winner appears in
// the generated output. The CLI flag (CLIHeader) should override the
// local config file value.
func TestIntegration_PriorityChainEndToEnd(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Create go.mod at root.
	writeErr := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module test\n"),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write go.mod: %v", writeErr)
	}

	// Create .depgen.toml with a file-level header (low priority).
	tomlContent := `header = "File-level header from config"
`

	writeErr = os.WriteFile(
		filepath.Join(root, ".depgen.toml"),
		[]byte(tomlContent),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write .depgen.toml: %v", writeErr)
	}

	// Load config with CLIHeader set (highest priority).
	cfg, loadErr := config.LoadConfig(&config.LoadOptions{
		ScanPath:  root,
		CLIHeader: "CLI flag wins over config file",
	})
	if loadErr != nil {
		t.Fatalf("LoadConfig failed: %v", loadErr)
	}

	// Scan.
	results, scanErr := Scan(root, cfg.IgnoreDirs)
	if scanErr != nil {
		t.Fatalf("Scan failed: %v", scanErr)
	}

	// Convert ecosystem defaults.
	ecoDefaults := make(
		map[string]EcosystemSettings, len(cfg.EcosystemDefaults),
	)

	for k, v := range cfg.EcosystemDefaults {
		ecoDefaults[k] = EcosystemSettings{Fields: v.Fields}
	}

	// Generate.
	genOpts := &GenerateOptions{
		CommentText:       cfg.HeaderComment,
		EcosystemDefaults: ecoDefaults,
	}

	output, genErr := Generate(results, genOpts)
	if genErr != nil {
		t.Fatalf("Generate failed: %v", genErr)
	}

	// CLI header should win.
	if !strings.Contains(output, "# CLI flag wins over config file") {
		t.Fatalf(
			"expected CLI header in output, got:\n%s", output,
		)
	}

	// Config file header should NOT appear.
	if strings.Contains(output, "File-level header from config") {
		t.Fatal(
			"config file header should not appear when CLI flag is set",
		)
	}

	// Also verify env var priority over config file.
	cfgEnv, loadErr := config.LoadConfig(&config.LoadOptions{
		ScanPath:  root,
		EnvHeader: "Env var wins over config file",
	})
	if loadErr != nil {
		t.Fatalf("LoadConfig (env) failed: %v", loadErr)
	}

	if cfgEnv.HeaderComment != "Env var wins over config file" {
		t.Fatalf(
			"expected env header %q, got %q",
			"Env var wins over config file",
			cfgEnv.HeaderComment,
		)
	}

	// CLI should still win over env.
	cfgBoth, loadErr := config.LoadConfig(&config.LoadOptions{
		ScanPath:  root,
		CLIHeader: "CLI wins over everything",
		EnvHeader: "Env var loses to CLI",
	})
	if loadErr != nil {
		t.Fatalf("LoadConfig (both) failed: %v", loadErr)
	}

	if cfgBoth.HeaderComment != "CLI wins over everything" {
		t.Fatalf(
			"expected CLI header %q, got %q",
			"CLI wins over everything",
			cfgBoth.HeaderComment,
		)
	}
}

// TestIntegration_DirectoryExclusionEndToEnd creates a tree with
// excluded and non-excluded directories, scans with ignoreDirs from
// config, and verifies only non-excluded results appear in the
// generated YAML.
func TestIntegration_DirectoryExclusionEndToEnd(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	setupExclusionFixture(t, root)

	// Load config.
	cfg, loadErr := config.LoadConfig(&config.LoadOptions{
		ScanPath: root,
	})
	if loadErr != nil {
		t.Fatalf("LoadConfig failed: %v", loadErr)
	}

	// Scan with the config's ignore dirs.
	results, scanErr := Scan(root, cfg.IgnoreDirs)
	if scanErr != nil {
		t.Fatalf("Scan failed: %v", scanErr)
	}

	// Verify excluded directories do not appear.
	assertNoResultContains(t, results, "vendor")
	assertNoResultContains(t, results, ".cache")

	// Verify app/ IS present.
	assertEcosystemAtDir(t, results, "gomod", "/app")

	// Generate output and verify excluded dirs are not in YAML.
	output := generateWithConfig(t, results, cfg)

	// vendor and .cache should not appear in the generated YAML.
	if strings.Contains(output, "vendor") {
		t.Fatalf(
			"vendor should not appear in output, got:\n%s", output,
		)
	}

	if strings.Contains(output, ".cache") {
		t.Fatalf(
			".cache should not appear in output, got:\n%s", output,
		)
	}

	// /app should be present in the output.
	assertOutputContains(t, output, "/app")
	assertOutputContains(t, output, "package-ecosystem: gomod")
}

// setupExclusionFixture creates the fixture tree for the directory
// exclusion test: app/ (included), vendor/ (excluded), .cache/
// (excluded by glob), and a .depgen.toml config.
func setupExclusionFixture(t *testing.T, root string) {
	t.Helper()

	// Create app/ with go.mod (should be included).
	appDir := filepath.Join(root, "app")

	mkErr := os.MkdirAll(appDir, 0o0755)
	if mkErr != nil {
		t.Fatalf("failed to create app/: %v", mkErr)
	}

	writeErr := os.WriteFile(
		filepath.Join(appDir, "go.mod"),
		[]byte("module app\n"),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write app/go.mod: %v", writeErr)
	}

	// Create vendor/ with go.mod (should be excluded).
	vendorDir := filepath.Join(root, "vendor")

	mkErr = os.MkdirAll(vendorDir, 0o0755)
	if mkErr != nil {
		t.Fatalf("failed to create vendor/: %v", mkErr)
	}

	writeErr = os.WriteFile(
		filepath.Join(vendorDir, "go.mod"),
		[]byte("module vendor\n"),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write vendor/go.mod: %v", writeErr)
	}

	// Create .cache/ with go.mod (should be excluded by .* pattern).
	cacheDir := filepath.Join(root, ".cache")

	mkErr = os.MkdirAll(cacheDir, 0o0755)
	if mkErr != nil {
		t.Fatalf("failed to create .cache/: %v", mkErr)
	}

	writeErr = os.WriteFile(
		filepath.Join(cacheDir, "go.mod"),
		[]byte("module cache\n"),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write .cache/go.mod: %v", writeErr)
	}

	// Create .depgen.toml with ignore-dirs.
	tomlContent := `ignore-dirs = ["vendor", ".*"]
`

	writeErr = os.WriteFile(
		filepath.Join(root, ".depgen.toml"),
		[]byte(tomlContent),
		0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write .depgen.toml: %v", writeErr)
	}
}

// ---------------------------------------------------------------------------
// Shared integration test helpers
// ---------------------------------------------------------------------------.

// generateWithConfig converts config ecosystem defaults to scanner
// settings and calls Generate, returning the output string. It fails
// the test on error.
func generateWithConfig(
	t *testing.T, results []ScanResult, cfg *config.Config,
) string {
	t.Helper()

	ecoDefaults := make(
		map[string]EcosystemSettings, len(cfg.EcosystemDefaults),
	)

	for k, v := range cfg.EcosystemDefaults {
		ecoDefaults[k] = EcosystemSettings{Fields: v.Fields}
	}

	genOpts := &GenerateOptions{
		CommentText:       cfg.HeaderComment,
		EcosystemDefaults: ecoDefaults,
	}

	output, genErr := Generate(results, genOpts)
	if genErr != nil {
		t.Fatalf("Generate failed: %v", genErr)
	}

	return output
}

// assertNoResultContains verifies that no scan result's Directory
// field contains the given substring.
func assertNoResultContains(
	t *testing.T, results []ScanResult, substr string,
) {
	t.Helper()

	for _, r := range results {
		if strings.Contains(r.Directory, substr) {
			t.Fatalf(
				"%s should be excluded, got: %+v", substr, r,
			)
		}
	}
}

// assertEcosystemAtDir verifies that at least one scan result
// matches the given ecosystem and directory.
func assertEcosystemAtDir(
	t *testing.T, results []ScanResult, ecosystem, dir string,
) {
	t.Helper()

	for _, r := range results {
		if r.Ecosystem == ecosystem && r.Directory == dir {
			return
		}
	}

	t.Fatalf("expected %s at %s, got: %+v", ecosystem, dir, results)
}

// assertOutputHasPrefix verifies the output string starts with the
// expected prefix.
func assertOutputHasPrefix(t *testing.T, output, expected string) {
	t.Helper()

	if !strings.HasPrefix(output, expected) {
		t.Fatalf(
			"expected output to start with %q, got:\n%s",
			expected, output,
		)
	}
}

// assertOutputContains verifies the output string contains the
// expected substring.
func assertOutputContains(t *testing.T, output, expected string) {
	t.Helper()

	if !strings.Contains(output, expected) {
		t.Fatalf(
			"expected %q in output, got:\n%s", expected, output,
		)
	}
}
