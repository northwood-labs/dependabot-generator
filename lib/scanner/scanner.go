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

package scanner

import "github.com/goreleaser/fileglob"

// Scan performs a scan on the project directory in order to identify a set of
// known project files, then associate a Dependabot project type with it.
func Scan(path string) {
	fileglob.Glob("")
}

// Generate creates a Dependabot configuration file based on the project types.
func Generate(proj string) {}
