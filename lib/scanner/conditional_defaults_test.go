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

// Unit tests for the conditional defaults mechanism verify table validation,
// field resolution merge precedence, and non-interference with non-matching
// ecosystems.

package scanner // lint:allow_naming_conflict_stdlib

import (
	"errors"
	"maps"
	"testing"

	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

// TestConditionalDefaults_ValidateProductionTable confirms the compiled-in
// conditionalDefaults table passes all validation rules, catching programming
// mistakes early.
func TestConditionalDefaults_ValidateProductionTable(t *testing.T) {
	t.Parallel()

	err := ValidateConditionalDefaults()
	if err != nil {
		t.Fatalf("expected production table to pass validation, got: %v", err)
	}
}

// TestConditionalDefaults_Validate_RejectsDefaultEcosystem confirms that
// validation rejects entries using the reserved "_default" ecosystem
// identifier.
func TestConditionalDefaults_Validate_RejectsDefaultEcosystem(t *testing.T) {
	t.Parallel()

	orig := conditionalDefaults

	conditionalDefaults = []ConditionalDefault{
		{
			FieldKey:   "schedule.interval",
			FieldValue: "weekly",
			Ecosystems: []string{"bundler", "_default"},
		},
	}

	defer func() { conditionalDefaults = orig }()

	err := ValidateConditionalDefaults()
	if err == nil {
		t.Fatal("expected error for _default ecosystem, got nil")
	}

	if !errors.Is(err, ErrConditionalDefaultInvalid) {
		t.Fatalf(
			"expected ErrConditionalDefaultInvalid, got: %v", err,
		)
	}
}

// TestConditionalDefaults_Validate_RejectsEmptyEcosystems confirms that
// validation rejects entries with an empty Ecosystems slice.
func TestConditionalDefaults_Validate_RejectsEmptyEcosystems(t *testing.T) {
	t.Parallel()

	orig := conditionalDefaults

	conditionalDefaults = []ConditionalDefault{
		{
			FieldKey:   "schedule.interval",
			FieldValue: "weekly",
			Ecosystems: nil,
		},
	}

	defer func() { conditionalDefaults = orig }()

	err := ValidateConditionalDefaults()
	if err == nil {
		t.Fatal("expected error for empty ecosystems, got nil")
	}

	if !errors.Is(err, ErrConditionalDefaultInvalid) {
		t.Fatalf(
			"expected ErrConditionalDefaultInvalid, got: %v", err,
		)
	}
}

// TestConditionalDefaults_Validate_RejectsEmptyFieldKey confirms that
// validation rejects entries with an empty FieldKey.
func TestConditionalDefaults_Validate_RejectsEmptyFieldKey(t *testing.T) {
	t.Parallel()

	orig := conditionalDefaults

	conditionalDefaults = []ConditionalDefault{
		{
			FieldKey:   "",
			FieldValue: "deny",
			Ecosystems: []string{"bundler"},
		},
	}

	defer func() { conditionalDefaults = orig }()

	err := ValidateConditionalDefaults()
	if err == nil {
		t.Fatal("expected error for empty field key, got nil")
	}

	if !errors.Is(err, ErrConditionalDefaultInvalid) {
		t.Fatalf(
			"expected ErrConditionalDefaultInvalid, got: %v", err,
		)
	}
}

// TestConditionalDefaults_ResolveFields_BundlerGetsInsecureDeny confirms that
// bundler receives the insecure-external-code-execution: deny field from the
// conditional defaults table.
func TestConditionalDefaults_ResolveFields_BundlerGetsInsecureDeny(t *testing.T) {
	t.Parallel()

	ecoDefaults := map[string]EcosystemSettings{
		"_default": {
			Fields: map[string]any{
				"schedule.interval": "monthly",
			},
		},
	}

	result := resolveFields("bundler", ecoDefaults)

	val, ok := result["insecure-external-code-execution"]
	if !ok {
		t.Fatal("expected insecure-external-code-execution in result")
	}

	if val != "deny" {
		t.Fatalf("expected \"deny\", got: %v", val)
	}
}

// TestConditionalDefaults_ResolveFields_MixGetsInsecureDeny confirms that mix
// receives the insecure-external-code-execution: deny field from the
// conditional defaults table.
func TestConditionalDefaults_ResolveFields_MixGetsInsecureDeny(t *testing.T) {
	t.Parallel()

	ecoDefaults := map[string]EcosystemSettings{
		"_default": {
			Fields: map[string]any{
				"schedule.interval": "monthly",
			},
		},
	}

	result := resolveFields("mix", ecoDefaults)

	val, ok := result["insecure-external-code-execution"]
	if !ok {
		t.Fatal("expected insecure-external-code-execution in result")
	}

	if val != "deny" {
		t.Fatalf("expected \"deny\", got: %v", val)
	}
}

// TestConditionalDefaults_ResolveFields_PipGetsInsecureDeny confirms that pip
// receives the insecure-external-code-execution: deny field from the
// conditional defaults table.
func TestConditionalDefaults_ResolveFields_PipGetsInsecureDeny(t *testing.T) {
	t.Parallel()

	ecoDefaults := map[string]EcosystemSettings{
		"_default": {
			Fields: map[string]any{
				"schedule.interval": "monthly",
			},
		},
	}

	result := resolveFields("pip", ecoDefaults)

	val, ok := result["insecure-external-code-execution"]
	if !ok {
		t.Fatal("expected insecure-external-code-execution in result")
	}

	if val != "deny" {
		t.Fatalf("expected \"deny\", got: %v", val)
	}
}

// TestConditionalDefaults_ResolveFields_GomodNoInsecureField confirms
// that gomod does NOT receive the insecure-external-code-execution field
// since it is not listed in the conditional defaults table.
func TestConditionalDefaults_ResolveFields_GomodNoInsecureField(t *testing.T) {
	t.Parallel()

	ecoDefaults := map[string]EcosystemSettings{
		"_default": {
			Fields: map[string]any{
				"schedule.interval": "monthly",
			},
		},
	}

	result := resolveFields("gomod", ecoDefaults)

	_, ok := result["insecure-external-code-execution"]
	if ok {
		t.Fatal("gomod should NOT have insecure-external-code-execution")
	}
}

// TestConditionalDefaults_ResolveFields_UserOverrideWins confirms that a
// user-configured ecosystem override for bundler takes precedence over the
// conditional default value.
func TestConditionalDefaults_ResolveFields_UserOverrideWins(t *testing.T) {
	t.Parallel()

	ecoDefaults := map[string]EcosystemSettings{
		"_default": {
			Fields: map[string]any{
				"schedule.interval": "monthly",
			},
		},
		"bundler": {
			Fields: map[string]any{
				"insecure-external-code-execution": "allow",
			},
		},
	}

	result := resolveFields("bundler", ecoDefaults)

	val, ok := result["insecure-external-code-execution"]
	if !ok {
		t.Fatal("expected insecure-external-code-execution in result")
	}

	if val != "allow" {
		t.Fatalf("expected user override \"allow\", got: %v", val)
	}
}

// TestConditionalDefaults_ResolveFields_EmptyTableIdentity confirms that when
// the conditionalDefaults table is empty, resolveFields produces output
// identical to the pre-feature behavior (only _default fields).
func TestConditionalDefaults_ResolveFields_EmptyTableIdentity(t *testing.T) {
	t.Parallel()

	orig := conditionalDefaults

	conditionalDefaults = nil

	defer func() {
		conditionalDefaults = orig
	}()

	ecoDefaults := map[string]EcosystemSettings{
		"_default": {
			Fields: map[string]any{
				"schedule.interval": "monthly",
			},
		},
	}

	result := resolveFields("bundler", ecoDefaults)
	if len(result) != 1 {
		t.Fatalf("expected 1 field, got: %d", len(result))
	}

	val, ok := result["schedule.interval"]
	if !ok {
		t.Fatal("expected schedule.interval in result")
	}

	if val != "monthly" {
		t.Fatalf("expected \"monthly\", got: %v", val)
	}

	_, hasInsecure := result["insecure-external-code-execution"]
	if hasInsecure {
		t.Fatal("empty table should not inject insecure-external-code-execution")
	}
}

// TestConditionalDefaults_ResolveFields_DeadEcosystemNoEffect confirms that a
// conditional default entry listing an ecosystem identifier that does not match
// the current ecosystem has no effect on the output.
func TestConditionalDefaults_ResolveFields_DeadEcosystemNoEffect(t *testing.T) {
	t.Parallel()

	orig := conditionalDefaults

	conditionalDefaults = []ConditionalDefault{
		{
			FieldKey:   "some-field",
			FieldValue: "some-value",
			Ecosystems: []string{"nonexistent-ecosystem"},
		},
	}

	defer func() {
		conditionalDefaults = orig
	}()

	ecoDefaults := map[string]EcosystemSettings{
		"_default": {
			Fields: map[string]any{
				"schedule.interval": "monthly",
			},
		},
	}

	result := resolveFields("gomod", ecoDefaults)
	if len(result) != 1 {
		t.Fatalf("expected 1 field, got: %d", len(result))
	}

	val, ok := result["schedule.interval"]
	if !ok {
		t.Fatal("expected schedule.interval in result")
	}

	if val != "monthly" {
		t.Fatalf("expected \"monthly\", got: %v", val)
	}

	_, hasSomeField := result["some-field"]
	if hasSomeField {
		t.Fatal("dead ecosystem should not inject some-field into gomod")
	}
}

// -----------------------------------------------------------------------------
// PROPERTY-BASED TESTS.

// TestProperty_MergePrecedence verifies that the highest-priority layer always
// wins when the same field key is defined at multiple layers.
//
// Feature: conditional-ecosystem-defaults, Property 1: Merge precedence.
func TestProperty_MergePrecedence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		fieldKey := rapid.StringMatching("[a-z][a-z0-9.\\-]{0,19}").Draw(t, "fieldKey")
		defaultVal := rapid.String().Draw(t, "defaultVal")
		conditionalVal := rapid.String().Draw(t, "conditionalVal")
		overrideVal := rapid.String().Draw(t, "overrideVal")
		eco := rapid.StringMatching("[a-z][a-z0-9-]{0,9}").Draw(t, "ecosystem")

		// Save and restore global state.
		orig := conditionalDefaults
		defer func() {
			conditionalDefaults = orig
		}()

		conditionalDefaults = []ConditionalDefault{
			{
				FieldKey:   fieldKey,
				FieldValue: conditionalVal,
				Ecosystems: []string{eco},
			},
		}

		// Case 1: All 3 layers present — ecosystem override wins.
		ecoDefaults := map[string]EcosystemSettings{
			defaultEcosystemKey: {
				Fields: map[string]any{
					fieldKey: defaultVal,
				},
			},
			eco: {
				Fields: map[string]any{
					fieldKey: overrideVal,
				},
			},
		}

		result := resolveFields(eco, ecoDefaults)
		if result[fieldKey] != overrideVal {
			t.Fatalf("expected override %q, got %q", overrideVal, result[fieldKey])
		}

		// Case 2: Only _default + conditional — conditional wins.
		ecoDefaultsNoOverride := map[string]EcosystemSettings{
			defaultEcosystemKey: {
				Fields: map[string]any{
					fieldKey: defaultVal,
				},
			},
		}

		result2 := resolveFields(eco, ecoDefaultsNoOverride)
		if result2[fieldKey] != conditionalVal {
			t.Fatalf("expected conditional %q, got %q", conditionalVal, result2[fieldKey])
		}
	})
}

// TestProperty_ConditionalApplication verifies that a conditional default field
// appears in the output for ecosystems listed in the entry.
//
// Feature: conditional-ecosystem-defaults, Property 2: Conditional application.
func TestProperty_ConditionalApplication(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		fieldKey := rapid.StringMatching("[a-z][a-z0-9.\\-]{0,19}").Draw(t, "fieldKey")
		fieldValue := rapid.String().Draw(t, "fieldValue")
		numEcos := rapid.IntRange(1, 5).Draw(t, "numEcos")
		ecosystems := make([]string, numEcos)

		for i := range numEcos {
			ecosystems[i] = rapid.StringMatching("[a-z][a-z0-9-]{0,9}").Draw(t, "eco")
		}

		// Pick one ecosystem from the list to test.
		targetEco := ecosystems[rapid.IntRange(0, len(ecosystems)-1).Draw(t, "targetIdx")]

		// Save and restore global state.
		orig := conditionalDefaults
		defer func() {
			conditionalDefaults = orig
		}()

		conditionalDefaults = []ConditionalDefault{
			{
				FieldKey:   fieldKey,
				FieldValue: fieldValue,
				Ecosystems: ecosystems,
			},
		}

		// No user override for the field key.
		ecoDefaults := map[string]EcosystemSettings{
			defaultEcosystemKey: {
				Fields: map[string]any{
					"other-field": "x",
				},
			},
		}

		result := resolveFields(targetEco, ecoDefaults)

		val, ok := result[fieldKey]
		if !ok {
			t.Fatalf("expected field %q in result for eco %q", fieldKey, targetEco)
		}

		if val != fieldValue {
			t.Fatalf("expected value %q, got %q", fieldValue, val)
		}
	})
}

// TestProperty_NonInterference verifies that ecosystems not in any conditional
// default entry produce identical output with or without the conditional
// defaults table populated.
//
// Feature: conditional-ecosystem-defaults, Property 3: Non-interference for
// non-matching ecosystems.
func TestProperty_NonInterference(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate an ecosystem that won't match any conditional entry.
		eco := "zz-nomatch-" + rapid.StringMatching("[a-z]{1,5}").Draw(t, "suffix")

		// Generate random _default fields.
		numFields := rapid.IntRange(1, 5).Draw(t, "numFields")
		defaultFields := make(map[string]any, numFields)

		for i := range numFields {
			k := rapid.StringMatching("[a-z][a-z0-9]{0,9}").Draw(t, "key")

			defaultFields[k] = rapid.String().Draw(t, "val")
			_ = i
		}

		ecoDefaults := map[string]EcosystemSettings{
			defaultEcosystemKey: {
				Fields: defaultFields,
			},
		}

		// Save and restore global state.
		orig := conditionalDefaults
		defer func() {
			conditionalDefaults = orig
		}()

		// With conditional defaults (non-matching eco).
		conditionalDefaults = []ConditionalDefault{
			{
				FieldKey:   "some-conditional-field",
				FieldValue: "some-value",
				Ecosystems: []string{"xxx-never-match"},
			},
		}

		resultA := resolveFields(eco, ecoDefaults)

		// Without conditional defaults.
		conditionalDefaults = nil

		resultB := resolveFields(eco, ecoDefaults)

		// Both should be identical.
		if len(resultA) != len(resultB) {
			t.Fatalf("length mismatch: %d vs %d", len(resultA), len(resultB))
		}

		for k, va := range resultA {
			vb, ok := resultB[k]
			if !ok {
				t.Fatalf("key %q missing in resultB", k)
			}

			if va != vb {
				t.Fatalf("key %q: %v != %v", k, va, vb)
			}
		}
	})
}

// TestProperty_FieldPreservation verifies that fields from lower-priority
// sources whose keys are not present in higher layers appear unchanged in the
// merged result.
//
// Feature: conditional-ecosystem-defaults, Property 4: Field preservation from
// lower-priority sources.
func TestProperty_FieldPreservation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		eco := rapid.StringMatching("[a-z][a-z0-9-]{0,9}").Draw(t, "ecosystem")

		// Generate unique _default field keys that won't collide with
		// conditional or override keys.
		numFields := rapid.IntRange(1, 5).Draw(t, "numFields")
		defaultFields := make(map[string]any, numFields)

		for i := range numFields {
			k := "def-" + rapid.StringMatching("[a-z]{1,8}").Draw(t, "defKey")

			defaultFields[k] = rapid.String().Draw(t, "defVal")
			_ = i
		}

		// Save and restore global state.
		orig := conditionalDefaults
		defer func() {
			conditionalDefaults = orig
		}()

		// Conditional default uses a distinct key prefix.
		conditionalDefaults = []ConditionalDefault{
			{
				FieldKey:   "cond-field",
				FieldValue: "cond-value",
				Ecosystems: []string{eco},
			},
		}

		// Ecosystem override uses a distinct key prefix.
		ecoDefaults := map[string]EcosystemSettings{
			defaultEcosystemKey: {
				Fields: defaultFields,
			},
			eco: {
				Fields: map[string]any{
					"ovr-field": "ovr-value",
				},
			},
		}

		result := resolveFields(eco, ecoDefaults)

		// All _default fields with "def-" prefix should be preserved.
		for k, expected := range defaultFields {
			actual, ok := result[k]
			if !ok {
				t.Fatalf("expected _default field %q in result", k)
			}

			if actual != expected {
				t.Fatalf("field %q: expected %q, got %q", k, expected, actual)
			}
		}
	})
}

// TestProperty_EmptyTableIdentity verifies that with an empty
// conditionalDefaults slice, resolveFields produces the same output as a simple
// _default-plus-override merge.
//
// Feature: conditional-ecosystem-defaults, Property 5: Empty table identity.
func TestProperty_EmptyTableIdentity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		eco := rapid.StringMatching("[a-z][a-z0-9-]{0,9}").Draw(t, "ecosystem")

		// Generate random _default fields.
		defaultFields := drawFieldMap(t, "def")

		// Optionally generate ecosystem override fields.
		hasOverride := rapid.Bool().Draw(t, "hasOverride")
		overrideFields := make(map[string]any)

		if hasOverride {
			overrideFields = drawFieldMap(t, "ovr")
		}

		ecoDefaults := map[string]EcosystemSettings{
			defaultEcosystemKey: {
				Fields: defaultFields,
			},
		}

		if hasOverride {
			ecoDefaults[eco] = EcosystemSettings{
				Fields: overrideFields,
			}
		}

		// Save and restore global state.
		orig := conditionalDefaults
		defer func() {
			conditionalDefaults = orig
		}()

		conditionalDefaults = nil

		result := resolveFields(eco, ecoDefaults)

		// Build expected: start with _default, overlay eco override.
		expected := make(map[string]any)
		maps.Copy(expected, defaultFields)

		if hasOverride {
			maps.Copy(expected, overrideFields)
		}

		assertMapsEqual(t, result, expected)
	})
}

// TestProperty_OutputDeterminism verifies that calling Generate multiple times
// with identical inputs produces byte-for-byte identical output.
//
// Feature: conditional-ecosystem-defaults, Property 6: Output determinism.
func TestProperty_OutputDeterminism(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		numResults := rapid.IntRange(1, 10).Draw(t, "numResults")
		results := make([]ScanResult, numResults)

		for i := range numResults {
			results[i] = ScanResult{
				Directory: "/" + rapid.StringMatching("[a-z]{1,10}").Draw(t, "dir"),
				Ecosystem: rapid.StringMatching("[a-z][a-z0-9-]{0,9}").Draw(t, "eco"),
			}
		}

		// Generate random ecosystem defaults.
		defaultFields := map[string]any{
			"schedule.interval": rapid.SampledFrom(
				[]string{
					"daily",
					"weekly",
					"monthly",
				},
			).Draw(t, "interval"),
		}

		opts := &GenerateOptions{
			EcosystemDefaults: map[string]EcosystemSettings{
				defaultEcosystemKey: {
					Fields: defaultFields,
				},
			},
		}

		// Call Generate 5 times with identical inputs.
		first, firstErr := Generate(results, opts)
		if firstErr != nil {
			t.Fatalf("Generate failed: %v", firstErr)
		}

		for i := range 4 {
			output, genErr := Generate(results, opts)
			if genErr != nil {
				t.Fatalf("Generate iteration %d failed: %v", i, genErr)
			}

			if output != first {
				t.Fatalf("output mismatch on iteration %d", i+1)
			}
		}
	})
}

// TestProperty_AlphabeticalFieldOrdering verifies that extra fields in the
// generated YAML appear in strict alphabetical order by top-level key after the
// directory field.
//
// Feature: conditional-ecosystem-defaults, Property 7: Alphabetical field
// ordering.
func TestProperty_AlphabeticalFieldOrdering(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		eco := rapid.StringMatching("[a-z][a-z0-9-]{0,9}").Draw(t, "ecosystem")

		// Generate multiple random field keys.
		fields := drawFieldMap(t, "field")

		results := []ScanResult{
			{
				Directory: "/",
				Ecosystem: eco,
			},
		}

		opts := &GenerateOptions{
			EcosystemDefaults: map[string]EcosystemSettings{
				defaultEcosystemKey: {
					Fields: fields,
				},
			},
		}

		output, genErr := Generate(results, opts)
		if genErr != nil {
			t.Fatalf("Generate failed: %v", genErr)
		}

		keys := extractFieldKeysFromYAML(t, output)

		// Verify alphabetical ordering.
		for i := 1; i < len(keys); i++ {
			if keys[i] < keys[i-1] {
				t.Fatalf("keys not alphabetical: %q before %q", keys[i-1], keys[i])
			}
		}
	})
}

// drawFieldMap generates a random map of 2-5 field key-value pairs for use in
// property tests.
func drawFieldMap(t *rapid.T, prefix string) map[string]any {
	numFields := rapid.IntRange(2, 5).Draw(t, prefix+"Num")
	fields := make(map[string]any, numFields)

	for i := range numFields {
		k := rapid.StringMatching("[a-z][a-z0-9]{0,9}").Draw(t, prefix+"Key")

		fields[k] = rapid.String().Draw(t, prefix+"Val")
		_ = i
	}

	return fields
}

// assertMapsEqual fails the test if result and expected differ in length or any
// key-value pair.
func assertMapsEqual(t *rapid.T, result, expected map[string]any) {
	if len(result) != len(expected) {
		t.Fatalf("length mismatch: got %d, want %d", len(result), len(expected))
	}

	for k, want := range expected {
		got, ok := result[k]
		if !ok {
			t.Fatalf("expected key %q in result", k)
		}

		if got != want {
			t.Fatalf("key %q: got %q, want %q", k, got, want)
		}
	}
}

// extractFieldKeysFromYAML parses Generate output and returns the top-level
// keys appearing after "directory" in the first update entry.
func extractFieldKeysFromYAML(t *rapid.T, output string) []string {
	var doc yaml.Node

	unmarshalErr := yaml.Unmarshal([]byte(output), &doc)
	if unmarshalErr != nil {
		t.Fatalf("YAML unmarshal failed: %v", unmarshalErr)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		t.Fatal("unexpected YAML structure: no document")
	}

	rootMap := doc.Content[0]
	if rootMap.Kind != yaml.MappingNode {
		t.Fatal("unexpected YAML structure: root not mapping")
	}

	updatesSeq := findMappingValue(rootMap, "updates")
	if updatesSeq == nil || len(updatesSeq.Content) == 0 {
		t.Fatal("no updates found in YAML")
	}

	entryMap := updatesSeq.Content[0]
	if entryMap.Kind != yaml.MappingNode {
		t.Fatal("update entry is not a mapping")
	}

	return extractKeysAfterDirectory(entryMap)
}

// findMappingValue returns the value node for a given key in a YAML mapping
// node, or nil if not found.
func findMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}

	return nil
}

// extractKeysAfterDirectory collects YAML mapping keys that appear after the
// "directory" key in a yaml.Node mapping.
func extractKeysAfterDirectory(mapping *yaml.Node) []string {
	var keys []string

	pastDirectory := false

	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]

		if keyNode.Value == "directory" {
			pastDirectory = true

			continue
		}

		if pastDirectory {
			keys = append(keys, keyNode.Value)
		}
	}

	return keys
}
