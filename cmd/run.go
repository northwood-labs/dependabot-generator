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
	"strings"

	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	clihelpers "go.nwlabs.dev/cli-helpers/v2"
)

// runCmd represents the run command
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
	RunE: func(cmd *cobra.Command, args []string) error {
		// path := "."
		// if len(args) > 0 {
		// 	path = args[0]
		// }

		// proj, err := scanner.Scan(path)
		// if err != nil {
		// 	return fmt.Errorf("error when scanning project files: %w", err)
		// }

		// if err := scanner.Generate(proj); err != nil {
		// 	return fmt.Errorf("error when generating Dependabot configuration: %w", err)
		// }

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
