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
	"maps"
	"slices"
)

var (
	// ErrConditionalDefaultInvalid is a sentinel callers can match with
	// [errors.Is] to identify programming mistakes in the compiled-in
	// conditional defaults table. Validation catches these during tests, not at
	// runtime.
	ErrConditionalDefaultInvalid = errors.New(
		"invalid conditional default entry",
	)

	// conditionalDefaults is the compiled-in table of ecosystem-specific field
	// defaults. Each entry specifies a field key-value pair and the ecosystems
	// where it applies. Adding a new conditional default requires appending a
	// single struct literal to this slice.
	//
	// The table is intentionally not exported — it is consumed only by Generate
	// and validated by tests. External code interacts with conditional defaults
	// only through the generated YAML output.
	conditionalDefaults = []ConditionalDefault{
		{
			FieldKey:   "insecure-external-code-execution",
			FieldValue: "deny",
			Ecosystems: []string{"bundler", "mix", "pip"},
		},
	}
)

type (
	// ConditionalDefault defines a single field-value pair that is injected
	// into generated update entries only when the entry's ecosystem matches one
	// of the listed identifiers. This struct is the unit of the compiled-in
	// conditional defaults table.
	ConditionalDefault struct {
		// FieldKey is the dotted-path field name matching the format used in
		// EcosystemConfig.Fields (e.g., "insecure-external-code-execution",
		// "schedule.interval").
		FieldKey string

		// FieldValue is the value to inject, consistent with the existing
		// map[string]any pattern used by EcosystemSettings.Fields.
		FieldValue any

		// Ecosystems is the set of ecosystem identifiers (matching
		// EcosystemRule.Identifier values) for which this default applies. Must
		// be non-empty.
		Ecosystems []string
	}
)

// resolveFields implements the four-layer field merge for a single update
// entry. It combines builtin _default fields, conditional defaults matched by
// ecosystem, and user ecosystem-specific overrides into a single map. The merge
// precedence (highest to lowest) is:
//
//  1. User ecosystem-specific override (ecoDefaults[ecosystem])
//  2. Conditional defaults matched by ecosystem
//  3. User/builtin _default (ecoDefaults["_default"])
func resolveFields(ecosystem string, ecoDefaults map[string]EcosystemSettings) map[string]any {
	merged := make(map[string]any)

	// Layer 3 (lowest priority): _default (builtin merged with user _default by
	// the config loader).
	if defSettings, ok := ecoDefaults[defaultEcosystemKey]; ok {
		if defSettings.Fields != nil {
			maps.Copy(merged, defSettings.Fields)
		}
	}

	// Layer 2: Conditional defaults for this ecosystem.
	for i := range conditionalDefaults {
		if slices.Contains(conditionalDefaults[i].Ecosystems, ecosystem) {
			merged[conditionalDefaults[i].FieldKey] = conditionalDefaults[i].FieldValue
		}
	}

	// Layer 1 (highest priority): User ecosystem-specific override.
	if ecoSettings, ok := ecoDefaults[ecosystem]; ok {
		if ecoSettings.Fields != nil {
			maps.Copy(merged, ecoSettings.Fields)
		}
	}

	if len(merged) == 0 {
		return nil
	}

	return merged
}

// ValidateConditionalDefaults checks the compiled-in table for invalid entries.
// It rejects entries with empty ecosystem lists, empty field keys, and entries
// that use the reserved "_default" identifier. Validation is called from tests
// rather than at runtime startup to keep binary startup cost at zero.
func ValidateConditionalDefaults() error {
	for i, entry := range conditionalDefaults {
		if len(entry.Ecosystems) == 0 {
			return fmt.Errorf(
				"%w: entry %d has empty ecosystems list",
				ErrConditionalDefaultInvalid, i,
			)
		}

		if entry.FieldKey == "" {
			return fmt.Errorf(
				"%w: entry %d has empty field key",
				ErrConditionalDefaultInvalid, i,
			)
		}

		if slices.Contains(entry.Ecosystems, defaultEcosystemKey) {
			return fmt.Errorf(
				"%w: entry %d uses reserved key %q",
				ErrConditionalDefaultInvalid, i, defaultEcosystemKey,
			)
		}
	}

	return nil
}
