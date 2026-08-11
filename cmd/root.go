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

import (
	"context"
	"os"
	"strings"

	"charm.land/fang/v2"
	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	clihelpers "go.nwlabs.dev/cli-helpers/v2"
)

var (
	// fVerbose is a count (not a bool) so users can request progressive
	// verbosity levels: -v for basic, -vv for detailed, -vvv for trace-level
	// output — matching conventions from tools like ssh and curl.
	fVerbose int

	// rootCmd is the namespace command — it deliberately has no RunE so that
	// invoking the binary without a subcommand falls through to Cobra's
	// automatic help display.
	rootCmd = &cobra.Command{
		Use:   "dependabot-generator",
		Short: "Reviews the file structure for files which match Dependabot ecosystem patterns.",
		Long: clihelpers.LongHelpText(`
		Reviews the file structure for files which match Dependabot ecosystem patterns.

		Generates a default .github/dependabot.yml file based on these files.
		`),
		Example: strings.TrimSpace(dedent.Dedent(`
		# Generate for the local project directory.
		dependabot-generator run .

		# Get help.
		dependabot-generator --help

		# View long-form version info.
		dependabot-generator version
		`)),
	}
)

func init() { // lint:allow_init
	// Persistent flags are registered in init() because Cobra requires all
	// flags to be defined before Execute() is called. This is a framework
	// constraint — Cobra parses the full flag set during command dispatch.
	rootCmd.PersistentFlags().CountVarP(
		&fVerbose, "verbose", "v",
		"increase verbosity level (can be used multiple times)",
	)
}

// Execute is the single call that main.go makes to hand control to Cobra. It
// uses fang to wrap execution with terminal color detection, ensuring the CLI
// renders correctly on both interactive terminals and piped output. The
// [os.Exit](1) is here because Cobra signals failure via a returned error
// rather than exiting itself, so the top-level caller must translate that into
// a non-zero exit code for the shell.
func Execute() {
	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		os.Exit(1) // lint:allow_exit
	}
}
