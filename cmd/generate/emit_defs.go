package main

import (
	"fmt"
	"sort"
	"strings"
)

// fileEmitter renders one schema document into one TypeScript file.
type fileEmitter struct {
	store *schemaStore
	file  *schemaFile
	w     *tsWriter
	ctx   *typeCtx
	reg   *nameRegistry
	// hoisted maps "<defName>.<prop>" to the enum type name already emitted,
	// so a property can refer to the union hoisted out of it.
	hoisted map[string]string
	first   bool
}

func newFileEmitter(store *schemaStore, f *schemaFile, reg *nameRegistry) *fileEmitter {
	w := newTSWriter()
	return &fileEmitter{
		store:   store,
		file:    f,
		w:       w,
		ctx:     &typeCtx{store: store, file: f, w: w},
		reg:     reg,
		hoisted: map[string]string{},
		first:   true,
	}
}

// section writes a blank line between declarations, but not before the first.
func (e *fileEmitter) section() {
	if e.first {
		e.first = false
		return
	}
	e.w.blank()
}

// declare records a top-level export, failing on any collision within the file
// or its module barrel.
func (e *fileEmitter) declare(name string) error {
	src := e.file.Path
	if err := e.reg.claim("file "+e.file.TSPath(), name, src); err != nil {
		return err
	}
	return e.reg.claim("module "+e.file.Module, name, src)
}

func (e *fileEmitter) emit() (string, error) {
	if body, ok := fixedBody[e.file.Path]; ok {
		for _, name := range fixedBodyExports(body) {
			if err := e.declare(name); err != nil {
				return "", err
			}
		}
		return normalizeTrailing(genHeader + "\n// Source: " + e.file.Path + "\n\n" + body), nil
	}

	for _, key := range e.file.DefKeys {
		if err := e.emitDef(key); err != nil {
			return "", fmt.Errorf("%s: %w", key, err)
		}
	}
	return e.w.String(e.file.Path), nil
}

func (e *fileEmitter) emitDef(key string) error {
	def := e.file.Def(key)
	_, name := splitDefKey(key)

	switch classify(def) {
	case kindObject:
		if err := e.emitObject(key, name, def); err != nil {
			return err
		}
	case kindArray:
		if err := e.emitArray(key, name, def); err != nil {
			return err
		}
	case kindEnum:
		if err := e.emitEnum(name, def, def.Description, docTags(def)); err != nil {
			return err
		}
	case kindMap:
		if err := e.emitMap(name, def); err != nil {
			return err
		}
	case kindEmptyObj:
		if err := e.emitEmptyObject(name, def); err != nil {
			return err
		}
	case kindValue:
		if err := e.emitValue(name, def); err != nil {
			return err
		}
	}
	return e.emitSchemaID(key, name)
}

// emitValue renders a scalar wrapper such as cbc.Code or num.Amount. The
// constraints (pattern, format, length) survive as JSDoc; branding them into
// nominal types would force casts at every literal and make the library
// noticeably worse to use.
func (e *fileEmitter) emitValue(name string, def *JSONSchema) error {
	if err := e.declare(name); err != nil {
		return err
	}
	expr, err := e.ctx.typeExpr(&JSONSchema{Type: orDefault(def.Type, "string")})
	if err != nil {
		return err
	}
	e.section()
	e.w.b.WriteString(jsDoc("", def.Description, docTags(def)))
	e.w.linef("export type %s = %s;", name, expr)
	return nil
}

// emitEnum renders a union of string constants, plus a label map carrying the
// human-readable titles and descriptions so consumers can build pickers
// without re-reading the schema.
func (e *fileEmitter) emitEnum(name string, def *JSONSchema, description string, tags []string) error {
	entries := def.constEntries()
	strict := def.isStrictEnum()

	unionName := name
	if !strict {
		unionName = name + "Known"
	}

	values := make([]string, 0, len(entries))
	for _, ent := range entries {
		v, err := ent.constString()
		if err != nil {
			return err
		}
		values = append(values, v)
	}

	if !strict {
		if err := e.declare(unionName); err != nil {
			return err
		}
	}
	if err := e.declare(name); err != nil {
		return err
	}
	if err := e.declare(name + "Labels"); err != nil {
		return err
	}

	e.section()
	if strict {
		e.w.b.WriteString(jsDoc("", description, tags))
		e.w.b.WriteString(unionDecl(name, stringUnion(values)))
	} else {
		e.w.b.WriteString(jsDoc("", "Known values for {@link "+name+"}.", nil))
		e.w.b.WriteString(unionDecl(unionName, stringUnion(values)))
		e.w.blank()
		e.w.b.WriteString(jsDoc("", description, tags))
		// The `(string & {})` arm preserves autocomplete for the known values
		// while still accepting the open-ended values the schema permits.
		e.w.linef("export type %s = %s | (string & {});", name, unionName)
	}

	e.w.blank()
	e.w.linef("export const %sLabels: EnumLabels<%s> = {", name, unionName)
	for i, ent := range entries {
		e.w.linef("  %s: {", quoteKey(values[i]))
		if ent.Title != "" {
			e.w.b.WriteString(objectEntry("    ", "title", quote(ent.Title)))
		}
		if ent.Description != "" {
			e.w.b.WriteString(objectEntry("    ", "description", quote(ent.Description)))
		}
		e.w.line("  },")
	}
	e.w.line("};")
	e.w.importNamed("../label"+importSuffix, "EnumLabels")
	return nil
}

// emitMap renders a patternProperties type. The key pattern cannot be
// expressed in TypeScript, so it is recorded as JSDoc.
func (e *fileEmitter) emitMap(name string, def *JSONSchema) error {
	if def.PatternProperties.Len() != 1 {
		return fmt.Errorf("expected exactly one patternProperties entry, got %d", def.PatternProperties.Len())
	}
	pair := def.PatternProperties.Oldest()
	value, err := e.ctx.typeExpr(pair.Value)
	if err != nil {
		return err
	}
	if err := e.declare(name); err != nil {
		return err
	}
	tags := docTags(def, "@pattern "+singleLine(pair.Key))
	e.section()
	e.w.b.WriteString(jsDoc("", def.Description, tags))
	e.w.linef("export type %s = { [key: string]: %s };", name, value)
	return nil
}

// emitEmptyObject renders schema.Object, the polymorphic hole an envelope's
// document sits in. It is a type alias rather than an interface so that it
// carries an implicit index signature and stays assignable to Record types.
func (e *fileEmitter) emitEmptyObject(name string, def *JSONSchema) error {
	if err := e.declare(name); err != nil {
		return err
	}
	e.section()
	e.w.b.WriteString(jsDoc("", def.Description, docTags(def)))
	e.w.linef("export type %s = { [key: string]: unknown };", name)
	return nil
}

// emitArray renders a top-level array definition such as tax.Set.
func (e *fileEmitter) emitArray(key, name string, def *JSONSchema) error {
	elem, err := e.hoistOrType(name, "[]", def.Items)
	if err != nil {
		return err
	}
	if err := e.declare(name); err != nil {
		return err
	}
	e.section()
	e.w.b.WriteString(jsDoc("", def.Description, docTags(def)))
	e.w.linef("export type %s = %s;", name, arrayOf(elem))
	return nil
}

// emitObject renders an interface, hoisting any inline property enums to named
// unions first so that no union ever appears in a property position.
func (e *fileEmitter) emitObject(key, name string, def *JSONSchema) error {
	// Hoist in declaration order so the emitted enums read in the same order as
	// the properties that use them.
	for pair := def.Properties.Oldest(); pair != nil; pair = pair.Next() {
		if _, err := e.hoistOrType(name, pair.Key, pair.Value); err != nil {
			return fmt.Errorf("property %q: %w", pair.Key, err)
		}
	}

	gen, hasGeneric := genericOverrides[key]

	if err := e.declare(name); err != nil {
		return err
	}
	var defTags []string
	if hasConditional(def) {
		// The schema restricts one property's values based on another's via
		// if/then. Encoding that needs a discriminated union, but the
		// discriminant (tax.Combo.cat) is an open cbc.Code, so the union would
		// need a fallback arm that absorbs the narrow ones: no added safety, at
		// the cost of the single nominal name 20+ properties refer to. Record
		// the constraint instead.
		defTags = append(defTags,
			"@remarks The source schema further restricts some property values "+
				"conditionally (JSON Schema if/then). That constraint is not "+
				"expressible in TypeScript and is not encoded here.")
	}

	e.section()
	e.w.b.WriteString(jsDoc("", def.Description, docTags(def, defTags...)))

	decl := "export interface " + name
	if hasGeneric {
		decl += "<" + gen.Params + ">"
		// The default type argument references schema.Object.
		if strings.Contains(gen.Params, "schema.") {
			e.w.importNamespace("../schema/index"+importSuffix, "schema")
		}
	}
	e.w.line(decl + " {")

	for pair := def.Properties.Oldest(); pair != nil; pair = pair.Next() {
		prop, propName := pair.Value, pair.Key

		var expr string
		if hasGeneric && gen.PropTypes[propName] != "" {
			expr = gen.PropTypes[propName]
		} else if h, ok := e.hoisted[name+"."+propName]; ok {
			expr = h
		} else if h, ok := e.hoisted[name+"."+propName+".[]"]; ok {
			expr = arrayOf(h)
		} else {
			var err error
			expr, err = e.ctx.typeExpr(prop)
			if err != nil {
				return fmt.Errorf("property %q: %w", propName, err)
			}
		}

		var tags []string
		if isRecommended(def, propName) {
			tags = append(tags, "@recommended")
		}
		if prop.Calculated {
			tags = append(tags, "@calculated Computed by GOBL; omit when building a document.")
		}
		doc := jsDoc("  ", prop.Description, docTags(prop, tags...))
		e.w.b.WriteString(doc)

		opt := ""
		if isOptional(def, propName) {
			opt = "?"
		}
		e.w.linef("  %s%s: %s;", quoteKey(propName), opt, expr)
	}
	e.w.line("}")

	// Because calculated properties are optional on input, a built document's
	// guaranteed fields would otherwise be invisible. Exporting the key list
	// lets the hand-written Calculated<T, K> helper recover the read-side view
	// without generating a second interface per definition.
	if keys := calculatedKeys(def); len(keys) > 0 {
		labelName := name + "CalculatedKeys"
		if err := e.declare(labelName); err != nil {
			return err
		}
		e.w.blank()
		e.w.b.WriteString(jsDoc("", "Properties of {@link "+name+"} that GOBL computes during calculation.", nil))
		e.w.b.WriteString(unionDecl(labelName, stringUnion(keys)))
	}
	return nil
}

// hoistOrType emits a named union for an inline enum attached to a property or
// array item, returning the type expression to use in its place. It returns an
// empty string when there is nothing to hoist.
//
// Hoisting keeps every property type a short identifier, which is what limits
// the emitter to two width rules, and gives each union a name that the label
// map and consumer code can refer to.
func (e *fileEmitter) hoistOrType(defName, prop string, s *JSONSchema) (string, error) {
	if s == nil {
		return "", nil
	}

	// An array-valued property whose items carry the enum.
	if s.Type == "array" && s.Items != nil && s.Items.hasEnum() {
		name, err := e.hoistEnum(defName, prop, s, s.Items)
		if err != nil {
			return "", err
		}
		e.hoisted[defName+"."+prop+".[]"] = name
		return name, nil
	}

	if !s.hasEnum() {
		if prop == "[]" {
			return e.ctx.typeExpr(s)
		}
		return "", nil
	}

	name, err := e.hoistEnum(defName, prop, s, s)
	if err != nil {
		return "", err
	}
	e.hoisted[defName+"."+prop] = name
	return name, nil
}

func (e *fileEmitter) hoistEnum(defName, prop string, doc, enum *JSONSchema) (string, error) {
	name := hoistName(e.file.Module, defName, prop)
	description := doc.Description
	if description == "" {
		description = enum.Description
	}
	tags := docTags(&JSONSchema{Title: doc.Title, Format: doc.Format})
	if err := e.emitEnum(name, enum, description, tags); err != nil {
		return "", err
	}
	return name, nil
}

// emitSchemaID exports the canonical identifier for a definition. The primary
// definition of a file uses the document $id; the others append a $defs
// fragment, matching how GOBL itself addresses sub-schemas.
func (e *fileEmitter) emitSchemaID(key, name string) error {
	id := e.file.ID
	if key != e.file.PrimaryDefKey {
		id = e.file.ID + "#/$defs/" + key
	}
	constName := name + "SchemaID"
	if err := e.declare(constName); err != nil {
		return err
	}
	e.section()
	e.w.b.WriteString(constDecl(constName, quote(id)))
	return nil
}

// fixedBodyExports extracts the exported names from a hand-written body so
// that collisions with generated names are still detected.
func fixedBodyExports(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		for _, kw := range []string{"export type ", "export const ", "export interface "} {
			if strings.HasPrefix(line, kw) {
				rest := strings.TrimPrefix(line, kw)
				name := rest
				for i, r := range rest {
					if !(r == '_' || r == '$' || (r >= 'a' && r <= 'z') ||
						(r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
						name = rest[:i]
						break
					}
				}
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return out
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
