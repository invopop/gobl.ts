package main

// The override table is this generator's escape hatch, replacing the
// SkipFileError / custom-ref mechanism in gobl.generator. It is deliberately
// tiny: every entry is a place where the JSON Schema cannot express something
// TypeScript can, and each one is justified below.

// hoistRename gives a better name to a hoisted enum than the mechanical
// "<Def><Property>" would. Keyed by "<module>.<Def>.<property>", or
// "<module>.<Def>.[]" for an array definition's item type.
var hoistRename = map[string]string{
	// The element of an addon list is a single addon key. "AddonListItem"
	// describes the schema; "AddonKey" describes the value.
	"tax.AddonList.[]": "AddonKey",
	// One element of "$tags" is a single tag, so the type reads better
	// singular: `$tags?: InvoiceTag[]`.
	"bill.Invoice.$tags": "InvoiceTag",
}

// genericOverride turns a definition into a generic interface. Used only for
// the envelope, whose "doc" is deliberately untyped in the schema
// (schema.Object exists precisely to be a polymorphic hole) but which callers
// always know the concrete type of.
type genericOverride struct {
	// Params is the type parameter list, without angle brackets.
	Params string
	// PropTypes replaces the resolved type of the named properties.
	PropTypes map[string]string
}

var genericOverrides = map[string]genericOverride{
	// `T extends object`, not `T extends Record<string, unknown>`: an interface
	// has no implicit index signature, so the latter would reject every
	// generated document type, including bill.Invoice.
	"gobl.Envelope": {
		Params:    "T extends object = schema.Object",
		PropTypes: map[string]string{"doc": "T"},
	},
}

// fixedBody replaces the generated body of an entire schema file. The only
// user is tax/extensions, whose schema says Record<string, cbc.Code> while the
// registry knows the exact key set and each key's value domain.
var fixedBody = map[string]string{
	"schemas/tax/extensions.json": `import type { Code } from '../cbc/code.js';
import type { ExtensionValues } from '../registry/extension-values.js';

/**
 * Extensions are a key component of GOBL that are used to include additional structured data in
 * documents that doesn't fit into any of the common or universal fields.
 *
 * Keys and their value domains are generated from the tax regimes, addons and catalogues
 * registered in this build of GOBL.
 *
 * @pattern ^(?:[a-z]|[a-z0-9][a-z0-9-+]*[a-z0-9])$
 */
export type Extensions = { [K in keyof ExtensionValues]?: ExtensionValues[K] };

/**
 * Loosely typed extension map, accepting any key. Use this when working with
 * documents that may carry extension keys from a newer release of GOBL than
 * this package was generated against.
 */
export type ExtensionsAny = { [key: string]: Code };

export const ExtensionsSchemaID = 'https://gobl.org/draft-0/tax/extensions';
`,
}

// hoistName returns the type name for a hoisted enum.
func hoistName(module, defName, prop string) string {
	if n, ok := hoistRename[module+"."+defName+"."+prop]; ok {
		return n
	}
	if prop == "[]" {
		return defName + "Item"
	}
	return defName + pascalCase(prop)
}
