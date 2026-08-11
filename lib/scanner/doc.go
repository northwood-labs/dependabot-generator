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

// Package scanner exists as a standalone package so that the core detection
// logic remains decoupled from CLI concerns (flags, output formatting, exit
// codes). This separation lets the scanner be invoked programmatically — from
// tests, CI tooling, or future library consumers — without pulling in Cobra or
// terminal dependencies.
//
// The package solves two problems that Dependabot cannot solve for itself: (1)
// discovering which package ecosystems exist across an arbitrarily nested
// repository tree, and (2) rendering a valid dependabot.yml from those
// discoveries. By combining both responsibilities here, consumers get a single
// Scan → Generate pipeline with no intermediate serialization or coordination
// required.
package scanner // lint:allow_naming_conflict_stdlib
