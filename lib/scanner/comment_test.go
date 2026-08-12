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
	"strings"
	"testing"
)

func TestFormatComment_EmptyInput(t *testing.T) {
	t.Parallel()

	got := FormatComment("")
	if got != "" {
		t.Fatalf("FormatComment(%q) = %q, want %q", "", got, "")
	}
}

func TestFormatComment_WhitespaceOnly(t *testing.T) {
	t.Parallel()

	cases := []string{
		" ",
		"\t",
		"\n",
		"  \t\n  \n",
		"\n\n\n",
	}

	for _, input := range cases {
		got := FormatComment(input)
		if got != "" {
			t.Fatalf(
				"FormatComment(%q) = %q, want empty string",
				input, got,
			)
		}
	}
}

func TestFormatComment_SimpleLine(t *testing.T) {
	t.Parallel()

	input := "Hello world"
	got := FormatComment(input)
	want := "# Hello world"

	if got != want {
		t.Fatalf(
			"FormatComment(%q) = %q, want %q",
			input, got, want,
		)
	}
}

func TestFormatComment_AlreadyPrefixed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "hash space text",
			input: "# Already prefixed",
			want:  "# Already prefixed",
		},
		{
			name:  "hash no space",
			input: "#no space",
			want:  "#no space",
		},
		{
			name:  "hash only",
			input: "#",
			want:  "#",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := FormatComment(tc.input)
			if got != tc.want {
				t.Fatalf(
					"FormatComment(%q) = %q, want %q",
					tc.input, got, tc.want,
				)
			}
		})
	}
}

func TestFormatComment_TrailingNewline(t *testing.T) {
	t.Parallel()

	input := "Hello world\n"
	got := FormatComment(input)
	want := "# Hello world"

	if got != want {
		t.Fatalf(
			"FormatComment(%q) = %q, want %q",
			input, got, want,
		)
	}

	// Trailing newlines should not produce a trailing bare #.
	if strings.HasSuffix(got, "\n#") {
		t.Fatalf(
			"FormatComment(%q) ends with bare #: %q",
			input, got,
		)
	}
}

func TestFormatComment_InternalBlankLines(t *testing.T) {
	t.Parallel()

	input := "First\n\nSecond"
	got := FormatComment(input)
	want := "# First\n#\n# Second"

	if got != want {
		t.Fatalf(
			"FormatComment(%q) =\n%q\nwant:\n%q",
			input, got, want,
		)
	}
}

func TestFormatComment_LongLineWrapping(t *testing.T) {
	t.Parallel()

	// Build a line that exceeds 78 characters of content.
	// Each word is 8 chars + space, so 10 words = ~89 chars.
	words := make([]string, 10)
	for i := range words {
		words[i] = "wordword"
	}

	input := strings.Join(words, " ")
	got := FormatComment(input)

	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if len(line) > 80 { // lint:allow_raw_number
			t.Fatalf(
				"line %d exceeds 80 chars (len=%d): %q",
				i, len(line), line,
			)
		}
	}

	// Must produce more than one line.
	if len(lines) < 2 { // lint:allow_raw_number
		t.Fatalf(
			"expected wrapping to produce multiple lines, got %d",
			len(lines),
		)
	}
}

func TestFormatComment_URLPreservation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
	}{
		{
			name: "long https URL",
			input: "See https://docs.github.com/github/" +
				"administering-a-repository/" +
				"configuration-options-for-dependency-updates",
		},
		{
			name: "long http URL",
			input: "Visit http://example.com/" +
				"very/long/path/that/exceeds/the/limit/" +
				"for/testing/purposes/only/here",
		},
		{
			name:  "URL mid-line",
			input: "For reference, see https://example.com/docs for details about configuration",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := FormatComment(tc.input)
			lines := strings.Split(got, "\n")

			// URL-bearing lines should NOT be split.
			if len(lines) != 1 {
				t.Fatalf(
					"expected 1 line (URL preserved), got %d: %q",
					len(lines), got,
				)
			}

			// The URL must be present intact.
			if !strings.Contains(got, "https://") &&
				!strings.Contains(got, "http://") {
				t.Fatalf("URL not preserved in output: %q", got)
			}

			// Must have prefix.
			if !strings.HasPrefix(got, "# ") {
				t.Fatalf(
					"expected '# ' prefix, got: %q", got,
				)
			}
		})
	}
}

func TestFormatComment_MixedContent(t *testing.T) {
	t.Parallel()

	input := "Short line\n" +
		"# Already prefixed\n" +
		"https://example.com/very/long/url/path/exceeding/limit\n" +
		"\n" +
		strings.Repeat("mixed ", 20)

	got := FormatComment(input)
	lines := strings.Split(got, "\n")

	// First line: short, gets prefix.
	if !strings.HasPrefix(lines[0], "# Short line") {
		t.Fatalf("line 0 unexpected: %q", lines[0])
	}

	// Second line: already prefixed, preserved.
	if lines[1] != "# Already prefixed" {
		t.Fatalf("line 1 unexpected: %q", lines[1])
	}

	// Third line: URL, preserved intact.
	if !strings.Contains(lines[2], "https://example.com") {
		t.Fatalf("line 2 should contain URL: %q", lines[2])
	}

	// Fourth line: internal blank → bare #.
	if lines[3] != "#" { // lint:allow_raw_number
		t.Fatalf("line 3 should be bare #: %q", lines[3])
	}

	// Remaining lines: wrapped long line, all ≤ 80 chars.
	for i := 4; i < len(lines); i++ {
		if len(lines[i]) > 80 { // lint:allow_raw_number
			t.Fatalf(
				"line %d exceeds 80 chars: %q",
				i, lines[i],
			)
		}
	}
}

func TestFormatComment_MultipleLines(t *testing.T) {
	t.Parallel()

	input := "Line one\nLine two\nLine three"
	got := FormatComment(input)
	want := "# Line one\n# Line two\n# Line three"

	if got != want {
		t.Fatalf(
			"FormatComment(%q) =\n%q\nwant:\n%q",
			input, got, want,
		)
	}
}

func TestWrapLine_ShortLine(t *testing.T) {
	t.Parallel()

	input := "Short line"
	got := WrapLine(input, 78) // lint:allow_raw_number

	if len(got) != 1 {
		t.Fatalf("expected 1 element, got %d: %v", len(got), got)
	}

	if got[0] != input {
		t.Fatalf("WrapLine(%q, 78) = %q, want %q", input, got[0], input)
	}
}

func TestWrapLine_LongLine(t *testing.T) {
	t.Parallel()

	// Create a line longer than 78 chars.
	words := make([]string, 12)
	for i := range words {
		words[i] = "testing"
	}

	input := strings.Join(words, " ")
	got := WrapLine(input, 78) // lint:allow_raw_number

	if len(got) < 2 { // lint:allow_raw_number
		t.Fatalf(
			"expected multiple lines, got %d: %v",
			len(got), got,
		)
	}

	// Each line must be ≤ 78 chars.
	for i, line := range got {
		if len(line) > 78 { // lint:allow_raw_number
			t.Fatalf(
				"line %d exceeds 78 chars (len=%d): %q",
				i, len(line), line,
			)
		}
	}
}

func TestWrapLine_URLLine(t *testing.T) {
	t.Parallel()

	input := "Check https://docs.github.com/github/" +
		"administering-a-repository/very-long-page-name"
	got := WrapLine(input, 78) // lint:allow_raw_number

	if len(got) != 1 {
		t.Fatalf(
			"URL line should not wrap, got %d elements: %v",
			len(got), got,
		)
	}

	if got[0] != input {
		t.Fatalf("WrapLine URL = %q, want %q", got[0], input)
	}
}

func TestWrapLine_ExactLimit(t *testing.T) {
	t.Parallel()

	// Build a string of exactly 78 characters.
	input := strings.Repeat("a", 78) // lint:allow_raw_number
	got := WrapLine(input, 78)       // lint:allow_raw_number

	if len(got) != 1 {
		t.Fatalf("expected 1 element for exact limit, got %d", len(got))
	}

	if got[0] != input {
		t.Fatalf("WrapLine exact = %q, want %q", got[0], input)
	}
}

func TestWrapLine_FillsOptimally(t *testing.T) {
	t.Parallel()

	// "aaaa bbbb cccc" = 14 chars. With limit=10, "aaaa bbbb"
	// fits (9 chars) but "aaaa bbbb cccc" (14) does not.
	// Optimal fill: line 1 = "aaaa bbbb", line 2 = "cccc".
	input := "aaaa bbbb cccc"
	got := WrapLine(input, 10) // lint:allow_raw_number

	if len(got) != 2 { // lint:allow_raw_number
		t.Fatalf("expected 2 lines, got %d: %v", len(got), got)
	}

	if got[0] != "aaaa bbbb" {
		t.Fatalf("line 0 = %q, want %q", got[0], "aaaa bbbb")
	}

	if got[1] != "cccc" {
		t.Fatalf("line 1 = %q, want %q", got[1], "cccc")
	}
}

func TestWrapLine_SingleLongWord(t *testing.T) {
	t.Parallel()

	// A single word longer than the limit can't be split.
	input := strings.Repeat("x", 100) // lint:allow_raw_number
	got := WrapLine(input, 78)        // lint:allow_raw_number

	if len(got) != 1 {
		t.Fatalf("expected 1 line for single long word, got %d", len(got))
	}

	if got[0] != input {
		t.Fatalf("WrapLine single word = %q, want %q", got[0], input)
	}
}
