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

// Unit tests for the scanner package exercise the Scan function across multiple
// categories of concern:
//
// 1. Error handling — verifying that each sentinel error surfaces for
//    the right failure mode, so the CLI can present targeted guidance.
// 2. Ecosystem fixtures — integration-level tests that scan real fixture
//    directories to confirm glob patterns actually match filesystem
//    structures end-to-end (not just in isolation).
// 3. Precedence — protecting against regressions where a generic
//    ecosystem appears alongside its more specific replacement.
// 4. Path formatting — ensuring the Dependabot-expected "/" prefix and
//    forward-slash separators survive across platforms.
// 5. Compound matching — verifying AND-group semantics work correctly
//    with real files.
// 6. Multi-ecosystem — confirming independent rules don't interfere
//    with each other in the same directory.

package scanner // lint:allow_naming_conflict_stdlib

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestScan_RootPathNotExist verifies the CLI can distinguish "path doesn't
// exist" from other failures, enabling a message like "directory not found"
// rather than a generic "scan failed.".
func TestScan_RootPathNotExist(t *testing.T) {
	t.Parallel()

	_, err := Scan("/no/such/path/exists")
	if err == nil {
		t.Fatal("expected error for non-existent path, got nil")
	}

	if !errors.Is(err, ErrRootNotExist) {
		t.Fatalf("expected ErrRootNotExist, got: %v", err)
	}
}

// TestScan_RootPathIsFile verifies the CLI can tell the user they accidentally
// pointed at a file instead of a directory, rather than producing a confusing
// "permission denied" or empty result.
func TestScan_RootPathIsFile(t *testing.T) {
	t.Parallel()

	tmpFile, createErr := os.CreateTemp(t.TempDir(), "scantest")
	if createErr != nil {
		t.Fatalf("failed to create temp file: %v", createErr)
	}

	closeErr := tmpFile.Close()
	if closeErr != nil {
		t.Fatalf("failed to close temp file: %v", closeErr)
	}

	_, err := Scan(tmpFile.Name())
	if err == nil {
		t.Fatal("expected error for file path, got nil")
	}

	if !errors.Is(err, ErrRootNotDir) {
		t.Fatalf("expected ErrRootNotDir, got: %v", err)
	}
}

// TestScan_RootPathValidDirectory confirms the baseline behavior: an empty
// directory produces zero results without error, so we know the scanner doesn't
// hallucinate ecosystems or crash on the simplest valid input.
func TestScan_RootPathValidDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	results, err := Scan(dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected empty results, got: %v", results)
	}
}

// TestScan_RootPathUnreadable protects against the case where a directory
// exists (Stat succeeds) but can't be listed due to permissions. Without this
// check, the scanner would panic or produce a misleading error during the walk.
func TestScan_RootPathUnreadable(t *testing.T) {
	t.Parallel()

	if os.Getuid() == 0 {
		t.Skip("skipping unreadable test when running as root")
	}

	dir := t.TempDir()
	unreadable := filepath.Join(dir, "noperm")

	mkdirErr := os.Mkdir(unreadable, 0o0000)
	if mkdirErr != nil {
		t.Fatalf("failed to create dir: %v", mkdirErr)
	}

	t.Cleanup(func() {
		chmodErr := os.Chmod(unreadable, 0o0755) // lint:allow_755
		if chmodErr != nil {
			t.Logf("cleanup chmod failed: %v", chmodErr)
		}
	})

	_, err := Scan(unreadable)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrRootNotReadable) {
		t.Fatalf("expected ErrRootNotReadable, got: %v", err)
	}
}

// TestScan_EcosystemFixtures runs the scanner against real fixture directories
// under src/<ecosystem>/<variant>. These are integration-level tests that give
// confidence the glob patterns in rules.go actually match filesystem structures
// end-to-end, catching issues that isolated pattern tests would miss (e.g.,
// fileglob behavior quirks, case sensitivity).
func TestScan_EcosystemFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fixture   string
		ecosystem string
	}{
		{name: "bazel/a", fixture: "../../src/bazel/a", ecosystem: "bazel"},
		{name: "bazel/b", fixture: "../../src/bazel/b", ecosystem: "bazel"},
		{name: "bun/a", fixture: "../../src/bun/a", ecosystem: "bun"},
		{name: "bun/b", fixture: "../../src/bun/b", ecosystem: "bun"},
		{name: "bundler/a", fixture: "../../src/bundler/a", ecosystem: "bundler"},
		{name: "bundler/b", fixture: "../../src/bundler/b", ecosystem: "bundler"},
		{name: "bundler/c", fixture: "../../src/bundler/c", ecosystem: "bundler"},
		{name: "cargo/a", fixture: "../../src/cargo/a", ecosystem: "cargo"},
		{name: "composer/a", fixture: "../../src/composer/a", ecosystem: "composer"},
		{name: "conda/a", fixture: "../../src/conda/a", ecosystem: "conda"},
		{name: "conda/b", fixture: "../../src/conda/b", ecosystem: "conda"},
		{name: "deno/a", fixture: "../../src/deno/a", ecosystem: "deno"},
		{name: "deno/b", fixture: "../../src/deno/b", ecosystem: "deno"},
		{name: "devcontainers/a", fixture: "../../src/devcontainers/a", ecosystem: "devcontainers"},
		{name: "devcontainers/b", fixture: "../../src/devcontainers/b", ecosystem: "devcontainers"},
		{name: "devcontainers/c", fixture: "../../src/devcontainers/c", ecosystem: "devcontainers"},
		{name: "devcontainers/d", fixture: "../../src/devcontainers/d", ecosystem: "devcontainers"},
		{name: "docker/a", fixture: "../../src/docker/a", ecosystem: "docker"},
		{name: "docker/b", fixture: "../../src/docker/b", ecosystem: "docker"},
		{name: "docker/c", fixture: "../../src/docker/c", ecosystem: "docker"},
		{name: "docker/d", fixture: "../../src/docker/d", ecosystem: "docker"},
		{name: "docker-compose/a", fixture: "../../src/docker-compose/a", ecosystem: "docker-compose"},
		{name: "docker-compose/b", fixture: "../../src/docker-compose/b", ecosystem: "docker-compose"},
		{name: "docker-compose/c", fixture: "../../src/docker-compose/c", ecosystem: "docker-compose"},
		{name: "docker-compose/d", fixture: "../../src/docker-compose/d", ecosystem: "docker-compose"},
		{name: "docker-compose/e", fixture: "../../src/docker-compose/e", ecosystem: "docker-compose"},
		{name: "docker-compose/f", fixture: "../../src/docker-compose/f", ecosystem: "docker-compose"},
		{name: "dotnet-sdk/a", fixture: "../../src/dotnet-sdk/a", ecosystem: "dotnet-sdk"},
		{name: "elm/a", fixture: "../../src/elm/a", ecosystem: "elm"},
		{name: "github-actions/a", fixture: "../../src/github-actions/a", ecosystem: "github-actions"},
		{name: "github-actions/b", fixture: "../../src/github-actions/b", ecosystem: "github-actions"},
		{name: "github-actions/c", fixture: "../../src/github-actions/c", ecosystem: "github-actions"},
		{name: "github-actions/d", fixture: "../../src/github-actions/d", ecosystem: "github-actions"},
		{name: "gitsubmodule/a", fixture: "../../src/gitsubmodule/a", ecosystem: "gitsubmodule"},
		{name: "gomod/a", fixture: "../../src/gomod/a", ecosystem: "gomod"},
		{name: "gomod/b", fixture: "../../src/gomod/b", ecosystem: "gomod"},
		{name: "gradle/a", fixture: "../../src/gradle/a", ecosystem: "gradle"},
		{name: "gradle/b", fixture: "../../src/gradle/b", ecosystem: "gradle"},
		{name: "helm/a", fixture: "../../src/helm/a", ecosystem: "helm"},
		{name: "julia/a", fixture: "../../src/julia/a", ecosystem: "julia"},
		{name: "julia/b", fixture: "../../src/julia/b", ecosystem: "julia"},
		{name: "julia/c", fixture: "../../src/julia/c", ecosystem: "julia"},
		{name: "julia/d", fixture: "../../src/julia/d", ecosystem: "julia"},
		{name: "julia/e", fixture: "../../src/julia/e", ecosystem: "julia"},
		{name: "julia/f", fixture: "../../src/julia/f", ecosystem: "julia"},
		{name: "maven/a", fixture: "../../src/maven/a", ecosystem: "maven"},
		{name: "mix/a", fixture: "../../src/mix/a", ecosystem: "mix"},
		{name: "nix/a", fixture: "../../src/nix/a", ecosystem: "nix"},
		{name: "nix/b", fixture: "../../src/nix/b", ecosystem: "nix"},
		{name: "npm/a", fixture: "../../src/npm/a", ecosystem: "npm"},
		{name: "npm/b", fixture: "../../src/npm/b", ecosystem: "npm"},
		{name: "npm/c", fixture: "../../src/npm/c", ecosystem: "npm"},
		{name: "npm/d", fixture: "../../src/npm/d", ecosystem: "npm"},
		{name: "npm/e", fixture: "../../src/npm/e", ecosystem: "npm"},
		{name: "npm/f", fixture: "../../src/npm/f", ecosystem: "npm"},
		{name: "npm/g", fixture: "../../src/npm/g", ecosystem: "npm"},
		{name: "npm/h", fixture: "../../src/npm/h", ecosystem: "npm"},
		{name: "nuget/a", fixture: "../../src/nuget/a", ecosystem: "nuget"},
		{name: "opentofu/a", fixture: "../../src/opentofu/a", ecosystem: "opentofu"},
		{name: "pip/a", fixture: "../../src/pip/a", ecosystem: "pip"},
		{name: "pip/b", fixture: "../../src/pip/b", ecosystem: "pip"},
		{name: "pip/c", fixture: "../../src/pip/c", ecosystem: "pip"},
		{name: "pip/d", fixture: "../../src/pip/d", ecosystem: "pip"},
		{name: "pip/e", fixture: "../../src/pip/e", ecosystem: "pip"},
		{name: "pip/f", fixture: "../../src/pip/f", ecosystem: "pip"},
		{name: "pip/g", fixture: "../../src/pip/g", ecosystem: "pip"},
		{name: "pip/h", fixture: "../../src/pip/h", ecosystem: "pip"},
		{name: "pip/i", fixture: "../../src/pip/i", ecosystem: "pip"},
		{name: "pip/j", fixture: "../../src/pip/j", ecosystem: "pip"},
		{name: "pip/k", fixture: "../../src/pip/k", ecosystem: "pip"},
		{name: "pip/l", fixture: "../../src/pip/l", ecosystem: "pip"},
		{name: "pip/m", fixture: "../../src/pip/m", ecosystem: "pip"},
		{name: "pip/n", fixture: "../../src/pip/n", ecosystem: "pip"},
		{name: "pip/o", fixture: "../../src/pip/o", ecosystem: "pip"},
		{name: "pip/p", fixture: "../../src/pip/p", ecosystem: "pip"},
		{name: "pre-commit/a", fixture: "../../src/pre-commit/a", ecosystem: "pre-commit"},
		{name: "pre-commit/b", fixture: "../../src/pre-commit/b", ecosystem: "pre-commit"},
		{name: "pre-commit/c", fixture: "../../src/pre-commit/c", ecosystem: "pre-commit"},
		{name: "pre-commit/d", fixture: "../../src/pre-commit/d", ecosystem: "pre-commit"},
		{name: "pub/a", fixture: "../../src/pub/a", ecosystem: "pub"},
		{name: "pub/b", fixture: "../../src/pub/b", ecosystem: "pub"},
		{name: "rust-toolchain/a", fixture: "../../src/rust-toolchain/a", ecosystem: "rust-toolchain"},
		{name: "rust-toolchain/b", fixture: "../../src/rust-toolchain/b", ecosystem: "rust-toolchain"},
		{name: "sbt/a", fixture: "../../src/sbt/a", ecosystem: "sbt"},
		{name: "swift/a", fixture: "../../src/swift/a", ecosystem: "swift"},
		{name: "swift/b", fixture: "../../src/swift/b", ecosystem: "swift"},
		{name: "swift/c", fixture: "../../src/swift/c", ecosystem: "swift"},
		{name: "swift/d", fixture: "../../src/swift/d", ecosystem: "swift"},
		{name: "swift/e", fixture: "../../src/swift/e", ecosystem: "swift"},
		{name: "terraform/a", fixture: "../../src/terraform/a", ecosystem: "terraform"},
		{name: "uv/a", fixture: "../../src/uv/a", ecosystem: "uv"},
		{name: "uv/b", fixture: "../../src/uv/b", ecosystem: "uv"},
		{name: "uv/c", fixture: "../../src/uv/c", ecosystem: "uv"},
		{name: "uv/d", fixture: "../../src/uv/d", ecosystem: "uv"},
		{name: "uv/e", fixture: "../../src/uv/e", ecosystem: "uv"},
		{name: "uv/f", fixture: "../../src/uv/f", ecosystem: "uv"},
		{name: "uv/g", fixture: "../../src/uv/g", ecosystem: "uv"},
		{name: "vcpkg/a", fixture: "../../src/vcpkg/a", ecosystem: "vcpkg"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assertFixtureEcosystem(t, tc.fixture, tc.ecosystem)
		})
	}
}

// assertFixtureEcosystem resolves a fixture path, skips if missing (CI
// environments may not have the full fixture tree checked out), scans, and
// asserts the expected ecosystem at root. Skipping rather than failing avoids
// false negatives in partial-checkout environments.
func assertFixtureEcosystem(t *testing.T, fixture, ecosystem string) {
	t.Helper()

	fixtureDir, absErr := filepath.Abs(fixture)
	if absErr != nil {
		t.Fatalf("failed to resolve path: %v", absErr)
	}

	// Skip if fixture directory does not exist.
	_, statErr := os.Stat(fixtureDir)
	if statErr != nil {
		t.Skipf("fixture directory not available: %s", fixtureDir)
	}

	results, scanErr := Scan(fixtureDir)
	if scanErr != nil {
		t.Fatalf("unexpected error: %v", scanErr)
	}

	found := false

	for _, r := range results {
		if r.Ecosystem == ecosystem && r.Directory == "/" {
			found = true

			break
		}
	}

	if !found {
		t.Fatalf("expected %s at /, got: %+v", ecosystem, results)
	}
}

// TestScan_OpenTofuPrecedenceOverTerraform protects against the real-world
// scenario where a repository migrates from Terraform to OpenTofu — both file
// types coexist during migration, but only the opentofu ecosystem should appear
// in the generated config to avoid duplicate dependency update PRs.
func TestScan_OpenTofuPrecedenceOverTerraform(t *testing.T) {
	t.Parallel()

	// Create a dir with both .tofu and .tf files.
	root := t.TempDir()

	writeErr := os.WriteFile(filepath.Join(root, ".terraform.lock.hcl"), []byte(""), 0o0666)
	if writeErr != nil {
		t.Fatalf("failed to write file: %v", writeErr)
	}

	writeErr = os.WriteFile(filepath.Join(root, "main.tofu"), []byte(""), 0o0666)
	if writeErr != nil {
		t.Fatalf("failed to write file: %v", writeErr)
	}

	writeErr = os.WriteFile(filepath.Join(root, "main.tf"), []byte(""), 0o0666)
	if writeErr != nil {
		t.Fatalf("failed to write file: %v", writeErr)
	}

	results, scanErr := Scan(root)
	if scanErr != nil {
		t.Fatalf("unexpected error: %v", scanErr)
	}

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
		t.Fatal("terraform should not appear when opentofu present")
	}
}

// TestScan_TerraformWithoutOpenTofu confirms that precedence suppression is
// conditional — terraform still appears when no .tofu files exist, preventing
// false suppression in pure Terraform projects.
func TestScan_TerraformWithoutOpenTofu(t *testing.T) {
	t.Parallel()

	fixtureDir, absErr := filepath.Abs("../../src/terraform/a")
	if absErr != nil {
		t.Fatalf("failed to resolve path: %v", absErr)
	}

	// Skip if fixture directory does not exist.
	_, statErr := os.Stat(fixtureDir)
	if statErr != nil {
		t.Skipf("fixture directory not available: %s", fixtureDir)
	}

	results, scanErr := Scan(fixtureDir)
	if scanErr != nil {
		t.Fatalf("unexpected error: %v", scanErr)
	}

	found := false

	for _, r := range results {
		if r.Ecosystem == "terraform" && r.Directory == "/" {
			found = true

			break
		}
	}

	if !found {
		t.Fatalf("expected terraform at /, got: %+v", results)
	}
}

// TestScan_OpenTofuWithoutTerraform confirms that a pure OpenTofu project (no
// .tf files) correctly produces only the opentofu ecosystem — the absence of
// terraform shouldn't be a side effect of precedence, but of the terraform rule
// simply not matching.
func TestScan_OpenTofuWithoutTerraform(t *testing.T) {
	t.Parallel()

	fixtureDir, absErr := filepath.Abs("../../src/opentofu/a")
	if absErr != nil {
		t.Fatalf("failed to resolve path: %v", absErr)
	}

	// Skip if fixture directory does not exist.
	_, statErr := os.Stat(fixtureDir)
	if statErr != nil {
		t.Skipf("fixture directory not available: %s", fixtureDir)
	}

	results, scanErr := Scan(fixtureDir)
	if scanErr != nil {
		t.Fatalf("unexpected error: %v", scanErr)
	}

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
		t.Fatal("terraform should not appear for opentofu fixture")
	}
}

// TestScan_RelativePathFormat ensures the output uses Dependabot's expected
// path format (forward-slash-prefixed, relative to repo root) regardless of the
// OS path separator or how deeply nested the match is.
func TestScan_RelativePathFormat(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "sub", "dir")

	mkErr := os.MkdirAll(nested, 0o0755)
	if mkErr != nil {
		t.Fatalf("failed to create nested dirs: %v", mkErr)
	}

	writeErr := os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module test\n"), 0o0666)
	if writeErr != nil {
		t.Fatalf("failed to write go.mod: %v", writeErr)
	}

	results, scanErr := Scan(root)
	if scanErr != nil {
		t.Fatalf("unexpected error: %v", scanErr)
	}

	found := false

	for _, r := range results {
		if r.Ecosystem == "gomod" && r.Directory == "/sub/dir" {
			found = true

			break
		}
	}

	if !found {
		t.Fatalf("expected gomod at /sub/dir, got: %+v", results)
	}
}

// TestScan_BunCompoundMatch exercises the AND-group detection path for bun
// (package.json + bun.lock). This is distinct from the bunfig.toml OR-path and
// must be tested separately because AND-group evaluation has different failure
// modes.
func TestScan_BunCompoundMatch(t *testing.T) {
	t.Parallel()

	// bun/b has both package.json and bun.lock (AND-group).
	fixtureDir, absErr := filepath.Abs("../../src/bun/b")
	if absErr != nil {
		t.Fatalf("failed to resolve path: %v", absErr)
	}

	// Skip if fixture directory does not exist.
	_, statErr := os.Stat(fixtureDir)
	if statErr != nil {
		t.Skipf("fixture directory not available: %s", fixtureDir)
	}

	results, scanErr := Scan(fixtureDir)
	if scanErr != nil {
		t.Fatalf("unexpected error: %v", scanErr)
	}

	found := false

	for _, r := range results {
		if r.Ecosystem == "bun" && r.Directory == "/" {
			found = true

			break
		}
	}

	if !found {
		t.Fatalf("expected bun at /, got: %+v", results)
	}
}

// TestScan_BunNegativeNoMatch is a negative test confirming that partial
// matches don't trigger detection. bun.lockb is a different file from bun.lock
// (binary vs text lockfile), and having it alone should NOT satisfy any bun
// AND-group.
func TestScan_BunNegativeNoMatch(t *testing.T) {
	t.Parallel()

	fixtureDir, absErr := filepath.Abs("../../src/bun/c")
	if absErr != nil {
		t.Fatalf("failed to resolve path: %v", absErr)
	}

	_, statErr := os.Stat(fixtureDir)
	if statErr != nil {
		t.Skipf("fixture directory not available: %s", fixtureDir)
	}

	results, scanErr := Scan(fixtureDir)
	if scanErr != nil {
		t.Fatalf("unexpected error: %v", scanErr)
	}

	for _, r := range results {
		if r.Ecosystem == "bun" && r.Directory == "/" {
			t.Fatal("bun should not match bun.lockb alone")
		}
	}
}

// TestScan_OpenTofuBFixtureTerraformOnly covers the edge case where a repo has
// the lock file + .tf files but no .tofu files. Despite living under
// src/opentofu/, this fixture should produce a terraform match — proving that
// the directory name is irrelevant and only file contents drive detection.
func TestScan_OpenTofuBFixtureTerraformOnly(t *testing.T) {
	t.Parallel()

	fixtureDir, absErr := filepath.Abs("../../src/opentofu/b")
	if absErr != nil {
		t.Fatalf("failed to resolve path: %v", absErr)
	}

	_, statErr := os.Stat(fixtureDir)
	if statErr != nil {
		t.Skipf("fixture directory not available: %s", fixtureDir)
	}

	results, scanErr := Scan(fixtureDir)
	if scanErr != nil {
		t.Fatalf("unexpected error: %v", scanErr)
	}

	hasTerraform := false
	hasOpentofu := false

	for _, r := range results {
		if r.Directory == "/" && r.Ecosystem == "terraform" {
			hasTerraform = true
		}

		if r.Directory == "/" && r.Ecosystem == "opentofu" {
			hasOpentofu = true
		}
	}

	if !hasTerraform {
		t.Fatalf("expected terraform at /, got: %+v", results)
	}

	if hasOpentofu {
		t.Fatal("opentofu should not match without .tofu files")
	}
}

// TestScan_TerraformBFixtureNoMatch confirms that the lock file alone (without
// any .tf files) is insufficient for terraform detection. This prevents false
// positives in directories where terraform was removed but the lock file
// remains.
func TestScan_TerraformBFixtureNoMatch(t *testing.T) {
	t.Parallel()

	fixtureDir, absErr := filepath.Abs("../../src/terraform/b")
	if absErr != nil {
		t.Fatalf("failed to resolve path: %v", absErr)
	}

	_, statErr := os.Stat(fixtureDir)
	if statErr != nil {
		t.Skipf("fixture directory not available: %s", fixtureDir)
	}

	results, scanErr := Scan(fixtureDir)
	if scanErr != nil {
		t.Fatalf("unexpected error: %v", scanErr)
	}

	for _, r := range results {
		if r.Directory == "/" && r.Ecosystem == "terraform" {
			t.Fatal("terraform should not match without .tf files")
		}
	}
}

// TestScan_OpenTofuCFixtureNoMatch mirrors the terraform negative test: the
// lock file without .tofu files must not produce opentofu. This catches
// over-eager pattern matching that might treat the lock file alone as
// sufficient.
func TestScan_OpenTofuCFixtureNoMatch(t *testing.T) {
	t.Parallel()

	fixtureDir, absErr := filepath.Abs("../../src/opentofu/c")
	if absErr != nil {
		t.Fatalf("failed to resolve path: %v", absErr)
	}

	_, statErr := os.Stat(fixtureDir)
	if statErr != nil {
		t.Skipf("fixture directory not available: %s", fixtureDir)
	}

	results, scanErr := Scan(fixtureDir)
	if scanErr != nil {
		t.Fatalf("unexpected error: %v", scanErr)
	}

	for _, r := range results {
		if r.Directory == "/" && r.Ecosystem == "opentofu" {
			t.Fatal("opentofu should not match without .tofu files")
		}
	}
}

// TestScan_MultipleEcosystemsSameDirectory confirms that independent ecosystem
// rules don't interfere with each other. A real-world Go project with Docker
// support should get both gomod and docker entries — missing either would leave
// dependencies unmonitored.
func TestScan_MultipleEcosystemsSameDirectory(t *testing.T) {
	t.Parallel()

	// Create a directory with both go.mod and Dockerfile.
	root := t.TempDir()

	writeErr := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n"), 0o0666)
	if writeErr != nil {
		t.Fatalf("failed to write go.mod: %v", writeErr)
	}

	writeErr = os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("FROM alpine\n"), 0o0666)
	if writeErr != nil {
		t.Fatalf("failed to write Dockerfile: %v", writeErr)
	}

	results, scanErr := Scan(root)
	if scanErr != nil {
		t.Fatalf("unexpected error: %v", scanErr)
	}

	hasGomod := false
	hasDocker := false

	for _, r := range results {
		if r.Directory == "/" && r.Ecosystem == "gomod" {
			hasGomod = true
		}

		if r.Directory == "/" && r.Ecosystem == "docker" {
			hasDocker = true
		}
	}

	if !hasGomod {
		t.Fatal("expected gomod in results")
	}

	if !hasDocker {
		t.Fatal("expected docker in results")
	}
}

// TestScan_BunPrecedenceOverNpm protects against the scenario where a
// bun-managed project (package.json + bun.lock) would generate both bun and npm
// entries, causing duplicate and conflicting dependency update PRs.
func TestScan_BunPrecedenceOverNpm(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	writeErr := os.WriteFile(
		filepath.Join(root, "package.json"), []byte("{}"), 0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write package.json: %v", writeErr)
	}

	writeErr = os.WriteFile(
		filepath.Join(root, "bun.lock"), []byte(""), 0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write bun.lock: %v", writeErr)
	}

	assertPrecedenceWinnerOnly(t, root, "bun", "npm")
}

// TestScan_UvPrecedenceOverPip protects against the scenario where a uv-managed
// Python project would generate both uv and pip entries, causing the same
// dependencies to be tracked twice with potentially conflicting update
// strategies.
func TestScan_UvPrecedenceOverPip(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	writeErr := os.WriteFile(
		filepath.Join(root, "uv.lock"), []byte(""), 0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write uv.lock: %v", writeErr)
	}

	writeErr = os.WriteFile(
		filepath.Join(root, "requirements.txt"), []byte("flask\n"), 0o0666,
	)
	if writeErr != nil {
		t.Fatalf("failed to write requirements.txt: %v", writeErr)
	}

	assertPrecedenceWinnerOnly(t, root, "uv", "pip")
}

// assertPrecedenceWinnerOnly is a shared helper that reduces repetition across
// precedence tests. It encodes the invariant that exactly the winner must be
// present and the loser must be absent — the core correctness property of
// precedence.
func assertPrecedenceWinnerOnly(t *testing.T, root, winner, loser string) {
	t.Helper()

	results, scanErr := Scan(root)
	if scanErr != nil {
		t.Fatalf("unexpected error: %v", scanErr)
	}

	hasWinner := false
	hasLoser := false

	for _, r := range results {
		if r.Directory == "/" && r.Ecosystem == winner {
			hasWinner = true
		}

		if r.Directory == "/" && r.Ecosystem == loser {
			hasLoser = true
		}
	}

	if !hasWinner {
		t.Fatalf("expected %s in results", winner)
	}

	if hasLoser {
		t.Fatalf("%s should not appear when %s is present", loser, winner)
	}
}

// TestScan_NpmWithoutBun confirms that precedence suppression is conditional —
// npm must still appear when bun is not detected, so pure npm/yarn/pnpm
// projects aren't broken by overzealous filtering.
func TestScan_NpmWithoutBun(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	writeErr := os.WriteFile(filepath.Join(root, "package.json"), []byte("{}"), 0o0666)
	if writeErr != nil {
		t.Fatalf("failed to write package.json: %v", writeErr)
	}

	results, scanErr := Scan(root)
	if scanErr != nil {
		t.Fatalf("unexpected error: %v", scanErr)
	}

	hasNpm := false

	for _, r := range results {
		if r.Directory == "/" && r.Ecosystem == "npm" {
			hasNpm = true
		}
	}

	if !hasNpm {
		t.Fatal("expected npm in results when bun is not present")
	}
}

// TestScan_PipWithoutUv mirrors the npm/bun test: pip must still appear when uv
// is absent, so standard Python projects using pip/pipenv/setuptools aren't
// affected by the uv precedence rule.
func TestScan_PipWithoutUv(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	writeErr := os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("flask\n"), 0o0666)
	if writeErr != nil {
		t.Fatalf("failed to write requirements.txt: %v", writeErr)
	}

	results, scanErr := Scan(root)
	if scanErr != nil {
		t.Fatalf("unexpected error: %v", scanErr)
	}

	hasPip := false

	for _, r := range results {
		if r.Directory == "/" && r.Ecosystem == "pip" {
			hasPip = true
		}
	}

	if !hasPip {
		t.Fatal("expected pip in results when uv is not present")
	}
}

// TestScan_EcosystemFixturesCaseInsensitive tests fixtures that rely on
// case-insensitive filesystem behavior (macOS HFS+/APFS). These fixtures use
// non-canonical casing (e.g., "build.BAZEL") that only matches on
// case-insensitive filesystems. We skip on Linux because the test would fail
// due to the OS, not our code.
func TestScan_EcosystemFixturesCaseInsensitive(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "darwin" {
		t.Skip("skipping case-insensitive fixtures: requires macOS (HFS+/APFS)")
	}

	tests := []struct {
		name      string
		fixture   string
		ecosystem string
	}{
		{name: "bazel/d", fixture: "../../src/bazel/d", ecosystem: "bazel"},
		{name: "bundler/d", fixture: "../../src/bundler/d", ecosystem: "bundler"},
		{name: "cargo/b", fixture: "../../src/cargo/b", ecosystem: "cargo"},
		{name: "nuget/b", fixture: "../../src/nuget/b", ecosystem: "nuget"},
		{name: "nuget/c", fixture: "../../src/nuget/c", ecosystem: "nuget"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assertFixtureEcosystem(t, tc.fixture, tc.ecosystem)
		})
	}
}
