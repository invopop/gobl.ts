package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/tax"
)

// The registry pass covers only what the JSON Schemas cannot.
//
// tax/regime-code.json, tax/addon-list.json and currency/code.json already
// carry their unions as strict enums, and the addon list is built from
// AllAddonDefs() together with ApprovedAddons(), so it already includes the
// external addons. Re-emitting any of those here would produce duplicate
// exports. What remains is the extension key space and its per-key value
// domains, which live only in the Go registries.

// extensionDef is one flattened extension definition.
type extensionDef struct {
	Key    string
	Source string // where it came from, for the doc comment
	Name   i18n.String
	Desc   i18n.String
	Values []*cbc.Definition
	// Open is true when the value space is a pattern or a code map rather than
	// an enumerated list.
	Open bool
}

// emitRegistry returns the registry files keyed by name within src/gen/registry.
func emitRegistry() (map[string]string, error) {
	if err := assertAddonsRegistered(); err != nil {
		return nil, err
	}

	defs, err := collectExtensions()
	if err != nil {
		return nil, err
	}

	files := map[string]string{
		"extension-values.ts": emitExtensionValues(defs),
		"extension-keys.ts":   emitExtensionKeys(defs),
		"catalogues.ts":       emitCatalogues(),
		"index.ts":            emitRegistryBarrel(),
	}
	return files, nil
}

// assertAddonsRegistered fails when an approved addon is not actually linked
// in. Because ExtensionKey narrows tax.Extensions, a missing bundle module
// would silently shrink the union and start rejecting previously valid
// documents, so this must be fatal rather than a warning.
func assertAddonsRegistered() error {
	var missing []string
	for _, ext := range tax.ApprovedAddons() {
		if tax.AddonForKey(ext.Key) == nil {
			missing = append(missing, fmt.Sprintf("%s (%s)", ext.Key, ext.Module))
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf(
			"approved addons are not registered: %s\n"+
				"add the module to github.com/invopop/gobl.dev/bundle, or the generated "+
				"ExtensionKey union will be incomplete",
			strings.Join(missing, ", "),
		)
	}
	return nil
}

// collectExtensions gathers every extension definition. All three registries
// carry extensions, not only addons and catalogues: tax regimes define their
// own as well.
func collectExtensions() ([]extensionDef, error) {
	byKey := map[string]extensionDef{}

	add := func(source string, list []*cbc.Definition) error {
		for _, d := range list {
			key := d.Key.String()
			if key == "" {
				return fmt.Errorf("%s: extension definition with no key", source)
			}
			def := extensionDef{
				Key:    key,
				Source: source,
				Name:   d.Name,
				Desc:   d.Desc,
				Values: d.Values,
				Open:   len(d.Values) == 0,
			}
			if prev, ok := byKey[key]; ok {
				// The same key may be contributed by more than one source; keep
				// the richer definition.
				if len(prev.Values) >= len(def.Values) {
					continue
				}
			}
			byKey[key] = def
		}
		return nil
	}

	for _, r := range tax.AllRegimeDefs() {
		if err := add("regime "+r.Country.String(), r.Extensions); err != nil {
			return nil, err
		}
	}
	for _, a := range tax.AllAddonDefs() {
		if err := add("addon "+a.Key.String(), a.Extensions); err != nil {
			return nil, err
		}
	}
	for _, c := range tax.AllCatalogueDefs() {
		if err := add("catalogue "+c.Key.String(), c.Extensions); err != nil {
			return nil, err
		}
	}

	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]extensionDef, 0, len(keys))
	for _, k := range keys {
		out = append(out, byKey[k])
	}
	return out, nil
}

// valueCode returns the wire value of one extension value definition. Core
// GOBL always uses Code, but external addon modules are not visible to static
// analysis here, so Key is accepted as a fallback and ambiguity is an error.
func valueCode(d *cbc.Definition) (string, error) {
	code, key := d.Code.String(), d.Key.String()
	switch {
	case code != "" && key != "":
		return "", fmt.Errorf("extension value defines both code %q and key %q", code, key)
	case code != "":
		return code, nil
	case key != "":
		return key, nil
	}
	return "", fmt.Errorf("extension value defines neither code nor key")
}

// emitExtensionValues renders the key-to-value-domain map. It is types only,
// so it costs nothing at runtime while giving each extension key a precise
// value union.
func emitExtensionValues(defs []extensionDef) string {
	var b strings.Builder
	b.WriteString(genHeader + "\n\n")
	b.WriteString("import type { Code } from '../cbc/code" + importSuffix + "';\n\n")
	b.WriteString(jsDoc("", "Value domain of every extension key registered in this build of GOBL.\n\n"+
		"Keys whose values are validated by a pattern or code map rather than an\n"+
		"enumerated list fall back to the open `cbc.Code` type.", nil))
	b.WriteString("export interface ExtensionValues {\n")

	for _, d := range defs {
		doc := d.Name.String()
		if desc := d.Desc.String(); desc != "" {
			doc = strings.TrimRight(doc, ".") + ". " + desc
		}
		b.WriteString(jsDoc("  ", doc, []string{"@source " + d.Source}))

		expr := "Code"
		if !d.Open {
			var values []string
			for _, v := range d.Values {
				code, err := valueCode(v)
				if err != nil {
					// Fall back to the open type rather than emitting a broken
					// union; the assertion in valueCode surfaces real problems
					// through collectExtensions.
					values = nil
					break
				}
				values = append(values, code)
			}
			if len(values) > 0 {
				expr = strings.Join(stringUnion(values), " | ")
			}
		}

		b.WriteString(propUnion("  ", quoteKey(d.Key), expr))
	}

	b.WriteString("}\n\n")
	b.WriteString(jsDoc("", "Every extension key registered in this build of GOBL.", nil))
	b.WriteString("export type ExtensionKey = keyof ExtensionValues;\n")
	return normalizeTrailing(b.String())
}

// emitExtensionKeys renders the human-readable labels for extension keys. This
// is runtime data, but small enough (one entry per key) to keep alongside the
// types; the far larger per-value label sets stay in their own subpath files.
func emitExtensionKeys(defs []extensionDef) string {
	var b strings.Builder
	b.WriteString(genHeader + "\n\n")
	b.WriteString("import type { EnumLabels } from '../label" + importSuffix + "';\n")
	b.WriteString("import type { ExtensionKey } from './extension-values" + importSuffix + "';\n\n")
	b.WriteString(jsDoc("", "Display names for every extension key, in every language GOBL defines them in.", nil))
	b.WriteString("export const ExtensionKeyLabels: EnumLabels<ExtensionKey> = {\n")
	for _, d := range defs {
		b.WriteString("  " + quoteKey(d.Key) + ": {\n")
		if name := d.Name.String(); name != "" {
			b.WriteString(objectEntry("    ", "title", quote(name)))
		}
		if desc := d.Desc.String(); desc != "" {
			b.WriteString(objectEntry("    ", "description", quote(desc)))
		}
		b.WriteString("  },\n")
	}
	b.WriteString("};\n")
	return normalizeTrailing(b.String())
}

// emitCatalogues renders the catalogue key union.
func emitCatalogues() string {
	defs := tax.AllCatalogueDefs()
	keys := make([]string, 0, len(defs))
	for _, d := range defs {
		keys = append(keys, d.Key.String())
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(genHeader + "\n\n")
	b.WriteString(jsDoc("", "Extension catalogues registered in this build of GOBL.", nil))
	b.WriteString(unionDecl("CatalogueKey", stringUnion(keys)))
	return normalizeTrailing(b.String())
}

// emitRegistryBarrel re-exports only the type-level declarations.
//
// ExtensionKeyLabels is deliberately excluded: it is ~90 KB of display strings
// in every language GOBL defines, which has no place in the root import. It
// stays reachable on its own subpath, so an application that renders extension
// pickers can opt in:
//
//	import { ExtensionKeyLabels } from '@invopop/gobl.ts/registry/extension-keys';
func emitRegistryBarrel() string {
	var b strings.Builder
	b.WriteString(genHeader + "\n\n")
	b.WriteString("export * from './catalogues" + importSuffix + "';\n")
	b.WriteString("export * from './extension-values" + importSuffix + "';\n")
	return normalizeTrailing(b.String())
}
