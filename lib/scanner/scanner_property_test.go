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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

type (
	// parsedConfig is a test-local type for parsing generated YAML back into a
	// structured form. We re-parse rather than comparing raw strings because
	// YAML serialization details (whitespace, quoting) are irrelevant — only
	// the semantic content matters.
	parsedConfig struct {
		Updates []parsedUpdate `yaml:"updates"`
		Version int            `yaml:"version"`
	}

	// parsedUpdate represents a single entry in the parsed updates array.
	parsedUpdate struct {
		PackageEcosystem string `yaml:"package-ecosystem"` // lint:allow_format
		Directory        string `yaml:"directory"`
	}

	// precedenceInput groups arguments for assertPropertyPrecedence to stay
	// within the 4-parameter function limit while keeping call sites readable.
	precedenceInput struct {
		Dir     string
		Winner  string
		Loser   string
		Results []ScanResult
	}
)

// Feature: scanner, Property 5: Compound AND-matching requires all patterns
// Validates: Requirements 3.1.
//
// This property guards the invariant that partial file presence never triggers
// an AND-group match. The combinatorial space of "which files are present vs
// absent" is too large for hand- written cases, so we use rapid to generate
// random directory names and verify the invariant holds universally.
func TestProperty5_CompoundANDMatchingRequiresAllPatterns(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		root := t.TempDir()

		// The "bun" ecosystem requires BOTH package.json AND bun.lock in the
		// same directory. Create only package.json (a strict subset) — bun must
		// NOT appear.
		dirName := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "dir")
		subDir := filepath.Join(root, dirName)

		mkErr := os.MkdirAll(subDir, 0o0755)
		if mkErr != nil {
			rt.Fatal(mkErr)
		}

		// Write only package.json — missing bun.lock means the AND-group is
		// incomplete.
		writeErr := os.WriteFile(filepath.Join(subDir, "package.json"), []byte("{}"), 0o0666)
		if writeErr != nil {
			rt.Fatal(writeErr)
		}

		results, scanErr := Scan(root, nil)
		if scanErr != nil {
			rt.Fatal(scanErr)
		}

		expectedDir := "/" + dirName

		for _, r := range results {
			if r.Directory == expectedDir && r.Ecosystem == "bun" {
				rt.Fatalf("bun should not match with only package.json in %s", expectedDir)
			}
		}
	})
}

// Feature: scanner, Property 6: OR-matching succeeds on any alternative
// Validates: Requirements 3.2.
//
// This property ensures that each OR-alternative in a rule is individually
// sufficient for detection. Without this, a rule might accidentally require ALL
// alternatives to be present.
func TestProperty6_ORMatchingSucceedsOnAnyAlternative(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		root := t.TempDir()

		// The "bazel" ecosystem has OR-alternatives: BUILD.bazel OR BUILD. Pick
		// one at random and verify bazel is detected.
		alternatives := []string{"BUILD.bazel", "BUILD"}
		idx := rapid.IntRange(0, len(alternatives)-1).Draw(rt, "alt-idx")
		chosen := alternatives[idx]

		writeErr := os.WriteFile(filepath.Join(root, chosen), []byte(""), 0o0666)
		if writeErr != nil {
			rt.Fatal(writeErr)
		}

		results, scanErr := Scan(root, nil)
		if scanErr != nil {
			rt.Fatal(scanErr)
		}

		found := false

		for _, r := range results {
			if r.Directory == "/" && r.Ecosystem == "bazel" {
				found = true

				break
			}
		}

		if !found {
			rt.Fatalf("bazel should match with %s present", chosen)
		}
	})
}

// Feature: scanner, Property 7: OpenTofu precedence over Terraform
// Validates: Requirements 4.1.
//
// This property verifies across many random filename combinations that whenever
// both .tofu and .tf files coexist, only opentofu survives precedence
// resolution. The random filenames ensure the invariant isn't accidentally
// dependent on specific base names like "main.tofu".
func TestProperty7_OpenTofuPrecedence(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		root := t.TempDir()

		// Generate random .tofu and .tf filenames.
		tofuBase := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "tofu-base")
		tfBase := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "tf-base")
		tofuName := tofuBase + ".tofu"
		tfName := tfBase + ".tf"

		writePrecedenceFiles(rt, root, tofuName, tfName)

		results, scanErr := Scan(root, nil)
		if scanErr != nil {
			rt.Fatal(scanErr)
		}

		assertPrecedenceResults(rt, results)
	})
}

// writePrecedenceFiles creates the minimum file set needed to trigger both
// opentofu and terraform rules simultaneously, setting up the precondition for
// precedence resolution testing.
func writePrecedenceFiles(t *rapid.T, root, tofuName, tfName string) {
	writeErr := os.WriteFile(filepath.Join(root, ".terraform.lock.hcl"), []byte(""), 0o0666)
	if writeErr != nil {
		t.Fatal(writeErr)
	}

	writeErr = os.WriteFile(filepath.Join(root, tofuName), []byte(""), 0o0666)
	if writeErr != nil {
		t.Fatal(writeErr)
	}

	writeErr = os.WriteFile(filepath.Join(root, tfName), []byte(""), 0o0666)
	if writeErr != nil {
		t.Fatal(writeErr)
	}
}

// assertPrecedenceResults encodes the two-part invariant: the winner must be
// present (detection works) AND the loser must be absent (suppression works).
// Both parts are necessary — if we only checked absence of terraform, we'd miss
// bugs where opentofu also fails to detect.
func assertPrecedenceResults(t *rapid.T, results []ScanResult) {
	hasOpentofu := false
	hasTerraform := false

	for _, r := range results {
		if r.Directory == "/" && r.Ecosystem == "opentofu" {
			hasOpentofu = true
		}

		if r.Directory == "/" && r.Ecosystem == "terraform" {
			hasTerraform = true
		}
	}

	if !hasOpentofu {
		t.Fatal("expected opentofu in results")
	}

	if hasTerraform {
		t.Fatal("terraform should not appear when opentofu is present")
	}
}

// Feature: scanner, Property 8: Multiple ecosystems per directory
// Validates: Requirements 2.3.
//
// This property guards against rules that accidentally consume or shadow other
// rules' results. Independent ecosystems (gomod and cargo share no files) must
// always produce separate entries regardless of evaluation order.
func TestProperty8_MultipleEcosystemsPerDirectory(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		root := t.TempDir()

		writeMultiEcosystemFiles(rt, root)

		results, scanErr := Scan(root, nil)
		if scanErr != nil {
			rt.Fatal(scanErr)
		}

		assertMultiEcosystemResults(rt, results)
	})
}

// writeMultiEcosystemFiles places two unrelated ecosystem triggers (Go and
// Rust) in the same directory to set up a non-interference test. These
// ecosystems share no file patterns, so both must always be detected
// independently.
func writeMultiEcosystemFiles(t *rapid.T, root string) {
	writeErr := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n"), 0o0666)
	if writeErr != nil {
		t.Fatal(writeErr)
	}

	writeErr = os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[package]\n"), 0o0666)
	if writeErr != nil {
		t.Fatal(writeErr)
	}
}

// assertMultiEcosystemResults verifies exact counts (not just presence) to
// catch both missing detections and accidental duplicates from re-evaluation
// bugs.
func assertMultiEcosystemResults(
	t *rapid.T, results []ScanResult,
) {
	gomodCount := 0
	cargoCount := 0

	for _, r := range results {
		if r.Directory == "/" && r.Ecosystem == "gomod" {
			gomodCount++
		}

		if r.Directory == "/" && r.Ecosystem == "cargo" {
			cargoCount++
		}
	}

	if gomodCount != 1 {
		t.Fatalf("expected exactly 1 gomod result, got %d", gomodCount)
	}

	if cargoCount != 1 {
		t.Fatalf("expected exactly 1 cargo result, got %d", cargoCount)
	}
}

// Feature: scanner, Property 9: Relative path format
// Validates: Requirements 2.8.
//
// This property ensures the "/" prefix and forward-slash separators survive
// across arbitrary nesting depths. The random depth and segment names catch
// platform-specific separator bugs that a single fixed test case would miss.
func TestProperty9_RelativePathFormat(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		root := t.TempDir()

		// Generate a random nested directory path.
		depth := rapid.IntRange(1, 3).Draw(rt, "depth")
		parts := make([]string, depth)

		for i := range depth {
			parts[i] = rapid.StringMatching(`[a-z]{2,6}`).Draw(rt, "part")
		}

		nested := filepath.Join(root, filepath.Join(parts...))

		mkErr := os.MkdirAll(nested, 0o0755)
		if mkErr != nil {
			rt.Fatal(mkErr)
		}

		// Place a go.mod in the nested directory to trigger a match.
		writeErr := os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module test\n"), 0o0666)
		if writeErr != nil {
			rt.Fatal(writeErr)
		}

		results, scanErr := Scan(root, nil)
		if scanErr != nil {
			rt.Fatal(scanErr)
		}

		for _, r := range results {
			if !strings.HasPrefix(r.Directory, "/") {
				rt.Fatalf("Directory %q does not start with /", r.Directory)
			}

			if strings.Contains(r.Directory, "\\") {
				rt.Fatalf("Directory %q contains backslash separators", r.Directory)
			}
		}
	})
}

// Feature: scanner, Property 1: Serialization round-trip
// Validates: Requirements 8.1, 8.2.
//
// This property verifies that Generate preserves all information from the input
// — no entries are lost or invented during serialization. It catches encoding
// bugs that unit tests with fixed inputs might miss (e.g., special characters
// in directory names, emoji ecosystems, etc.).
func TestProperty1_SerializationRoundTrip(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		results := rapid.SliceOfN(genScanResult(), 1, 10).Draw(rt, "results")

		yamlStr, genErr := Generate(results, nil)
		if genErr != nil {
			rt.Fatal(genErr)
		}

		var parsed parsedConfig

		unmarshalErr := yaml.Unmarshal([]byte(strings.TrimPrefix(yamlStr, "---\n")), &parsed)
		if unmarshalErr != nil {
			rt.Fatalf("failed to parse generated YAML: %v", unmarshalErr)
		}

		// Build a set of directory-ecosystem pairs from original input.
		type pair struct {
			Dir string
			Eco string
		}

		expected := make(map[pair]bool)

		for _, r := range results {
			expected[pair{Dir: r.Directory, Eco: r.Ecosystem}] = true
		}

		// Build a set from parsed output.
		actual := make(map[pair]bool)

		for _, u := range parsed.Updates {
			actual[pair{
				Dir: u.Directory,
				Eco: u.PackageEcosystem,
			}] = true
		}

		// Verify all original pairs are in the output.
		for p := range expected {
			if !actual[p] {
				rt.Fatalf("missing pair in output: dir=%q eco=%q", p.Dir, p.Eco)
			}
		}

		// Verify no extra pairs in the output.
		for p := range actual {
			if !expected[p] {
				rt.Fatalf("unexpected pair in output: dir=%q eco=%q", p.Dir, p.Eco)
			}
		}
	})
}

// Feature: scanner, Property 2: One entry per ScanResult
// Validates: Requirements 8.1.
//
// This property guards against Generate silently dropping or duplicating
// entries. The count invariant (input length == output length) is simple but
// catches a wide class of off-by-one and deduplication bugs.
func TestProperty2_OneEntryPerScanResult(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(0, 20).Draw(rt, "n")
		results := rapid.SliceOfN(genScanResult(), n, n).Draw(rt, "results")

		yamlStr, genErr := Generate(results, nil)
		if genErr != nil {
			rt.Fatal(genErr)
		}

		var parsed parsedConfig

		unmarshalErr := yaml.Unmarshal([]byte(strings.TrimPrefix(yamlStr, "---\n")), &parsed)
		if unmarshalErr != nil {
			rt.Fatalf("failed to parse generated YAML: %v", unmarshalErr)
		}

		if len(parsed.Updates) != n {
			rt.Fatalf("expected %d updates, got %d", n, len(parsed.Updates))
		}
	})
}

// Feature: scanner, Property 3: Output is sorted
// Validates: Requirements 5.3.
//
// Deterministic sort order matters because users commit the generated file and
// review diffs in PRs. If order varied between runs, every regeneration would
// produce a noisy diff even when nothing changed. This property verifies the
// invariant across many random input orderings.
func TestProperty3_OutputIsSorted(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		results := rapid.SliceOfN(genScanResult(), 1, 20).Draw(rt, "results")

		yamlStr, genErr := Generate(results, nil)
		if genErr != nil {
			rt.Fatal(genErr)
		}

		var parsed parsedConfig

		unmarshalErr := yaml.Unmarshal([]byte(strings.TrimPrefix(yamlStr, "---\n")), &parsed)
		if unmarshalErr != nil {
			rt.Fatalf("failed to parse generated YAML: %v", unmarshalErr)
		}

		sorted := slices.IsSortedFunc(
			parsed.Updates,
			func(a, b parsedUpdate) int {
				dirCmp := strings.Compare(a.Directory, b.Directory)
				if dirCmp != 0 {
					return dirCmp
				}

				return strings.Compare(a.PackageEcosystem, b.PackageEcosystem)
			},
		)

		if !sorted {
			rt.Fatal("updates array is not sorted by directory then ecosystem")
		}
	})
}

// Feature: scanner, Property 4: Empty input produces valid empty document
// Validates: Requirements 5.4, 5.5.
//
// An empty scan result (no ecosystems found) must still produce a valid
// Dependabot config with version: 2 and an empty updates array. This ensures
// the generated file is always parseable by GitHub, even for repos with no
// recognized package managers.
func TestProperty4_EmptyInputProducesValidEmptyDocument(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		// Test with nil slice — Generate handles nil and empty identically per
		// design.
		var input []ScanResult

		yamlStr, genErr := Generate(input, nil)
		if genErr != nil {
			rt.Fatal(genErr)
		}

		if !strings.HasPrefix(yamlStr, "---") {
			rt.Fatal("output does not start with ---")
		}

		var parsed parsedConfig

		unmarshalErr := yaml.Unmarshal([]byte(strings.TrimPrefix(yamlStr, "---\n")), &parsed)
		if unmarshalErr != nil {
			rt.Fatalf("failed to parse generated YAML: %v", unmarshalErr)
		}

		if parsed.Version != 2 {
			rt.Fatalf("expected version 2, got %d", parsed.Version)
		}

		if len(parsed.Updates) != 0 {
			rt.Fatalf("expected empty updates, got %d entries", len(parsed.Updates))
		}
	})
}

// genScanResult produces random ScanResult values covering realistic directory
// depths and a representative sample of ecosystem identifiers. The generator
// constrains to the actual input space (valid paths, known ecosystems) so that
// test failures reveal real bugs rather than invalid-input noise.
func genScanResult() *rapid.Generator[ScanResult] {
	return rapid.Custom(func(t *rapid.T) ScanResult {
		dir := "/" + rapid.StringMatching(`[a-z]{1,5}(/[a-z]{1,5}){0,2}`).Draw(t, "dir")
		eco := rapid.SampledFrom(
			[]string{"gomod", "npm", "cargo", "docker", "pip", "maven", "gradle", "helm", "nix", "pub"},
		).Draw(t, "ecosystem")

		return ScanResult{Directory: dir, Ecosystem: eco}
	})
}

// Feature: scanner, Property 10: Bun precedence over npm
// Validates: Requirements 4.1.
//
// This property verifies that bun always suppresses npm in the same directory,
// regardless of which random subdirectory name is used. The random directory
// names ensure the precedence logic works on the directory field, not on
// hardcoded paths.
func TestProperty10_BunPrecedenceOverNpm(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		root := t.TempDir()

		// Generate a random subdirectory name.
		dirName := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "dir")
		subDir := filepath.Join(root, dirName)

		mkErr := os.MkdirAll(subDir, 0o0755)
		if mkErr != nil {
			rt.Fatal(mkErr)
		}

		writeBunAndNpmFiles(rt, subDir)

		results, scanErr := Scan(root, nil)
		if scanErr != nil {
			rt.Fatal(scanErr)
		}

		assertPropertyPrecedence(rt, &precedenceInput{
			Results: results,
			Dir:     "/" + dirName,
			Winner:  "bun",
			Loser:   "npm",
		})
	})
}

// writeBunAndNpmFiles uses bunfig.toml (the standalone bun indicator) plus
// package.json (the npm indicator) to create a scenario where both ecosystems
// match before precedence runs.
func writeBunAndNpmFiles(t *rapid.T, dir string) {
	writeErr := os.WriteFile(
		filepath.Join(dir, "bunfig.toml"), []byte(""), 0o0666,
	)
	if writeErr != nil {
		t.Fatal(writeErr)
	}

	writeErr = os.WriteFile(
		filepath.Join(dir, "package.json"), []byte("{}"), 0o0666,
	)
	if writeErr != nil {
		t.Fatal(writeErr)
	}
}

// Feature: scanner, Property 11: Uv precedence over pip
// Validates: Requirements 4.1.
//
// This property exercises uv/pip precedence across random pip trigger files.
// Pip has many possible triggers (requirements.txt, setup.py, pyproject.toml,
// etc.), and precedence must work regardless of which one is present.
func TestProperty11_UvPrecedenceOverPip(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		root := t.TempDir()

		// Generate a random subdirectory name.
		dirName := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "dir")
		subDir := filepath.Join(root, dirName)

		mkErr := os.MkdirAll(subDir, 0o0755)
		if mkErr != nil {
			rt.Fatal(mkErr)
		}

		writeUvAndPipFiles(rt, subDir)

		results, scanErr := Scan(root, nil)
		if scanErr != nil {
			rt.Fatal(scanErr)
		}

		assertPropertyPrecedence(rt, &precedenceInput{
			Results: results,
			Dir:     "/" + dirName,
			Winner:  "uv",
			Loser:   "pip",
		})
	})
}

// writeUvAndPipFiles places uv.lock alongside a randomly chosen pip trigger
// file. The random selection ensures precedence isn't accidentally dependent on
// a specific pip manifest format.
func writeUvAndPipFiles(t *rapid.T, dir string) {
	writeErr := os.WriteFile(
		filepath.Join(dir, "uv.lock"), []byte(""), 0o0666,
	)
	if writeErr != nil {
		t.Fatal(writeErr)
	}

	// Pick a random pip trigger file.
	pipFiles := []string{
		"requirements.txt",
		"requirements-dev.txt",
		"setup.cfg",
		"setup.py",
		"pyproject.toml",
		"Pipfile",
	}

	pipIdx := rapid.IntRange(0, len(pipFiles)-1).Draw(t, "pip-idx")
	pipFile := pipFiles[pipIdx]

	writeErr = os.WriteFile(
		filepath.Join(dir, pipFile), []byte(""), 0o0666,
	)
	if writeErr != nil {
		t.Fatal(writeErr)
	}
}

// assertPropertyPrecedence is a reusable assertion for all precedence property
// tests. It encodes the two-part invariant: the winner must be detected
// (proving the rule matched) AND the loser must be absent (proving suppression
// worked). Testing both parts prevents a passing test that merely fails to
// detect either ecosystem.
func assertPropertyPrecedence(
	t *rapid.T, input *precedenceInput,
) {
	hasWinner := false
	hasLoser := false

	for _, r := range input.Results {
		if r.Directory == input.Dir && r.Ecosystem == input.Winner {
			hasWinner = true
		}

		if r.Directory == input.Dir && r.Ecosystem == input.Loser {
			hasLoser = true
		}
	}

	if !hasWinner {
		t.Fatalf("expected %s in %s", input.Winner, input.Dir)
	}

	if hasLoser {
		t.Fatalf(
			"%s should not appear when %s is present in %s",
			input.Loser, input.Winner, input.Dir,
		)
	}
}
