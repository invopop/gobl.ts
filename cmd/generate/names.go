package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// tsIdentRE matches a TypeScript identifier that needs no quoting as an object
// key. Deliberately conservative: ASCII only.
var tsIdentRE = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// pascalCase converts a schema property name into a PascalCase fragment used to
// build hoisted enum type names: "$tags" -> "Tags", "issue_date" -> "IssueDate",
// "credit-note" -> "CreditNote".
func pascalCase(s string) string {
	s = strings.TrimPrefix(s, "$")
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == ' '
	})
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

// quoteKey renders an object key, quoting it only when required. Extension keys
// and enum values are frequently hyphenated ("credit-note"), which must be
// quoted; plain identifiers are left bare to match prettier's output.
func quoteKey(k string) string {
	if tsIdentRE.MatchString(k) {
		return k
	}
	return quote(k)
}

// quote renders a TypeScript string literal.
//
// With singleQuote enabled, prettier still switches to double quotes when the
// string contains more single quotes than double ones, because that needs fewer
// escapes. Reproducing that rule is what keeps `prettier --check` clean over
// descriptions such as "item described by two's" and titles like "Cote d'Ivoire".
func quote(s string) string {
	delim := byte('\'')
	if strings.Count(s, "'") > strings.Count(s, `"`) {
		delim = '"'
	}
	var b strings.Builder
	b.WriteByte(delim)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == rune(delim):
			b.WriteByte('\\')
			b.WriteByte(delim)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(delim)
	return b.String()
}

// nameRegistry guards against two generated declarations colliding. A silent
// collision would produce a module barrel that shadows a type, so every clash
// is fatal rather than merely reported.
type nameRegistry struct {
	// owner maps scope -> name -> the source that claimed it.
	owner map[string]map[string]string
}

func newNameRegistry() *nameRegistry {
	return &nameRegistry{owner: map[string]map[string]string{}}
}

// claim records that source declares name within scope, failing if taken.
func (r *nameRegistry) claim(scope, name, source string) error {
	m, ok := r.owner[scope]
	if !ok {
		m = map[string]string{}
		r.owner[scope] = m
	}
	if prev, taken := m[name]; taken {
		return fmt.Errorf(
			"duplicate export %q in %s: declared by both %s and %s",
			name, scope, prev, source,
		)
	}
	m[name] = source
	return nil
}

// names returns the claimed names in a scope, sorted.
func (r *nameRegistry) names(scope string) []string {
	var out []string
	for n := range r.owner[scope] {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
