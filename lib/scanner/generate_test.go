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

package scanner // lint:allow_naming_conflict_stdlib

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type (
	// testConfigUpdate and testConfig exist to parse generated YAML back into
	// structured data. We verify structure rather than exact string matching
	// because string comparisons are brittle to whitespace, quoting, and
	// encoding changes that don't affect correctness.
	testConfigUpdate struct {
		PackageEcosystem string `yaml:"package-ecosystem"` // lint:allow_format
		Directory        string `yaml:"directory"`
	}

	testConfig struct {
		Updates []testConfigUpdate `yaml:"updates"`
		Version int                `yaml:"version"`
	}
)

// TestGenerate_YAMLStructure verifies the structural contract that GitHub
// expects: a document separator, version: 2, and an updates array. If any of
// these are missing, Dependabot rejects the file silently.
func TestGenerate_YAMLStructure(t *testing.T) {
	t.Parallel()

	results := []ScanResult{
		{Directory: "/", Ecosystem: "gomod"},
		{Directory: "/", Ecosystem: "github-actions"},
	}

	output, err := Generate(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(output, "---\n") {
		t.Fatal("output does not start with document separator ---")
	}

	if !strings.Contains(output, "version: 2") {
		t.Fatal("output does not contain version: 2")
	}

	if !strings.Contains(output, "updates:") {
		t.Fatal("output does not contain updates: section")
	}

	body := strings.TrimPrefix(output, "---\n")

	var cfg testConfig

	unmarshalErr := yaml.Unmarshal([]byte(body), &cfg)
	if unmarshalErr != nil {
		t.Fatalf("failed to parse YAML: %v", unmarshalErr)
	}

	if cfg.Version != 2 {
		t.Fatalf("expected version 2, got %d", cfg.Version)
	}

	if len(cfg.Updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(cfg.Updates))
	}
}

// TestGenerate_SortedOutput verifies deterministic ordering (directory
// ascending, then ecosystem ascending). This matters because users commit the
// generated file and review diffs — non-deterministic order would produce noisy
// diffs on every regeneration even when nothing actually changed.
func TestGenerate_SortedOutput(t *testing.T) {
	t.Parallel()

	results := []ScanResult{
		{Directory: "/z", Ecosystem: "npm"},
		{Directory: "/a", Ecosystem: "cargo"},
		{Directory: "/a", Ecosystem: "gomod"},
	}

	output, err := Generate(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := strings.TrimPrefix(output, "---\n")

	var cfg testConfig

	unmarshalErr := yaml.Unmarshal([]byte(body), &cfg)
	if unmarshalErr != nil {
		t.Fatalf("failed to parse YAML: %v", unmarshalErr)
	}

	if len(cfg.Updates) != 3 {
		t.Fatalf("expected 3 updates, got %d", len(cfg.Updates))
	}

	// Verify sort: /a cargo, /a gomod, /z npm.
	if cfg.Updates[0].Directory != "/a" ||
		cfg.Updates[0].PackageEcosystem != "cargo" {
		t.Fatalf("expected first entry /a cargo, got %s %s", cfg.Updates[0].Directory, cfg.Updates[0].PackageEcosystem)
	}

	if cfg.Updates[1].Directory != "/a" ||
		cfg.Updates[1].PackageEcosystem != "gomod" {
		t.Fatalf("expected second entry /a gomod, got %s %s", cfg.Updates[1].Directory, cfg.Updates[1].PackageEcosystem)
	}

	if cfg.Updates[2].Directory != "/z" ||
		cfg.Updates[2].PackageEcosystem != "npm" {
		t.Fatalf("expected third entry /z npm, got %s %s", cfg.Updates[2].Directory, cfg.Updates[2].PackageEcosystem)
	}
}

// TestGenerate_EmptyResults confirms that zero scan results produce a valid
// (parseable) config with an empty updates array rather than an error or
// malformed output. This handles the "repo has no recognized package managers"
// case gracefully.
func TestGenerate_EmptyResults(t *testing.T) {
	t.Parallel()

	var emptyResults []ScanResult

	output, err := Generate(emptyResults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(output, "---\n") {
		t.Fatal("output does not start with document separator ---")
	}

	body := strings.TrimPrefix(output, "---\n")

	var cfg testConfig

	unmarshalErr := yaml.Unmarshal([]byte(body), &cfg)
	if unmarshalErr != nil {
		t.Fatalf("failed to parse YAML: %v", unmarshalErr)
	}

	if cfg.Version != 2 {
		t.Fatalf("expected version 2, got %d", cfg.Version)
	}

	if len(cfg.Updates) != 0 {
		t.Fatalf("expected empty updates, got %d entries", len(cfg.Updates))
	}
}

// TestGenerate_NilResults tests nil separately from empty because they are
// different at the Go type level (nil slice vs allocated empty slice), but must
// behave identically for the user. This catches nil-pointer dereferences and
// nil-map panics that would only surface when Scan returns nil (e.g., on error
// paths).
func TestGenerate_NilResults(t *testing.T) {
	t.Parallel()

	output, err := Generate(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(output, "---\n") {
		t.Fatal("output does not start with document separator ---")
	}

	body := strings.TrimPrefix(output, "---\n")

	var cfg testConfig

	unmarshalErr := yaml.Unmarshal([]byte(body), &cfg)
	if unmarshalErr != nil {
		t.Fatalf("failed to parse YAML: %v", unmarshalErr)
	}

	if cfg.Version != 2 {
		t.Fatalf("expected version 2, got %d", cfg.Version)
	}

	if len(cfg.Updates) != 0 {
		t.Fatalf("expected empty updates for nil input, got %d entries", len(cfg.Updates))
	}
}

// TestGenerate_MultipleEcosystemsSameDirectory verifies that multiple
// ecosystems in one directory each get their own entry (no deduplication) and
// that entries within the same directory are sorted alphabetically by ecosystem
// name.
func TestGenerate_MultipleEcosystemsSameDirectory(t *testing.T) {
	t.Parallel()

	results := []ScanResult{
		{Directory: "/", Ecosystem: "gomod"},
		{Directory: "/", Ecosystem: "docker"},
		{Directory: "/", Ecosystem: "github-actions"},
	}

	output, err := Generate(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := strings.TrimPrefix(output, "---\n")

	var cfg testConfig

	unmarshalErr := yaml.Unmarshal([]byte(body), &cfg)
	if unmarshalErr != nil {
		t.Fatalf("failed to parse YAML: %v", unmarshalErr)
	}

	if len(cfg.Updates) != 3 {
		t.Fatalf("expected 3 updates, got %d", len(cfg.Updates))
	}

	for _, u := range cfg.Updates {
		if u.Directory != "/" {
			t.Fatalf("expected all entries at /, got %s", u.Directory)
		}
	}

	// Verify alphabetical sort within same directory.
	if cfg.Updates[0].PackageEcosystem != "docker" {
		t.Fatalf("expected first ecosystem docker, got %s", cfg.Updates[0].PackageEcosystem)
	}

	if cfg.Updates[1].PackageEcosystem != "github-actions" {
		t.Fatalf("expected second ecosystem github-actions, got %s", cfg.Updates[1].PackageEcosystem)
	}

	if cfg.Updates[2].PackageEcosystem != "gomod" {
		t.Fatalf("expected third ecosystem gomod, got %s", cfg.Updates[2].PackageEcosystem)
	}
}

// TestGenerate_DocumentSeparator specifically tests the "---\n" prefix because
// yaml.Encoder doesn't emit it automatically. This is the most likely
// regression point if the Generate implementation is refactored.
func TestGenerate_DocumentSeparator(t *testing.T) {
	t.Parallel()

	results := []ScanResult{
		{Directory: "/", Ecosystem: "npm"},
	}

	output, err := Generate(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(output, "---\n") {
		t.Fatalf("expected output to start with exactly '---\\n', got prefix: %q", output[:10])
	}
}
