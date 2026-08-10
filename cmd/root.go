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
	// CLI flags are variables prefixed with lowercase 'f'.
	fVerbose       int

	// rootCmd represents the base command when called without any subcommands.
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
		// Uncomment the following line if your bare application
		// has an action associated with it:
		// RunE: func(cmd *cobra.Command, args []string) error {
		// 	return nil
		// },
	}
)

func init() { // lint:allow_init
	// Here you will define your flags and configuration settings. Cobra
	// supports persistent flags, which, if defined here, will be global for
	// your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file
	// (default is $HOME/.dependabot-generator.yaml)").

	// Cobra also supports local flags, which will only run when this action is
	// called directly.
	rootCmd.PersistentFlags().CountVarP(
		&fVerbose, "verbose", "v",
		"increase verbosity level (can be used multiple times)",
	)
}

// Execute adds all child commands to the root command and sets flags
// appropriately. This is called by main.main(). It only needs to happen once to
// the rootCmd.
//
// We also connect the Fang library here which provides some color in the
// Terminal when run as a CLI.
func Execute() {
	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		os.Exit(1) // lint:allow_exit
	}
}
