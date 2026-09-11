package main

import (
	"encoding/json"
	"fmt"
	"sort"

	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// propMap preserves the declaration order of JSON Schema properties, which
// mirrors the Go struct field order in GOBL and is therefore meaningful.
type propMap = orderedmap.OrderedMap[string, *JSONSchema]

// JSONSchema is a deliberately partial model of JSON Schema 2020-12 covering
// exactly the keywords GOBL emits, plus its two custom annotations
// ("calculated" and "recommended").
//
// Anything outside knownKeywords is rejected at unmarshal time rather than
// silently ignored: a partial model that quietly drops a keyword would emit
// confidently wrong types the day GOBL starts using it.
type JSONSchema struct {
	Schema      string                 `json:"$schema,omitempty"`
	ID          string                 `json:"$id,omitempty"`
	Ref         string                 `json:"$ref,omitempty"`
	Defs        map[string]*JSONSchema `json:"$defs,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`

	Properties        *propMap `json:"properties,omitempty"`
	PatternProperties *propMap `json:"patternProperties,omitempty"`
	Required          []string `json:"required,omitempty"`
	Recommended       []string `json:"recommended,omitempty"`

	Items *JSONSchema   `json:"items,omitempty"`
	OneOf []*JSONSchema `json:"oneOf,omitempty"`
	AnyOf []*JSONSchema `json:"anyOf,omitempty"`

	Const     any    `json:"const,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	Format    string `json:"format,omitempty"`
	MinLength *int   `json:"minLength,omitempty"`
	MaxLength *int   `json:"maxLength,omitempty"`

	// ContentEncoding marks binary payloads, e.g. base64 on org.Image.
	ContentEncoding string `json:"contentEncoding,omitempty"`

	Examples []json.RawMessage `json:"examples,omitempty"`

	// Calculated marks a property GOBL populates during calculation. Such
	// properties are always optional on input.
	Calculated bool `json:"calculated,omitempty"`

	If   *JSONSchema `json:"if,omitempty"`
	Then *JSONSchema `json:"then,omitempty"`

	// Enum is deliberately never read. GOBL v0.505.0 emits a single, malformed
	// occurrence in schemas/bill/delivery.json: `"enum": "advice"`, a bare
	// string where JSON Schema requires an array. It comes from the struct tag
	// `jsonschema_extras:"enum=advice,note,waybill,receipt,other"` at
	// bill/delivery.go:107, which collapses to the first item. Decoding it as a
	// slice panics. The correct values live in the sibling "oneOf".
	Enum json.RawMessage `json:"enum,omitempty"`
}

// knownKeywords is every keyword the emitter understands. Keywords listed in
// ignoredKeywords are tolerated but intentionally unused.
var knownKeywords = map[string]bool{
	"$id": true, "$ref": true, "$defs": true,
	"type": true, "title": true, "description": true,
	"properties": true, "patternProperties": true,
	"required": true, "recommended": true,
	"items": true, "oneOf": true, "anyOf": true,
	"const": true, "pattern": true, "format": true,
	"minLength": true, "maxLength": true, "contentEncoding": true,
	"examples": true, "calculated": true,
	"if": true, "then": true,
}

// ignoredKeywords are present in the data but carry no type information.
var ignoredKeywords = map[string]bool{
	"$schema": true, // dialect declaration
	"enum":    true, // malformed upstream; see JSONSchema.Enum
}

// schemaAlias avoids infinite recursion in UnmarshalJSON.
type schemaAlias JSONSchema

// UnmarshalJSON decodes the schema and fails loudly on any keyword this
// generator does not model.
func (s *JSONSchema) UnmarshalJSON(data []byte) error {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	var unknown []string
	for k := range probe {
		if !knownKeywords[k] && !ignoredKeywords[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf(
			"unsupported JSON Schema keyword(s) %v: this generator models only the subset "+
				"GOBL emits. Teach cmd/generate about them rather than ignoring them",
			unknown,
		)
	}
	return json.Unmarshal(data, (*schemaAlias)(s))
}

// composition returns the enum-style composition entries, preferring oneOf.
// GOBL never uses both on one schema.
func (s *JSONSchema) composition() []*JSONSchema {
	if len(s.OneOf) > 0 {
		return s.OneOf
	}
	return s.AnyOf
}

// constEntries returns only the composition entries carrying a "const",
// dropping any open-ended escape entry.
func (s *JSONSchema) constEntries() []*JSONSchema {
	var out []*JSONSchema
	for _, e := range s.composition() {
		if e.Const != nil {
			out = append(out, e)
		}
	}
	return out
}

// isStrictEnum reports whether every composition entry is a const, i.e. the
// union is closed. A trailing {"pattern": ..., "title": "Any"} entry makes it
// open. Escape entries cannot be recognised by title: "$tags" names its escape
// "Any" while org.Unit gives its own no title at all. Absence of "const" is the
// only reliable test.
func (s *JSONSchema) isStrictEnum() bool {
	comp := s.composition()
	if len(comp) == 0 {
		return false
	}
	for _, e := range comp {
		if e.Const == nil {
			return false
		}
	}
	return true
}

// hasEnum reports whether there is at least one const entry to build a union
// from. bill.Delivery."$tags" has a composition made up solely of the open
// escape entry, which must fall back to its $ref base type rather than produce
// an empty union.
func (s *JSONSchema) hasEnum() bool {
	return len(s.constEntries()) > 0
}

// constString renders a const value as its JSON string. Every const in GOBL is
// a string.
func (s *JSONSchema) constString() (string, error) {
	v, ok := s.Const.(string)
	if !ok {
		return "", fmt.Errorf("const value %v is not a string", s.Const)
	}
	return v, nil
}
