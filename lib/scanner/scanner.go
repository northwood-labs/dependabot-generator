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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/goreleaser/fileglob"
	"gopkg.in/yaml.v3"
)

const (
	// defaultEcosystemKey is the fallback key used when looking up
	// per-ecosystem settings and no specific override exists for the
	// matched ecosystem.
	defaultEcosystemKey = "_default"
)

var (
	// ErrRootNotExist is a sentinel callers can match with [errors.Is] to
	// distinguish "path doesn't exist" from "path exists but isn't a directory"
	// or "path exists but is unreadable." This separation lets the CLI present
	// targeted guidance to the user.
	ErrRootNotExist = errors.New("root path does not exist")

	// ErrRootNotDir lets callers detect the specific case where a file path was
	// provided instead of a directory, enabling a more helpful error message
	// than a generic "cannot scan.".
	ErrRootNotDir = errors.New("root path is not a directory")

	// ErrRootNotReadable covers permission-denied and other OS-level access
	// failures that are distinct from "path missing" — the user knows the path
	// exists but needs to fix permissions.
	ErrRootNotReadable = errors.New("root path is not accessible")

	// ErrGlobEval wraps unexpected failures from the fileglob library so
	// callers can distinguish infrastructure errors (broken glob engine) from
	// expected "no match" results.
	ErrGlobEval = errors.New("glob evaluation failed")

	// ErrYAMLMarshal wraps marshaling failures so callers can distinguish
	// serialization bugs from scanning errors.
	ErrYAMLMarshal = errors.New("failed to marshal YAML")

	// precedenceRules is a data-driven table rather than inline if-else chains
	// so that adding new precedence relationships requires only a single line
	// change. Each rule encodes a real-world tool relationship: bun wraps npm,
	// uv wraps pip, and opentofu is a drop-in fork of terraform — when the more
	// specific tool is detected, the generic one is noise.
	precedenceRules = []precedenceRule{
		{Winner: "bun", Loser: "npm"},
		{Winner: "opentofu", Loser: "terraform"},
		{Winner: "uv", Loser: "pip"},
	}
)

type (
	// EcosystemRule uses an OR-of-AND encoding because ecosystem detection
	// often requires expressing "file A exists" OR "files B AND C both exist."
	// The outer slice provides OR-alternatives (any one group matching is
	// sufficient), while each inner slice is an AND-group (all patterns in the
	// group must match). This two-level structure avoids needing a custom
	// expression language while covering every real Dependabot detection
	// pattern.
	EcosystemRule struct {
		Identifier string
		// Files contains OR-groups. Each inner slice is an AND-group: all
		// patterns in an AND-group must match for that group to succeed. The
		// rule matches if ANY AND-group succeeds.
		Files [][]string
	}

	// ScanResult carries the minimum information that Dependabot needs per
	// entry: a relative directory path and an ecosystem identifier. Keeping it
	// minimal means the Generate function can map results directly to YAML
	// entries without transformation or filtering.
	ScanResult struct {
		Directory string
		Ecosystem string
	}

	// precedenceRule is a named type rather than an anonymous struct so that
	// the precedence table reads like declarative data and the resolution logic
	// can range over it generically. Adding a new relationship never requires
	// changing resolution code.
	precedenceRule struct {
		Winner string
		Loser  string
	}

	// GenerateOptions holds all inputs for YAML generation beyond
	// the scan results themselves.
	GenerateOptions struct {
		EcosystemDefaults map[string]EcosystemSettings
		CommentText       string
	}

	// EcosystemSettings holds additional Dependabot v2 fields for a
	// specific ecosystem within the Generate context. The Fields map
	// is keyed by dotted path (e.g., "schedule.interval").
	EcosystemSettings struct {
		Fields map[string]any
	}

	// dependabotUpdate represents a single entry in the Dependabot updates
	// array. ExtraFields holds per-ecosystem configuration values that
	// get merged into the YAML output after directory.
	dependabotUpdate struct {
		ExtraFields      map[string]any `yaml:"-"`
		PackageEcosystem string         `yaml:"package-ecosystem"`
		Directory        string         `yaml:"directory"`
	}
)

// Scan is the primary entry point for ecosystem detection. It discovers which
// package managers and build tools are present across an entire repository
// tree, producing a flat list of results that the generator can convert
// directly into a Dependabot configuration file. The caller provides a
// repository root and an optional set of directory ignore patterns, and Scan
// handles the recursive walk, pattern matching, and precedence resolution
// internally so that consumers don't need to know about the rule table or
// its evaluation semantics.
//
// The ignoreDirs parameter accepts glob patterns matched against each
// directory's base name using [filepath.Match] semantics. Passing nil or an
// empty slice preserves the original behavior (no directories excluded).
func Scan(path string, ignoreDirs []string) ([]ScanResult, error) {
	// Multi-step validation: Stat catches non-existence, IsDir catches "user
	// passed a file," and Open/Close catches permission-denied. Each step
	// produces a different sentinel so the CLI can give specific guidance.
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrRootNotExist, path)
		}

		return nil, fmt.Errorf("%w: %s: %w", ErrRootNotReadable, path, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrRootNotDir, path)
	}

	// Open/Close proves we can actually read directory entries, catching cases
	// where Stat succeeds (metadata is readable) but listing contents would
	// fail.
	dir, openErr := os.Open(path) // lint:allow_possible_insecure
	if openErr != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrRootNotReadable, path, openErr)
	}

	closeErr := dir.Close()
	if closeErr != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrRootNotReadable, path, closeErr)
	}

	// Resolve to absolute before walking so that all glob evaluations use
	// fully-qualified paths — fileglob requires absolute patterns to function
	// correctly.
	absRoot, absErr := filepath.Abs(path)
	if absErr != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrRootNotReadable, path, absErr)
	}

	var results []ScanResult

	// [os.DirFS] + [fs.WalkDir] is preferred over [filepath.Walk] because it
	// provides a rooted filesystem abstraction, produces consistent
	// forward-slash relative paths, and handles symlinks more predictably
	// across platforms.
	walkErr := fs.WalkDir(os.DirFS(absRoot), ".", func(relPath string, d fs.DirEntry, walkDirErr error) error {
		if walkDirErr != nil {
			return walkDirErr
		}

		if !d.IsDir() {
			return nil
		}

		// Check if this directory should be excluded based on the
		// caller-provided ignore patterns. The root directory itself
		// (relPath == ".") is never excluded because skipping it would
		// abort the entire walk.
		if len(ignoreDirs) > 0 && relPath != "." {
			dirName := filepath.Base(relPath)

			for _, pattern := range ignoreDirs {
				matched, matchErr := filepath.Match(pattern, dirName)
				if matchErr != nil {
					return fmt.Errorf(
						"invalid ignore pattern %q: %w",
						pattern, matchErr,
					)
				}

				if matched {
					return fs.SkipDir
				}
			}
		}

		fullDir := filepath.Join(absRoot, relPath)

		for i := range ecosystemRules {
			matched, evalErr := evaluateRule(fullDir, ecosystemRules[i])
			if evalErr != nil {
				return fmt.Errorf("%w: %w", ErrGlobEval, evalErr)
			}

			if !matched {
				continue
			}

			dirField := toRelativeDir(relPath)

			results = append(results, ScanResult{
				Directory: dirField,
				Ecosystem: ecosystemRules[i].Identifier,
			})
		}

		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walking directory tree: %w", walkErr)
	}

	// Precedence is resolved as a post-processing step rather than inline
	// during the walk because a winner and its loser may be discovered in
	// either order depending on rule-table ordering. Post-processing guarantees
	// correctness regardless of discovery order.
	results = resolvePrecedence(results)

	return results, nil
}

// Generate converts scan results into a Dependabot v2 YAML configuration
// string. It handles sorting and formatting so that the output is
// deterministic — users commit this file and review diffs in PRs, so
// identical inputs must always produce identical output regardless of the
// order results were discovered during the walk.
//
// When opts is nil or opts.CommentText is empty, the output is identical
// to the previous behavior (no header comment). When CommentText is
// non-empty, FormatComment is called and the result is inserted after
// the `---` separator with a blank line before the YAML body.
func Generate(results []ScanResult, opts *GenerateOptions) (string, error) {
	// Copy before sorting to avoid mutating the caller's slice, which may still
	// be needed for logging or further processing.
	sorted := make([]ScanResult, len(results))
	copy(sorted, results)

	// Sort by directory first, then ecosystem within each directory. This gives
	// users a predictable file layout: entries for the same directory are
	// grouped together, and within a directory the ecosystems are alphabetical.
	slices.SortFunc(sorted, func(a, b ScanResult) int {
		dirCmp := strings.Compare(a.Directory, b.Directory)
		if dirCmp != 0 {
			return dirCmp
		}

		return strings.Compare(a.Ecosystem, b.Ecosystem)
	})

	updates := make([]dependabotUpdate, len(sorted))

	for i, r := range sorted {
		updates[i] = dependabotUpdate{
			PackageEcosystem: r.Ecosystem,
			Directory:        r.Directory,
		}
	}

	// Resolve per-ecosystem extra fields from EcosystemDefaults when
	// provided. Each update entry looks up its ecosystem; if no specific
	// override exists, falls back to the _default key.
	if opts != nil && opts.EcosystemDefaults != nil {
		for i := range updates {
			eco := updates[i].PackageEcosystem

			settings, ok := opts.EcosystemDefaults[eco]
			if !ok {
				settings, ok = opts.EcosystemDefaults[defaultEcosystemKey]
			}

			if ok && settings.Fields != nil {
				updates[i].ExtraFields = settings.Fields
			}
		}
	}

	// Build YAML using Node tree for deterministic key ordering.
	doc := buildYAMLDocument(updates)

	var buf strings.Builder

	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)

	marshalErr := encoder.Encode(doc)
	if marshalErr != nil {
		return "", fmt.Errorf("%w: %w", ErrYAMLMarshal, marshalErr)
	}

	// [yaml.Encoder] does not emit the YAML document separator, but
	// Dependabot expects it and users expect a well-formed YAML document,
	// so we prepend it manually.
	output := "---\n"

	if opts != nil && opts.CommentText != "" {
		comment := FormatComment(opts.CommentText)
		if comment != "" {
			output += comment + "\n\n"
		}
	}

	output += buf.String()

	return output, nil
}

// evaluateRule exists to decouple the "does this directory match?" question
// from the walk logic. By isolating pattern evaluation here, Scan stays
// readable and the OR-of-AND matching semantics are testable in isolation
// without needing a real filesystem tree.
func evaluateRule(dirPath string, rule EcosystemRule) (bool, error) {
	// Outer loop = OR: any single AND-group succeeding means the ecosystem is
	// present. We return true on the first match because additional matches are
	// redundant.
	for _, andGroup := range rule.Files {
		allMatch := true

		// Inner loop = AND: every pattern in the group must find at least one
		// file. A single miss disqualifies the group.
		for _, pattern := range andGroup {
			fullPattern := filepath.Join(dirPath, pattern)

			matches, globErr := fileglob.Glob(fullPattern, fileglob.MaybeRootFS)
			if globErr != nil {
				// [fs.ErrNotExist] from a non-dynamic pattern simply means the
				// file isn't there — this is a normal "no match" condition, not
				// an infrastructure error.
				if errors.Is(globErr, fs.ErrNotExist) {
					allMatch = false

					break
				}

				return false, fmt.Errorf("%w: %s: %w", ErrGlobEval, dirPath, globErr)
			}

			// fileglob recurses into directories when the pattern path resolves
			// to a directory name. Filter matches to only include entries at
			// the expected path depth, preventing false positives from
			// subdirectory traversal. Patterns containing "**" are exempt
			// because they intentionally match at any depth.
			matches = filterMatchesByDepth(dirPath, pattern, matches)

			if len(matches) == 0 {
				allMatch = false

				break
			}
		}

		if allMatch {
			return true, nil
		}
	}

	return false, nil
}

// resolvePrecedence uses a two-pass approach to suppress generic ecosystems
// when a more specific tool is detected in the same directory. The first pass
// builds a set of directories where each winner appears; the second pass
// filters out losers from those directories. Two passes are more maintainable
// than inline checks during the walk because a winner and its loser may be
// discovered in either order depending on filesystem layout and rule ordering.
func resolvePrecedence(results []ScanResult) []ScanResult {
	// First pass: record which directories contain each winner.
	winnerDirs := make(map[string]map[string]bool)

	for _, rule := range precedenceRules {
		winnerDirs[rule.Winner] = make(map[string]bool)
	}

	for _, r := range results {
		if dirs, ok := winnerDirs[r.Ecosystem]; ok {
			dirs[r.Directory] = true
		}
	}

	// Second pass: keep every result except losers whose winner is present in
	// the same directory.
	filtered := make([]ScanResult, 0, len(results))

	for _, r := range results {
		suppress := false

		for _, rule := range precedenceRules {
			if r.Ecosystem == rule.Loser && winnerDirs[rule.Winner][r.Directory] {
				suppress = true

				break
			}
		}

		if !suppress {
			filtered = append(filtered, r)
		}
	}

	return filtered
}

// toRelativeDir normalizes a WalkDir-relative path into the "/"-prefixed format
// that Dependabot expects (e.g., "/" for root, "/subdir/nested" for deeper
// paths). This normalization is necessary because WalkDir uses "." for the root
// and OS-native separators, while Dependabot's YAML schema requires Unix-style
// paths starting with "/".
func toRelativeDir(relPath string) string {
	if relPath == "." {
		return "/"
	}

	return "/" + filepath.ToSlash(relPath)
}

// countPathSegments returns the number of path components in a "/"-normalized
// path. This is used to verify that glob matches are at the expected directory
// depth relative to the pattern, preventing false positives when fileglob
// recurses into subdirectories whose names happen to match a non-glob pattern.
func countPathSegments(p string) int {
	p = filepath.ToSlash(p)

	return len(strings.Split(p, "/"))
}

// filterMatchesByDepth removes glob matches that are deeper than the pattern
// implies. When fileglob encounters a pattern that resolves to a directory
// (e.g., "rust-toolchain" where a directory of that name exists), it recurses
// into the directory and returns files from within. This function retains only
// matches whose relative path from dirPath has the same segment count as the
// pattern itself. Patterns containing "**" are exempt because recursive
// matching is intentional.
func filterMatchesByDepth(dirPath, pattern string, matches []string) []string {
	if strings.Contains(pattern, "**") {
		return matches
	}

	patternDepth := countPathSegments(pattern)
	filtered := matches[:0]

	for _, m := range matches {
		rel, relErr := filepath.Rel(dirPath, m)
		if relErr != nil {
			continue
		}

		if countPathSegments(rel) == patternDepth {
			filtered = append(filtered, m)
		}
	}

	return filtered
}

// expandDottedKeys converts a flat map with dotted keys into a nested map
// structure suitable for YAML marshaling. Each period in a key becomes a level
// of nesting: "schedule.interval" with value "monthly" becomes
// map["schedule"]map["interval"]"monthly".
func expandDottedKeys(fields map[string]any) map[string]any {
	result := make(map[string]any)

	for key, value := range fields {
		parts := strings.Split(key, ".")
		current := result

		for i, part := range parts {
			if i == len(parts)-1 {
				current[part] = value
			} else {
				next, exists := current[part]
				if !exists {
					nested := make(map[string]any)

					current[part] = nested
					current = nested
				} else {
					nested, ok := next.(map[string]any)
					if !ok {
						nested = make(map[string]any)
						current[part] = nested
					}

					current = nested
				}
			}
		}
	}

	return result
}

// buildYAMLDocument constructs a yaml.Node tree representing the full
// Dependabot v2 configuration with deterministic key ordering. Using Node
// construction rather than struct marshaling allows inserting arbitrary extra
// fields per update entry in a controlled order.
func buildYAMLDocument(updates []dependabotUpdate) *yaml.Node {
	// Root mapping: version + updates.
	rootMapping := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}

	// version: 2
	rootMapping.Content = append(rootMapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "version", Tag: "!!str"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: "2", Tag: "!!int"},
	)

	// updates: [...]
	updatesSeq := &yaml.Node{
		Kind: yaml.SequenceNode,
		Tag:  "!!seq",
	}

	for i := range updates {
		entryNode := buildUpdateNode(&updates[i])

		updatesSeq.Content = append(updatesSeq.Content, entryNode)
	}

	rootMapping.Content = append(rootMapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "updates", Tag: "!!str"},
		updatesSeq,
	)

	return rootMapping
}

// buildUpdateNode constructs a yaml.Node mapping for a single update entry.
// Keys are emitted in order: package-ecosystem, directory, then extra fields
// sorted alphabetically by top-level key.
func buildUpdateNode(u *dependabotUpdate) *yaml.Node {
	mapping := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}

	// package-ecosystem.
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "package-ecosystem", Tag: "!!str"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: u.PackageEcosystem, Tag: "!!str"},
	)

	// directory.
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "directory", Tag: "!!str"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: u.Directory, Tag: "!!str"},
	)

	// Extra fields (expanded from dotted keys, sorted alphabetically).
	if u.ExtraFields != nil {
		expanded := expandDottedKeys(u.ExtraFields)
		keys := make([]string, 0, len(expanded))

		for k := range expanded {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for _, k := range keys {
			mapping.Content = append(mapping.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: k, Tag: "!!str"},
				valueToNode(expanded[k]),
			)
		}
	}

	return mapping
}

// valueToNode recursively converts an arbitrary Go value into a yaml.Node tree.
// It handles maps (sorted keys), slices, strings, ints, floats, and booleans —
// covering all types that appear in ecosystem configuration fields.
func valueToNode(v any) *yaml.Node {
	switch val := v.(type) {
	case map[string]any:
		return mapToNode(val)
	case []any:
		return sliceToNode(val)
	case []string:
		return stringSliceToNode(val)
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: val, Tag: "!!str"}
	case int:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: strconv.Itoa(val),
			Tag:   "!!int",
		}
	case int64:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: strconv.FormatInt(val, 10),
			Tag:   "!!int",
		}
	case float64:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: fmt.Sprintf("%g", val),
			Tag:   "!!float",
		}
	case bool:
		boolStr := "false"
		if val {
			boolStr = "true"
		}

		return &yaml.Node{Kind: yaml.ScalarNode, Value: boolStr, Tag: "!!bool"}
	default:
		// Fallback: convert to string representation.
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: fmt.Sprintf("%v", val),
			Tag:   "!!str",
		}
	}
}

// mapToNode converts a map[string]any to a yaml.Node mapping with keys sorted
// alphabetically for deterministic output.
func mapToNode(m map[string]any) *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}

	keys := make([]string, 0, len(m))

	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: k, Tag: "!!str"},
			valueToNode(m[k]),
		)
	}

	return node
}

// sliceToNode converts a []any slice to a yaml.Node sequence.
func sliceToNode(s []any) *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.SequenceNode,
		Tag:  "!!seq",
	}

	for _, item := range s {
		node.Content = append(node.Content, valueToNode(item))
	}

	return node
}

// stringSliceToNode converts a []string to a yaml.Node sequence. This handles
// the common case where TOML arrays of strings are stored as []string rather
// than []any.
func stringSliceToNode(s []string) *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.SequenceNode,
		Tag:  "!!seq",
	}

	for _, item := range s {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: item, Tag: "!!str"},
		)
	}

	return node
}
