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

package cmd

import clihelpers "go.nwlabs.dev/cli-helpers/v2"

// versionCmd is delegated to cli-helpers so that all Northwood Labs CLI tools
// share a consistent version display format. Centralizing this avoids
// duplicating version-screen logic across repositories and ensures updates
// propagate automatically via dependency bumps.
var versionCmd = clihelpers.VersionScreen()

func init() { // lint:allow_init
	// Cobra discovers subcommands only through explicit AddCommand calls — this
	// is the framework's registration mechanism.
	rootCmd.AddCommand(versionCmd)
}
