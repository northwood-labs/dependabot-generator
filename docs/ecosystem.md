# Ecosystem patterns

Patterns pulled from the Dependabot source code.

| Ecosystem         | Identifier       | Files                                                                                                     |
|-------------------|------------------|-----------------------------------------------------------------------------------------------------------|
| Bazel             | `bazel`          | `BUILD.bazel` or `BUILD`                                                                                  |
| Bun               | `bun`            | `bunfig.toml` or (`package.json` + `bun.lock`)                                                            |
| Bundler           | `bundler`        | `gems.rb` or `Gemfile` or `*.gemspec`                                                                     |
| Cargo             | `cargo`          | `Cargo.toml`                                                                                              |
| Composer          | `composer`       | `composer.json`                                                                                           |
| Conda             | `conda`          | `environment.yaml` or `environment.yml`,                                                                  |
| Deno              | `deno`           | `deno.json` or `deno.jsonc`                                                                               |
| Dev containers    | `devcontainers`  | `.devcontainer.json` or `.devcontainer/devcontainer.json` or `.devcontainer/**/devcontainer.json`         |
| Docker            | `docker`         | `Dockerfile*` or `Containerfile*`                                                                         |
| Docker Compose    | `docker-compose` | `(docker-)?compose*.ya?ml`                                                                                |
| .NET SDK          | `dotnet-sdk`     | `global.json`                                                                                             |
| elm-package       | `elm`            | `elm-package.json`                                                                                        |
| git submodule     | `gitsubmodule`   | `.gitmodules`                                                                                             |
| GitHub Actions    | `github-actions` | `action.ya?ml` or `.github/workflows/*.ya?ml`                                                             |
| Go modules        | `gomod`          | `go.mod`                                                                                                  |
| Gradle            | `gradle`         | `build.gradle` or `build.gradle.kts`                                                                      |
| Helm Charts       | `helm`           | `Chart.lock`                                                                                              |
| Hex               | `mix`            | `mix.exs`                                                                                                 |
| Julia             | `julia`          | `(Julia)?Project.toml`, `(Julia)?Manifest*.toml`                                                          |
| Maven             | `maven`          | `pom.xml`                                                                                                 |
| Nix flakes        | `nix`            | `flake.nix`                                                                                               |
| npm/pnpm/yarn     | `npm`            | `package.json`                                                                                            |
| NuGet             | `nuget`          | `NuGet.Config`                                                                                            |
| OpenTofu          | `opentofu`       | `.terraform.lock.hcl` + `*.tofu` (prioritized over `terraform`)                                           |
| Python (not `uv`) | `pip`            | `*requirements*.txt` or `*requirements*.in` or `setup.cfg` or `setup.py` or `pyproject.toml` or `Pipfile` |
| pre-commit        | `pre-commit`     | `.pre-commit(-config)?.ya?ml`                                                                             |
| pub               | `pub`            | `pubspec.yaml`                                                                                            |
| Rust toolchain    | `rust-toolchain` | `rust-toolchain(.toml)?`                                                                                  |
| sbt               | `sbt`            | `build.sbt`                                                                                               |
| Swift             | `swift`          | `Package.swift`                                                                                           |
| Terraform         | `terraform`      | `.terraform.lock.hcl` + `*.tf`                                                                            |
| uv                | `uv`             | `uv.lock`                                                                                                 |
| vcpkg             | `vcpkg`          | `vcpkg-configuration.json`                                                                                |
