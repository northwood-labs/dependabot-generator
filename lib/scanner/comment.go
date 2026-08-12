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

import "strings"

// commentPrefix is the standard YAML comment prefix used for header
// comment lines.
const commentPrefix = "# "

// FormatComment applies prefix normalization, text wrapping, and
// trailing newline stripping to raw comment text. It returns the
// formatted comment block ready for insertion, where each line is
// prefixed with `#`. Returns an empty string for whitespace-only
// input.
//
// Processing rules applied in order:
//   - Strip trailing newline from input.
//   - If input is whitespace-only after stripping, return empty.
//   - Split into lines and process each independently.
//   - Lines already starting with `#` are preserved unchanged.
//   - Non-`#` lines within the 78-character content limit are
//     prefixed with `# `.
//   - Non-`#` lines containing a URL are prefixed with `# `
//     without wrapping, even if they exceed the limit.
//   - Non-`#` lines exceeding 78 characters with no URL are
//     reflowed to fit within the limit.
//   - Internal blank lines become bare `#` lines.
func FormatComment(raw string) string {
	// Strip trailing newline.
	stripped := strings.TrimRight(raw, "\n")

	// If input is whitespace-only after stripping, return empty.
	if strings.TrimSpace(stripped) == "" {
		return ""
	}

	lines := strings.Split(stripped, "\n")
	contentLimit := 78

	var result []string

	for _, line := range lines {
		// Internal blank lines become bare `#`.
		if line == "" {
			result = append(result, "#")

			continue
		}

		// Lines already starting with `#` are preserved unchanged.
		if strings.HasPrefix(line, "#") {
			result = append(result, line)

			continue
		}

		// Line fits within the content limit — just prefix it.
		if len(line) <= contentLimit {
			result = append(result, commentPrefix+line)

			continue
		}

		// Line contains a URL — preserve intact regardless of length.
		if containsURL(line) {
			result = append(result, commentPrefix+line)

			continue
		}

		// Line exceeds limit with no URL — wrap it.
		wrapped := WrapLine(line, contentLimit)
		for _, w := range wrapped {
			result = append(result, commentPrefix+w)
		}
	}

	return strings.Join(result, "\n")
}

// WrapLine wraps a single line of text to the given character limit,
// preserving URLs intact on a single line even if they exceed the
// limit. It fills lines optimally — no early wrapping if the next
// word fits within the limit. Returns a slice of content-only
// strings without the `# ` prefix.
//
// The limit parameter is the content width (78 chars for standard
// use), not including the `# ` prefix. FormatComment passes 78 as
// the limit since the total line is 80 = 2 prefix + 78 content.
func WrapLine(line string, limit int) []string {
	// If the line contains a URL, return it as-is (URL exception).
	if containsURL(line) {
		return []string{line}
	}

	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string

	var current strings.Builder

	for _, word := range words {
		if current.Len() == 0 {
			// First word on a new line — always add it.
			current.WriteString(word)

			continue
		}

		// Check if adding this word (with a space) would exceed
		// the limit.
		if current.Len()+1+len(word) <= limit {
			current.WriteByte(' ')
			current.WriteString(word)
		} else {
			// Emit the current line and start a new one.
			lines = append(lines, current.String())
			current.Reset()
			current.WriteString(word)
		}
	}

	// Emit the final line.
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}

	return lines
}

// containsURL checks whether a line contains an HTTP or HTTPS URL
// pattern.
func containsURL(line string) bool {
	return strings.Contains(line, "http://") ||
		strings.Contains(line, "https://")
}
