package main

import (
	"fmt"
	"sort"
	"strings"
)

// emitModuleBarrel renders a module's index.ts. Exports are sorted by the
// emitted specifier, which is what an import-sorting linter would do and keeps
// the file stable across runs. Note "./code-map.js" sorts before "./code.js",
// since '-' precedes '.' in byte order.
func emitModuleBarrel(files []*schemaFile) string {
	specs := make([]string, 0, len(files))
	for _, f := range files {
		specs = append(specs, "./"+f.Stem+importSuffix)
	}
	sort.Strings(specs)

	var b strings.Builder
	b.WriteString(genHeader + "\n\n")
	for _, s := range specs {
		fmt.Fprintf(&b, "export * from %s;\n", quote(s))
	}
	return normalizeTrailing(b.String())
}

// emitRootBarrel renders src/gen/index.ts, the namespace re-export that makes
// `import type { bill } from '@invopop/gobl.ts'` work. Namespacing is not
// cosmetic: Amount, Code, Identity and Note each exist in two or three GOBL
// packages, so a flat export surface is impossible.
func emitRootBarrel(modules []string, hasRegistry bool) string {
	var b strings.Builder
	b.WriteString(genHeader + "\n\n")
	b.WriteString("export type { EnumLabel, EnumLabels } from './label" + importSuffix + "';\n")
	b.WriteString("export * from './schemas" + importSuffix + "';\n")
	b.WriteString("export { GOBL_VERSION } from './version" + importSuffix + "';\n")

	names := append([]string{}, modules...)
	if hasRegistry {
		names = append(names, "registry")
	}
	sort.Strings(names)
	for _, m := range names {
		fmt.Fprintf(&b, "export * as %s from './%s/index%s';\n", m, m, importSuffix)
	}
	return normalizeTrailing(b.String())
}

// emitLabel renders the shared EnumLabel type used by every generated label map.
func emitLabel() string {
	return normalizeTrailing(genHeader + `

/** Human-readable metadata for one value of a generated enum. */
export interface EnumLabel {
  /** Short display name, suitable for a form control. */
  title?: string;
  /** Longer explanation of when the value applies. */
  description?: string;
}

/**
 * Labels for every value of a generated enum.
 *
 * Spelled as a mapped type rather than the Record utility type, because GOBL
 * defines a type named Record (pay.Record) that shadows the TypeScript
 * built-in inside its own generated module.
 */
export type EnumLabels<K extends string> = { [P in K]: EnumLabel };
`)
}

// emitVersion records which GOBL release these types were generated from.
//
// The version lives here alone rather than in every file header: putting it in
// all 88 headers would make each GOBL bump touch every file and bury the real
// diff, and `debug.ReadBuildInfo` reports "(devel)" when go.mod has a local
// replace directive, so per-file headers would not even be reproducible.
func emitVersion(version string) string {
	return normalizeTrailing(fmt.Sprintf(`%s

/** Version of github.com/invopop/gobl these types were generated from. */
export const GOBL_VERSION = %s;
`, genHeader, quote(version)))
}

// emitSchemaMap renders the mapping from every GOBL schema ID to the type it
// describes. Only code generation can produce this, and it is what lets a
// caller narrow an envelope by the `$schema` it carries:
//
//	type Doc = SchemaTypes['https://gobl.org/draft-0/bill/invoice'];
func emitSchemaMap(store *schemaStore) string {
	type entry struct{ id, expr string }
	var entries []entry
	modules := map[string]bool{}

	for _, f := range store.files {
		for _, key := range f.DefKeys {
			id := f.ID
			if key != f.PrimaryDefKey {
				id = f.ID + "#/$defs/" + key
			}
			_, name := splitDefKey(key)
			modules[f.Module] = true
			entries = append(entries, entry{id: id, expr: f.Module + "." + name})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })

	names := make([]string, 0, len(modules))
	for m := range modules {
		names = append(names, m)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString(genHeader + "\n\n")
	for _, m := range names {
		fmt.Fprintf(&b, "import type * as %s from './%s/index%s';\n", m, m, importSuffix)
	}
	b.WriteString("\n")
	b.WriteString(jsDoc("", "Every GOBL schema ID, mapped to the type it describes.\n\n"+
		"Use it to type an envelope from the `$schema` its document carries.", nil))
	b.WriteString("export interface SchemaTypes {\n")
	for _, e := range entries {
		b.WriteString(objectEntryType("  ", quoteKey(e.id), e.expr))
	}
	b.WriteString("}\n\n")
	b.WriteString(jsDoc("", "The ID of any schema defined by this version of GOBL.", nil))
	b.WriteString("export type SchemaID = keyof SchemaTypes;\n\n")
	b.WriteString(jsDoc("", "Every schema ID, available at runtime.", nil))
	b.WriteString("export const SchemaIDs: readonly SchemaID[] = [\n")
	for _, e := range entries {
		b.WriteString("  " + quote(e.id) + ",\n")
	}
	b.WriteString("];\n")
	return normalizeTrailing(b.String())
}

// objectEntryType renders `key: Type;` inside an interface, breaking after the
// colon when the line exceeds the print width.
func objectEntryType(indent, key, value string) string {
	line := indent + key + ": " + value + ";"
	if len(line) <= printWidth {
		return line + "\n"
	}
	return indent + key + ":\n" + indent + "  " + value + ";\n"
}
