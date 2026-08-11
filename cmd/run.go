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
	"fmt"
	"os"
	"strings"

	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	clihelpers "go.nwlabs.dev/cli-helpers/v2"
	"go.nwlabs.dev/dependabot-generator/lib/scanner"
	"go.nwlabs.dev/x/logutils"
)

// runCmd represents the run command.
//
// The run command is the primary user-facing entry point. It defaults to "."
// because CLI tools conventionally operate on the current directory when no
// path is given, matching user expectations from git, make, and similar tools.
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs the scanner and generator.",
	Long: clihelpers.LongHelpText(`
	Runs the scanner and generator.

	Scans the contents of the directory for identified project files. When
	project files are identified, the generator will create a Dependabot
	configuration file based on the detected project type(s).

	If a custom path is set, the scanner will scan that path and treat it as the
	root of the repository when generating the Dependabot configuration file.
	`),
	Example: strings.TrimSpace(dedent.Dedent(`
	# Generate for the local project directory.
	dependabot-generator run .

	# Generate for the local project directory.
	dependabot-generator run /path/to/other/directory.
	`)),
	Args: cobra.RangeArgs(0, 1),
	RunE: func(_ *cobra.Command, args []string) error {
		// Default to current directory — the most common case is running the
		// tool from the repo root.
		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		results, scanErr := scanner.Scan(path)
		if scanErr != nil {
			// Wrap with a user-facing description so the error message makes
			// sense without knowing internal function names.
			return fmt.Errorf("error when scanning project files: %w", scanErr)
		}

		output, genErr := scanner.Generate(results)
		if genErr != nil {
			return fmt.Errorf("error when generating Dependabot configuration: %w", genErr)
		}

		// Output goes to stdout so it can be piped or redirected (e.g.,
		// `dependabot-generator run > .github/dependabot.yml`).
		fmt.Fprint(os.Stdout, output)

		return nil
	},
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		logger = logutils.GetDefaultLogger(fVerbose)

		return nil
	},
}

func init() { // lint:allow_init
	rootCmd.AddCommand(runCmd)
}
