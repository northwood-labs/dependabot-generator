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

// ecosystemRules encodes detection patterns derived from the Dependabot source
// code (see docs/ecosystem.md). The table is sorted alphabetically by
// identifier for human readability when reviewing or extending rules — runtime
// evaluation order does not depend on this ordering because every rule is
// checked independently per directory.
var ecosystemRules = []EcosystemRule{
	{
		Identifier: "bazel",
		Files:      [][]string{{"BUILD.bazel"}, {"BUILD"}},
	},
	{
		// Bun has two detection paths: bunfig.toml is the standalone config
		// (always means bun), while package.json + bun.lock proves bun manages
		// the project rather than npm (package.json alone defaults to npm).
		Identifier: "bun",
		Files:      [][]string{{"bunfig.toml"}, {"package.json", "bun.lock"}},
	},
	{
		Identifier: "bundler",
		Files:      [][]string{{"gems.rb"}, {"Gemfile"}, {"*.gemspec"}},
	},
	{
		Identifier: "cargo",
		Files:      [][]string{{"Cargo.toml"}},
	},
	{
		Identifier: "composer",
		Files:      [][]string{{"composer.json"}},
	},
	{
		Identifier: "conda",
		Files:      [][]string{{"environment.yaml"}, {"environment.yml"}},
	},
	{
		Identifier: "deno",
		Files:      [][]string{{"deno.json"}, {"deno.jsonc"}},
	},
	{
		// Devcontainers supports multiple valid locations per the devcontainer
		// spec: root-level .devcontainer.json, a dedicated .devcontainer/
		// folder, or nested subdirs within .devcontainer/ for multi-container
		// setups.
		Identifier: "devcontainers",
		Files: [][]string{
			{".devcontainer.json"},
			{".devcontainer/devcontainer.json"},
			{".devcontainer/**/devcontainer.json"},
		},
	},
	{
		Identifier: "docker",
		Files:      [][]string{{"Dockerfile*"}, {"Containerfile*"}},
	},
	{
		// Docker Compose was renamed from "docker-compose" to "compose" in v2,
		// so both prefixes are valid. YAML allows either .yml or .yaml
		// extension.
		Identifier: "docker-compose",
		Files: [][]string{
			{"docker-compose*.yml"},
			{"docker-compose*.yaml"},
			{"compose*.yml"},
			{"compose*.yaml"},
		},
	},
	{
		Identifier: "dotnet-sdk",
		Files:      [][]string{{"global.json"}},
	},
	{
		Identifier: "elm",
		Files:      [][]string{{"elm-package.json"}},
	},
	{
		Identifier: "github-actions",
		Files: [][]string{
			{"action.yml"},
			{"action.yaml"},
			{".github/workflows/*.yml"},
			{".github/workflows/*.yaml"},
		},
	},
	{
		Identifier: "gitsubmodule",
		Files:      [][]string{{".gitmodules"}},
	},
	{
		Identifier: "gomod",
		Files:      [][]string{{"go.mod"}},
	},
	{
		Identifier: "gradle",
		Files:      [][]string{{"build.gradle"}, {"build.gradle.kts"}},
	},
	{
		Identifier: "helm",
		Files:      [][]string{{"Chart.lock"}},
	},
	{
		Identifier: "julia",
		Files: [][]string{
			{"Project.toml"},
			{"JuliaProject.toml"},
			{"Manifest*.toml"},
			{"JuliaManifest*.toml"},
		},
	},
	{
		Identifier: "maven",
		Files:      [][]string{{"pom.xml"}},
	},
	{
		Identifier: "mix",
		Files:      [][]string{{"mix.exs"}},
	},
	{
		Identifier: "nix",
		Files:      [][]string{{"flake.nix"}},
	},
	{
		// npm uses just package.json intentionally — it's the default
		// assumption for any JS/TS project. When a more specific tool like bun
		// is also detected, precedence rules suppress npm rather than
		// complicating this rule.
		Identifier: "npm",
		Files:      [][]string{{"package.json"}},
	},
	{
		Identifier: "nuget",
		Files:      [][]string{{"NuGet.Config"}},
	},
	{
		// OpenTofu requires both the lock file AND a .tofu file. The lock file
		// alone proves Terraform/OpenTofu was run, but the .tofu extension is
		// what distinguishes OpenTofu from Terraform — without it, the
		// terraform rule fires instead.
		Identifier: "opentofu",
		Files:      [][]string{{".terraform.lock.hcl", "*.tofu"}},
	},
	{
		// pip is the catch-all for Python dependency management. Any recognized
		// manifest triggers it. When the more specific uv tool is also detected
		// (via uv.lock), precedence rules suppress pip automatically.
		Identifier: "pip",
		Files: [][]string{
			{"*requirements*.txt"},
			{"*requirements*.in"},
			{"setup.cfg"},
			{"setup.py"},
			{"pyproject.toml"},
			{"Pipfile"},
		},
	},
	{
		Identifier: "pre-commit",
		Files: [][]string{
			{".pre-commit-config.yml"},
			{".pre-commit-config.yaml"},
			{".pre-commit.yml"},
			{".pre-commit.yaml"},
		},
	},
	{
		Identifier: "pub",
		Files:      [][]string{{"pubspec.yaml"}},
	},
	{
		Identifier: "rust-toolchain",
		Files:      [][]string{{"rust-toolchain"}, {"rust-toolchain.toml"}},
	},
	{
		Identifier: "sbt",
		Files:      [][]string{{"build.sbt"}},
	},
	{
		Identifier: "swift",
		Files:      [][]string{{"Package.swift"}},
	},
	{
		// Terraform requires the lock file AND a .tf file for the same reason
		// as opentofu: the lock file proves active usage, and the .tf extension
		// confirms it's Terraform specifically (not OpenTofu).
		Identifier: "terraform",
		Files:      [][]string{{".terraform.lock.hcl", "*.tf"}},
	},
	{
		Identifier: "uv",
		Files:      [][]string{{"uv.lock"}},
	},
	{
		Identifier: "vcpkg",
		Files:      [][]string{{"vcpkg-configuration.json"}},
	},
}
