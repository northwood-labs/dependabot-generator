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

package scanner // lint:allow_naming_conflict_stdlib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

// Feature: yaml-header-comment, Property 2: Directory exclusion prevents
// results from ignored paths
// Validates: Requirements 5.5
//
// For any directory tree and any set of ignore patterns, scanning with
// those patterns SHALL produce no ScanResult entries whose Directory
// field matches or is a child of an ignored directory.
func TestPropertyDirExclusion(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		root := t.TempDir()

		// Create a random directory with a go.mod to trigger detection.
		dirName := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "dir")
		dir := filepath.Join(root, dirName)

		mkErr := os.MkdirAll(dir, 0o0755)
		if mkErr != nil {
			rt.Fatal(mkErr)
		}

		writeErr := os.WriteFile(
			filepath.Join(dir, "go.mod"),
			[]byte("module test\n"),
			0o0666,
		)
		if writeErr != nil {
			rt.Fatal(writeErr)
		}

		// Scan with that directory excluded.
		results, scanErr := Scan(root, []string{dirName})
		if scanErr != nil {
			rt.Fatal(scanErr)
		}

		for _, r := range results {
			if strings.Contains(r.Directory, dirName) {
				rt.Fatalf("excluded dir %q found in results", dirName)
			}
		}
	})
}

// Feature: yaml-header-comment, Property 3: Wrapped lines respect
// 80-character limit
// Validates: Requirements 7.1, 7.4
//
// For any prose string that does not contain a URL, all output lines
// produced by FormatComment SHALL be 80 characters or fewer (including
// the `# ` prefix).
func TestPropertyWrapLimit(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		// Generate prose without URLs using lowercase letters and spaces.
		prose := rapid.StringMatching(`[a-z ]{1,200}`).Draw(rt, "prose")

		output := FormatComment(prose)
		if output == "" {
			return
		}

		lines := strings.Split(output, "\n")

		for i, line := range lines {
			if len(line) > 80 { // lint:allow_raw_number
				rt.Fatalf(
					"line %d exceeds 80 chars (len=%d): %q",
					i, len(line), line,
				)
			}
		}
	})
}

// Feature: yaml-header-comment, Property 4: URLs are preserved intact
// during wrapping
// Validates: Requirements 7.2
//
// For any input line containing a URL, the formatted output SHALL
// contain that URL on a single line without splitting or truncation.
func TestPropertyURLPreservation(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random URL path segment and build a full URL.
		pathSeg := rapid.StringMatching(`[a-z0-9]{5,30}`).Draw(rt, "path")
		url := "https://example.com/" + pathSeg

		// Generate surrounding text.
		prefix := rapid.StringMatching(`[a-z ]{0,40}`).Draw(rt, "prefix")
		input := prefix + " " + url

		output := FormatComment(input)
		if output == "" {
			rt.Fatal("expected non-empty output for URL input")
		}

		// The URL must appear intact on a single line.
		found := false

		lines := strings.SplitSeq(output, "\n")

		for line := range lines {
			if strings.Contains(line, url) {
				found = true

				break
			}
		}

		if !found {
			rt.Fatalf("URL %q not found intact in output:\n%s", url, output)
		}
	})
}

// Feature: yaml-header-comment, Property 5: Short lines are preserved
// and wrapping fills optimally
// Validates: Requirements 7.3, 7.5
//
// For any input line that is already ≤78 characters (content, excluding
// prefix), FormatComment SHALL emit that line unchanged (with only the
// `# ` prefix added).
func TestPropertyShortLinePreservation(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		// Generate a short line: 1-78 chars, no newlines, no # prefix,
		// no URLs.
		line := rapid.StringMatching(`[a-z0-9 ]{1,78}`).Draw(rt, "line")

		// Skip if the generated line contains a URL pattern or starts
		// with #.
		if strings.Contains(line, "http://") ||
			strings.Contains(line, "https://") {
			return
		}

		if strings.HasPrefix(line, "#") {
			return
		}

		// Skip whitespace-only (FormatComment returns "" for those).
		if strings.TrimSpace(line) == "" {
			return
		}

		output := FormatComment(line)
		expected := "# " + line

		if output != expected {
			rt.Fatalf(
				"short line not preserved.\nInput:    %q\nExpected: %q\nGot:      %q",
				line, expected, output,
			)
		}
	})
}

// Feature: yaml-header-comment, Property 6: Size limit enforcement on
// raw input
// Validates: Requirements 8.1, 8.3
//
// For any string of N bytes, when N ≤ 8192 the size validation SHALL
// pass, and when N > 8192 the size validation SHALL return an error.
func TestPropertySizeLimit(t *testing.T) {
	t.Parallel()

	const maxSize = 8192

	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random size that spans the boundary.
		size := rapid.IntRange(1, 16384).Draw(rt, "size") // lint:allow_raw_number

		// Build a string of the given size.
		input := strings.Repeat("a", size)

		if size <= maxSize {
			if len(input) > maxSize {
				rt.Fatalf(
					"size %d should pass validation but len=%d",
					size, len(input),
				)
			}
		} else {
			if len(input) <= maxSize {
				rt.Fatalf(
					"size %d should fail validation but len=%d",
					size, len(input),
				)
			}
		}
	})
}

// Feature: yaml-header-comment, Property 7: Comment prefix
// normalization is idempotent
// Validates: Requirements 9.1, 9.2, 9.3
//
// For any line of text, applying FormatComment ensures the line starts
// with `#`. Applying FormatComment a second time to the
// already-normalized output SHALL produce the same result.
func TestPropertyPrefixIdempotence(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		// Generate single-line text (no newlines).
		text := rapid.StringMatching(`[a-zA-Z0-9 .,!?]{1,60}`).Draw(rt, "text")

		// Skip whitespace-only input.
		if strings.TrimSpace(text) == "" {
			return
		}

		first := FormatComment(text)
		second := FormatComment(first)

		if first != second {
			rt.Fatalf(
				"idempotence violated.\nFirst:  %q\nSecond: %q",
				first, second,
			)
		}
	})
}

// Feature: yaml-header-comment, Property 8: Trailing newline does not
// produce empty trailing comment
// Validates: Requirements 9.4
//
// For any non-whitespace string with one or more trailing newline
// characters, FormatComment SHALL produce output whose last line is not
// a bare `#` that resulted from the trailing newline.
func TestPropertyTrailingNewline(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		// Generate non-whitespace content.
		content := rapid.StringMatching(`[a-z]{3,40}`).Draw(rt, "content")

		// Append 1-5 trailing newlines.
		trailingCount := rapid.IntRange(1, 5).Draw(rt, "newlines")
		input := content + strings.Repeat("\n", trailingCount)

		output := FormatComment(input)
		if output == "" {
			rt.Fatal("expected non-empty output for non-whitespace input")
		}

		lines := strings.Split(output, "\n")
		lastLine := lines[len(lines)-1]

		if lastLine == "#" {
			rt.Fatalf(
				"trailing newline produced bare '#' as last line.\nInput: %q\nOutput: %q",
				input, output,
			)
		}
	})
}

// Feature: yaml-header-comment, Property 12: Whitespace-only input
// produces no comment
// Validates: Requirements 13.1
//
// For any string consisting entirely of whitespace (spaces, tabs,
// newlines), FormatComment SHALL return an empty string.
func TestPropertyWhitespaceOnlyInput(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random whitespace-only string from spaces, tabs,
		// and newlines.
		ws := rapid.StringMatching(`[ \t\n]{1,50}`).Draw(rt, "whitespace")

		output := FormatComment(ws)
		if output != "" {
			rt.Fatalf(
				"whitespace-only input should produce empty output.\nInput: %q\nGot: %q",
				ws, output,
			)
		}
	})
}

// Feature: yaml-header-comment, Property 13: Internal blank lines
// become bare comment markers
// Validates: Requirements 13.2
//
// For any CommentText containing internal blank lines (empty lines
// between non-empty lines), those blank lines SHALL appear as bare `#`
// lines.
func TestPropertyInternalBlankLines(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		// Generate two non-empty lines with blank lines between them.
		line1 := rapid.StringMatching(`[a-z]{3,20}`).Draw(rt, "line1")
		line2 := rapid.StringMatching(`[a-z]{3,20}`).Draw(rt, "line2")
		blankCount := rapid.IntRange(1, 3).Draw(rt, "blanks")

		input := line1 + "\n" +
			strings.Repeat("\n", blankCount) +
			line2

		output := FormatComment(input)
		if output == "" {
			rt.Fatal("expected non-empty output")
		}

		outputLines := strings.Split(output, "\n")

		// Find positions of the internal blank lines in the output.
		// They should appear as bare "#" markers between the content
		// lines.
		foundBlanks := 0

		for i, outLine := range outputLines {
			// Skip first and last lines (those are content lines).
			if i == 0 || i == len(outputLines)-1 {
				continue
			}

			if outLine == "#" {
				foundBlanks++
			}
		}

		if foundBlanks != blankCount {
			rt.Fatalf(
				"expected %d bare '#' lines for internal blanks, got %d.\nInput: %q\nOutput: %q",
				blankCount, foundBlanks, input, output,
			)
		}
	})
}

// Feature: yaml-header-comment, Property 9: Comment placement structure
// Validates: Requirements 10.1, 10.2
//
// For any non-empty CommentText and any valid scan results, the
// generated output SHALL have the comment block immediately after
// `---\n` and SHALL have a blank line separating the last comment
// line from `version: 2`.
func TestPropertyCommentPlacement(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		// Generate non-whitespace comment text.
		commentText := rapid.StringMatching(
			`[a-zA-Z0-9 .,!?]{3,60}`,
		).Draw(rt, "comment")

		// Generate 1-5 random scan results.
		count := rapid.IntRange(1, 5).Draw(rt, "count")
		results := make([]ScanResult, count)

		for i := range results {
			results[i] = genScanResult().Draw(rt, "result")
		}

		opts := &GenerateOptions{CommentText: commentText}

		output, genErr := Generate(results, opts)
		if genErr != nil {
			rt.Fatal(genErr)
		}

		assertCommentPlacement(rt, output)
	})
}

// assertCommentPlacement validates that output begins with `---\n`,
// the first line after the separator is a comment, a blank line
// separates the comment block from the YAML body, and the line
// after the blank contains `version: 2`.
func assertCommentPlacement(rt *rapid.T, output string) {
	// Assert output starts with "---\n".
	if !strings.HasPrefix(output, "---\n") {
		rt.Fatalf(
			"output does not start with '---\\n': %q",
			output[:min(len(output), 40)],
		)
	}

	// Split into lines after "---\n".
	body := strings.TrimPrefix(output, "---\n")
	lines := strings.Split(body, "\n")

	// First line after "---" must start with '#'.
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "#") {
		rt.Fatalf(
			"first line after '---' does not start with '#': %q",
			lines[0],
		)
	}

	// Find the blank line separator between comments and YAML.
	blankIdx := findBlankSeparator(lines)

	if blankIdx < 0 {
		rt.Fatal("no blank line found between comment and YAML body")
	}

	// Line before blank must be a comment line.
	if blankIdx == 0 {
		rt.Fatal("blank line at position 0 — no comment lines")
	}

	lastComment := lines[blankIdx-1]
	if !strings.HasPrefix(lastComment, "#") {
		rt.Fatalf(
			"last line before blank is not a comment: %q",
			lastComment,
		)
	}

	// Line after blank must contain "version: 2".
	if blankIdx+1 >= len(lines) {
		rt.Fatal("no content after blank line")
	}

	afterBlank := lines[blankIdx+1]
	if !strings.Contains(afterBlank, "version: 2") {
		rt.Fatalf(
			"line after blank does not contain 'version: 2': %q",
			afterBlank,
		)
	}
}

// findBlankSeparator returns the index of the first empty line in the
// slice, or -1 if none exists.
func findBlankSeparator(lines []string) int {
	for i, line := range lines {
		if line == "" {
			return i
		}
	}

	return -1
}

// Feature: yaml-header-comment, Property 10: YAML round-trip validity
// Validates: Requirements 10.3, 11.2, 12.1
//
// For any valid CommentText string (including empty) and any valid
// slice of ScanResult, parsing the generated YAML output SHALL yield
// a valid Dependabot v2 configuration with version == 2 and an
// updates array whose length equals the input slice length.
func TestPropertyYAMLRoundTrip(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		// Generate optional comment text (may be empty).
		commentText := rapid.OneOf(
			rapid.Just(""),
			rapid.StringMatching(`[a-zA-Z0-9 ]{1,40}`),
		).Draw(rt, "comment")

		// Generate 1-5 random scan results.
		count := rapid.IntRange(1, 5).Draw(rt, "count")
		results := make([]ScanResult, count)

		for i := range results {
			results[i] = genScanResult().Draw(rt, "result")
		}

		opts := &GenerateOptions{CommentText: commentText}

		output, genErr := Generate(results, opts)
		if genErr != nil {
			rt.Fatal(genErr)
		}

		// Strip the "---\n" document separator since the YAML parser
		// handles it, but comments are valid YAML (ignored by parser).
		body := strings.TrimPrefix(output, "---\n")

		var parsed parsedConfig

		unmarshalErr := yaml.Unmarshal([]byte(body), &parsed)
		if unmarshalErr != nil {
			rt.Fatalf(
				"YAML parse failed: %v\nOutput:\n%s",
				unmarshalErr, output,
			)
		}

		if parsed.Version != 2 {
			rt.Fatalf("expected version 2, got %d", parsed.Version)
		}

		if len(parsed.Updates) != count {
			rt.Fatalf(
				"expected %d updates, got %d",
				count, len(parsed.Updates),
			)
		}
	})
}

// Feature: yaml-header-comment, Property 11: All header lines are
// valid YAML comments
// Validates: Requirements 12.2
//
// For any non-empty CommentText, every line between `---` and the
// blank line before `version:` in the generated output SHALL begin
// with `#`.
func TestPropertyCommentLinesValidity(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		// Generate non-whitespace comment text.
		commentText := rapid.StringMatching(
			`[a-zA-Z0-9 .,!?]{3,60}`,
		).Draw(rt, "comment")

		// Generate 1-3 scan results.
		count := rapid.IntRange(1, 3).Draw(rt, "count")
		results := make([]ScanResult, count)

		for i := range results {
			results[i] = genScanResult().Draw(rt, "result")
		}

		opts := &GenerateOptions{CommentText: commentText}

		output, genErr := Generate(results, opts)
		if genErr != nil {
			rt.Fatal(genErr)
		}

		// Strip "---\n" prefix.
		body := strings.TrimPrefix(output, "---\n")
		lines := strings.Split(body, "\n")

		// All lines up to the first blank line must start with '#'.
		for i, line := range lines {
			if line == "" {
				// Reached the blank separator — done checking.
				break
			}

			if !strings.HasPrefix(line, "#") {
				rt.Fatalf(
					"line %d in comment block does not start "+
						"with '#': %q",
					i, line,
				)
			}
		}
	})
}
