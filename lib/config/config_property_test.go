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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"pgregory.net/rapid"
)

type (
	// priorityInput holds the generated values for each configuration level in
	// the priority chain.
	priorityInput struct {
		CLIHeader     string
		CLIHeaderFile string
		EnvHeader     string
		LocalHeader   string
		UserHeader    string
	}
)

// drawOptionalString generates a random non-empty string or empty string to
// indicate "not set" for a configuration source.
func drawOptionalString(rt *rapid.T, label string) string {
	isSet := rapid.Bool().Draw(rt, label+"-set")
	if !isSet {
		return ""
	}

	return rapid.StringMatching(`[a-z]{5,10}`).Draw(rt, label)
}

// setupPriorityFixtures writes local and user config files to the filesystem
// when their values are non-empty. Returns the scan directory and XDG config
// directory paths.
func setupPriorityFixtures(t *testing.T, rt *rapid.T, input *priorityInput) (string, string) {
	t.Helper()

	scanDir := t.TempDir()
	xdgDir := t.TempDir()

	if input.LocalHeader != "" {
		writeErr := os.WriteFile(
			filepath.Join(scanDir, ".depgen.toml"),
			fmt.Appendf(nil, "header = %q\n", input.LocalHeader),
			0o0666, // lint:allow_666
		)
		if writeErr != nil {
			rt.Fatal(writeErr)
		}
	}

	if input.UserHeader != "" {
		appDir := filepath.Join(xdgDir, "dependabot-generator")

		mkdirErr := os.MkdirAll(appDir, 0o0755)
		if mkdirErr != nil {
			rt.Fatal(mkdirErr)
		}

		writeErr := os.WriteFile(
			filepath.Join(appDir, "config.toml"),
			fmt.Appendf(nil, "header = %q\n", input.UserHeader),
			0o0666, // lint:allow_666
		)
		if writeErr != nil {
			rt.Fatal(writeErr)
		}
	}

	return scanDir, xdgDir
}

// expectedPriority returns the highest-priority non-empty value from the input,
// falling back to DefaultHeaderURL.
func expectedPriority(input *priorityInput) string {
	expected := DefaultHeaderURL

	if input.UserHeader != "" {
		expected = input.UserHeader
	}

	if input.LocalHeader != "" {
		expected = input.LocalHeader
	}

	if input.EnvHeader != "" {
		expected = input.EnvHeader
	}

	if input.CLIHeaderFile != "" {
		expected = input.CLIHeaderFile
	}

	if input.CLIHeader != "" {
		expected = input.CLIHeader
	}

	return expected
}

// Feature: yaml-header-comment, Property 1: Priority resolution selects
// highest-priority source
//
// Validates: Requirements 1.2, 4.2, 4.3, 5.2, 5.3, 5.4, 6.1, 6.2, 6.3.
//
// TestPropertyPriority_HighestSourceWins verifies that for any combination of
// configuration sources where at least one provides a non-empty header value,
// the resolved HeaderComment equals the value from the highest-priority
// non-empty source.
func TestPropertyPriority_HighestSourceWins(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		input := &priorityInput{
			CLIHeader:     drawOptionalString(rt, "cli-header"),
			CLIHeaderFile: drawOptionalString(rt, "cli-header-file"),
			EnvHeader:     drawOptionalString(rt, "env-header"),
			LocalHeader:   drawOptionalString(rt, "local-header"),
			UserHeader:    drawOptionalString(rt, "user-header"),
		}

		scanDir, xdgDir := setupPriorityFixtures(t, rt, input)
		t.Setenv("XDG_CONFIG_HOME", xdgDir)

		cfg, err := LoadConfig(&LoadOptions{
			CLIHeader:     input.CLIHeader,
			CLIHeaderFile: input.CLIHeaderFile,
			EnvHeader:     input.EnvHeader,
			ScanPath:      scanDir,
		})
		if err != nil {
			rt.Fatal(err)
		}

		expected := expectedPriority(input)
		if cfg.HeaderComment != expected {
			rt.Fatalf(
				"expected %q, got %q", expected, cfg.HeaderComment,
			)
		}
	})
}
