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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	clihelpers "go.nwlabs.dev/cli-helpers/v2"
	"go.nwlabs.dev/dependabot-generator/lib/config"
	"go.nwlabs.dev/dependabot-generator/lib/scanner"
)

// maxHeaderSize is the maximum allowed size in bytes for resolved
// comment text, measured on the raw input before formatting.
const maxHeaderSize = 8192

var (
	// fHeader is the inline header comment text provided via the --header CLI
	// flag.
	fHeader string

	// fHeaderFile is the path to a file containing header comment text,
	// provided via the --header-file CLI flag.
	fHeaderFile string

	// runCmd represents the run command.
	//
	// The run command is the primary user-facing entry point. It defaults to
	// "." because CLI tools conventionally operate on the current directory
	// when no path is given, matching user expectations from git, make, and
	// similar tools.
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Runs the scanner and generator.",
		Long: clihelpers.LongHelpText(`
		Runs the scanner and generator.

		Scans the contents of the directory for identified project files. When
		project files are identified, the generator will create a Dependabot
		configuration file based on the detected project type(s).

		If a custom path is set, the scanner will scan that path and treat it as
		the root of the repository when generating the Dependabot configuration
		file.
		`),
		Example: strings.TrimSpace(dedent.Dedent(`
		# Generate for the local project directory.
		dependabot-generator run .

		# Generate for the local project directory.
		dependabot-generator run /path/to/other/directory.
		`)),
		Args: cobra.RangeArgs(0, 1),
		RunE: func(_ *cobra.Command, args []string) error {
			// Default to current directory — the most common case is running
			// the tool from the repo root.
			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			// Mutual exclusivity check.
			if fHeader != "" && fHeaderFile != "" {
				return ErrFlagsMutuallyExclusive
			}

			// Read header file if specified.
			headerFileContent := ""
			if fHeaderFile != "" {
				headerFileContent = readHeaderFile(fHeaderFile)

				if headerFileContent == "" {
					return validateHeaderFilePath(fHeaderFile)
				}
			}

			// Environment variable lookup.
			envHeader := os.Getenv("DEPGEN_HEADER")

			// Load config from all sources with priority resolution.
			cfg, loadErr := config.LoadConfig(&config.LoadOptions{
				CLIHeader:     fHeader,
				CLIHeaderFile: headerFileContent,
				EnvHeader:     envHeader,
				ScanPath:      path,
			})
			if loadErr != nil {
				return mapConfigError(loadErr)
			}

			// Validate the resolved config (checks ignore patterns).
			validateErr := config.Validate(cfg)
			if validateErr != nil {
				return fmt.Errorf(
					"%w: %w", ErrIgnorePatternInvalid, validateErr,
				)
			}

			// Size limit enforcement on resolved comment text. The measurement
			// is on the raw input before any formatting.
			if len(cfg.HeaderComment) > maxHeaderSize {
				return ErrHeaderTooLarge
			}

			// Scan with ignore dirs.
			results, scanErr := scanner.Scan(path, cfg.IgnoreDirs)
			if scanErr != nil {
				return fmt.Errorf(
					"error when scanning project files: %w", scanErr,
				)
			}

			// Convert config ecosystem defaults to scanner settings.
			ecoDefaults := make(
				map[string]scanner.EcosystemSettings,
				len(cfg.EcosystemDefaults),
			)

			for k, v := range cfg.EcosystemDefaults {
				ecoDefaults[k] = scanner.EcosystemSettings{
					Fields: v.Fields,
				}
			}

			// Generate with options.
			genOpts := &scanner.GenerateOptions{
				CommentText:       cfg.HeaderComment,
				EcosystemDefaults: ecoDefaults,
			}

			output, genErr := scanner.Generate(results, genOpts)
			if genErr != nil {
				return fmt.Errorf(
					"error when generating Dependabot configuration: %w",
					genErr,
				)
			}

			// Output goes to stdout so it can be piped or redirected (e.g.,
			// `dependabot-generator run > .github/dependabot.yml`).
			fmt.Fprint(os.Stdout, output)

			return nil
		},
	}
)

func init() { // lint:allow_init
	runCmd.Flags().StringVar(
		&fHeader, "header", "",
		"inline header comment text",
	)
	runCmd.Flags().StringVar(
		&fHeaderFile, "header-file", "",
		"path to file containing header comment text",
	)

	rootCmd.AddCommand(runCmd)
}

// readHeaderFile attempts to read the contents of a header file. It returns the
// file contents as a string on success, or an empty string if any step fails.
// Callers should call validateHeaderFilePath to get the appropriate error when
// this returns empty.
func readHeaderFile(path string) string {
	data, readErr := os.ReadFile(path) // lint:allow_dynamic_filename
	if readErr != nil {
		return ""
	}

	return string(data)
}

// validateHeaderFilePath performs validation on a header file path, returning
// the appropriate sentinel error for the failure mode: path well-formedness,
// file existence, or file readability.
func validateHeaderFilePath(path string) error {
	// Well-formedness check: null bytes indicate an invalid path.
	if strings.ContainsRune(path, '\x00') {
		return fmt.Errorf("%w: %s", ErrHeaderFilePathInvalid, path)
	}

	// Existence check.
	_, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return fmt.Errorf(
				"%w: %s", ErrHeaderFileNotFound, path,
			)
		}

		return fmt.Errorf(
			"%w: %s: %w", ErrHeaderFileNotReadable, path, statErr,
		)
	}

	// If stat succeeds but ReadFile failed, it's a readability issue.
	return fmt.Errorf("%w: %s", ErrHeaderFileNotReadable, path)
}

// mapConfigError translates errors from config.LoadConfig into the appropriate
// CLI-layer sentinel errors for user-facing messages.
func mapConfigError(err error) error {
	if errors.Is(err, config.ErrConfigParse) {
		return fmt.Errorf("%w: %w", ErrConfigSyntax, err)
	}

	if errors.Is(err, config.ErrConfigRead) {
		return fmt.Errorf("%w: %w", ErrConfigNotReadable, err)
	}

	return fmt.Errorf("%w: %w", ErrConfigSyntax, err)
}
