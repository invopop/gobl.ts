package main

import (
	"fmt"
	"strings"
)

// importSuffix is appended to every relative specifier. Required by
// moduleResolution "nodenext" and harmless under bundler resolution.
const importSuffix = ".js"

// typeCtx resolves schema nodes to TypeScript type expressions for one output
// file, recording the imports each resolution needs.
type typeCtx struct {
	store *schemaStore
	file  *schemaFile
	w     *tsWriter
}

// typeExpr renders the TypeScript type for a schema node appearing in a
// property, item or definition position.
func (c *typeCtx) typeExpr(s *JSONSchema) (string, error) {
	if s == nil {
		return "unknown", nil
	}
	if s.Ref != "" {
		return c.resolveRef(s.Ref)
	}
	if s.Type == "array" && s.Items != nil {
		inner, err := c.typeExpr(s.Items)
		if err != nil {
			return "", err
		}
		return arrayOf(inner), nil
	}
	if s.hasEnum() {
		// An inline composition with no $ref, used directly as a type.
		members, err := c.enumMembers(s)
		if err != nil {
			return "", err
		}
		return strings.Join(members, " | "), nil
	}
	switch s.Type {
	case "string":
		return "string", nil
	case "boolean":
		return "boolean", nil
	case "integer", "number":
		return "number", nil
	case "object":
		return "{ [key: string]: unknown }", nil
	case "":
		return "unknown", nil
	}
	return "", fmt.Errorf("unsupported type %q", s.Type)
}

// arrayOf wraps an element type, parenthesising unions so `A | B` becomes
// `(A | B)[]`.
func arrayOf(inner string) string {
	if strings.Contains(inner, "|") || strings.Contains(inner, "&") {
		return "(" + inner + ")[]"
	}
	return inner + "[]"
}

// enumMembers renders the const entries of a composition as union members,
// appending the open-ended escape when the composition is not strict.
func (c *typeCtx) enumMembers(s *JSONSchema) ([]string, error) {
	entries := s.constEntries()
	values := make([]string, 0, len(entries))
	for _, e := range entries {
		v, err := e.constString()
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	members := stringUnion(values)
	if !s.isStrictEnum() {
		// `(string & {})` keeps editor autocomplete for the known values while
		// still accepting any other string, matching the schema's escape entry.
		members = append(members, "(string & {})")
	}
	return members, nil
}

// resolveRef maps a $ref to a type expression, registering any import needed.
//
// GOBL v0.505.0 emits only two forms: local "#/$defs/pkg.Type" and an absolute
// URL with no fragment. The fragment form is handled defensively in case a
// future release starts emitting it.
func (c *typeCtx) resolveRef(ref string) (string, error) {
	if strings.HasPrefix(ref, "#") {
		key, err := refDefKey(ref)
		if err != nil {
			return "", err
		}
		if _, ok := c.file.Root.Defs[key]; !ok {
			return "", fmt.Errorf("local $ref %q not found in %s", ref, c.file.Path)
		}
		_, name := splitDefKey(key)
		return name, nil
	}

	base, frag, _ := strings.Cut(ref, "#")
	target, ok := c.store.byID[base]
	if !ok {
		return "", fmt.Errorf("$ref %q does not resolve to any known schema", ref)
	}

	key := target.PrimaryDefKey
	if frag != "" {
		k, err := refDefKey(frag)
		if err != nil {
			return "", fmt.Errorf("$ref %q: %w", ref, err)
		}
		if _, ok := target.Root.Defs[k]; !ok {
			return "", fmt.Errorf("$ref %q names a definition absent from %s", ref, target.Path)
		}
		key = k
	}
	_, name := splitDefKey(key)

	// A reference whose target is this very file. Compare file identity rather
	// than ref syntax: cbc.Definition.values and org.Party.agent point at their
	// own $id by absolute URL, and treating those as external would make a file
	// import its own module barrel.
	if target == c.file {
		return name, nil
	}

	if target.Module == c.file.Module {
		spec := "./" + target.Stem + importSuffix
		c.w.importNamed(spec, name)
		return name, nil
	}

	spec := "../" + target.Module + "/index" + importSuffix
	c.w.importNamespace(spec, target.Module)
	return target.Module + "." + name, nil
}

// docTags builds the JSDoc annotations for a schema node. These carry the
// constraints that TypeScript cannot express, so they stay visible on hover.
func docTags(s *JSONSchema, extra ...string) []string {
	var tags []string
	if s.Title != "" {
		tags = append(tags, "@title "+singleLine(s.Title))
	}
	if s.Format != "" {
		tags = append(tags, "@format "+s.Format)
	}
	if s.Pattern != "" {
		tags = append(tags, "@pattern "+singleLine(s.Pattern))
	}
	if s.MinLength != nil {
		tags = append(tags, fmt.Sprintf("@minLength %d", *s.MinLength))
	}
	if s.MaxLength != nil {
		tags = append(tags, fmt.Sprintf("@maxLength %d", *s.MaxLength))
	}
	if s.ContentEncoding != "" {
		tags = append(tags, "@contentEncoding "+s.ContentEncoding)
	}
	for _, ex := range s.Examples {
		tags = append(tags, "@example "+singleLine(string(ex)))
	}
	return append(tags, extra...)
}

func singleLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}
