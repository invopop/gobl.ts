package main

// defKind is the structural classification of a $defs entry. The four-way
// split is ported from gobl.generator's Ruby parser, which models GOBL's
// schema idioms more completely than the Markdown generator in gobl.docs.
type defKind int

const (
	kindObject   defKind = iota // type: object with properties -> interface
	kindMap                     // type: object with patternProperties -> Record
	kindEmptyObj                // type: object with neither -> Record<string, unknown>
	kindArray                   // type: array -> T[]
	kindEnum                    // closed or open union of consts
	kindValue                   // scalar wrapper -> string alias
)

func classify(s *JSONSchema) defKind {
	switch {
	case s.Type == "array" && s.Items != nil:
		return kindArray
	case s.Type == "object" && s.Properties != nil && s.Properties.Len() > 0:
		return kindObject
	case s.Type == "object" && s.PatternProperties != nil && s.PatternProperties.Len() > 0:
		return kindMap
	case s.Type == "object":
		return kindEmptyObj
	case s.hasEnum():
		return kindEnum
	default:
		return kindValue
	}
}

// isOptional reports whether a property should be emitted with `?`.
//
// A property is optional if it is not required, OR if it is calculated.
// Calculated properties are populated by GOBL during the build, so a caller
// constructing a document must be able to omit them even where the schema
// marks them required. gobl.docs drops `required` entirely and gobl.generator
// encodes exactly this rule; this follows gobl.generator.
func isOptional(def *JSONSchema, name string) bool {
	prop, ok := lookupProp(def, name)
	if ok && prop.Calculated {
		return true
	}
	for _, r := range def.Required {
		if r == name {
			return false
		}
	}
	return true
}

// calculatedKeys lists the properties GOBL computes, in declaration order.
func calculatedKeys(def *JSONSchema) []string {
	var out []string
	if def.Properties == nil {
		return out
	}
	for pair := def.Properties.Oldest(); pair != nil; pair = pair.Next() {
		if pair.Value.Calculated {
			out = append(out, pair.Key)
		}
	}
	return out
}

// isRecommended reports whether the definition lists name under "recommended",
// a GOBL annotation meaning "show this field by default in a UI, but do not
// require it".
func isRecommended(def *JSONSchema, name string) bool {
	for _, r := range def.Recommended {
		if r == name {
			return true
		}
	}
	return false
}

func lookupProp(def *JSONSchema, name string) (*JSONSchema, bool) {
	if def.Properties == nil {
		return nil, false
	}
	return def.Properties.Get(name)
}

// hasConditional reports whether a definition carries JSON Schema if/then
// constraints, either directly or on its composition entries. GOBL uses the
// latter form in tax.Combo, restricting "key" by "cat".
func hasConditional(def *JSONSchema) bool {
	if def.If != nil || def.Then != nil {
		return true
	}
	for _, e := range def.composition() {
		if e.If != nil || e.Then != nil {
			return true
		}
	}
	return false
}
