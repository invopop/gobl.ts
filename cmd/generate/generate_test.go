package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchemaStoreInvariants(t *testing.T) {
	store, err := newSchemaStore()
	if err != nil {
		t.Fatalf("loading schemas: %v", err)
	}

	// The loader asserts these itself; this pins the shape of the data the
	// emitters were written against, so an upstream change is visible here
	// rather than as a puzzling diff.
	if got := len(store.files); got < 80 {
		t.Errorf("expected at least 80 schema files, got %d", got)
	}

	defs := 0
	for _, f := range store.files {
		defs += len(f.DefKeys)
		if f.Module == "" {
			t.Errorf("%s: empty module", f.Path)
		}
		if _, ok := f.Root.Defs[f.PrimaryDefKey]; !ok {
			t.Errorf("%s: primary def %q missing", f.Path, f.PrimaryDefKey)
		}
	}
	if defs < 95 {
		t.Errorf("expected at least 95 definitions, got %d", defs)
	}

	// envelope.json is the one file whose $id path carries no package segment,
	// so a URL-derived module would be empty. Deriving from the $defs key
	// ("gobl.Envelope") is what makes the rule total.
	env, ok := store.byID["https://gobl.org/draft-0/envelope"]
	if !ok {
		t.Fatal("envelope schema not found")
	}
	if env.Module != "gobl" {
		t.Errorf("envelope module = %q, want %q", env.Module, "gobl")
	}
}

// TestDeterministic is the guarantee that regenerating an unchanged GOBL
// version produces an identical tree, so `git diff --exit-code` in CI is a
// meaningful check rather than a source of noise.
func TestDeterministic(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	for _, dir := range []string{a, b} {
		if err := run(filepath.Join(dir, "gen"), "v0.0.0-test"); err != nil {
			t.Fatalf("generating into %s: %v", dir, err)
		}
	}

	files := map[string]bool{}
	read := func(root string) map[string]string {
		out := map[string]string{}
		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			body, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			out[rel] = string(body)
			files[rel] = true
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	first, second := read(filepath.Join(a, "gen")), read(filepath.Join(b, "gen"))
	if len(first) == 0 {
		t.Fatal("no files generated")
	}
	for name := range files {
		if first[name] != second[name] {
			t.Errorf("%s differs between runs", name)
		}
	}
}

// TestOwnedTreeGuard checks that a mistyped -out cannot delete hand-written
// source.
func TestOwnedTreeGuard(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mine.ts"), []byte("export const x = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := assertOwnedTree(dir); err == nil {
		t.Error("expected a refusal to wipe a directory holding foreign files")
	}

	generated := filepath.Join(t.TempDir())
	if err := os.WriteFile(filepath.Join(generated, "gen.ts"), []byte(genHeader+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := assertOwnedTree(generated); err != nil {
		t.Errorf("expected a generated tree to be wipeable, got %v", err)
	}
}

func TestQuote(t *testing.T) {
	// Prettier keeps singleQuote but switches to double quotes when that needs
	// fewer escapes; reproducing it is what keeps `prettier --check` clean.
	for _, tc := range []struct{ in, want string }{
		{"plain", `'plain'`},
		{"Côte d'Ivoire", `"Côte d'Ivoire"`},
		{`say "hi"`, `'say "hi"'`},
		{"line\nbreak", `'line\nbreak'`},
		{`back\slash`, `'back\\slash'`},
	} {
		if got := quote(tc.in); got != tc.want {
			t.Errorf("quote(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestUnionDecl(t *testing.T) {
	short := unionDecl("T", []string{"'a'", "'b'"})
	if short != "export type T = 'a' | 'b';\n" {
		t.Errorf("short union = %q", short)
	}

	// A union too wide for one line but narrow enough for an indented
	// continuation takes prettier's intermediate form before going one per line.
	mid := unionDecl(strings.Repeat("N", 70), []string{"'aaaaaaaaaa'", "'bbbbbbbbbb'"})
	if !strings.Contains(mid, "=\n  'aaaaaaaaaa' | 'bbbbbbbbbb';") {
		t.Errorf("expected an indented continuation, got %q", mid)
	}

	members := make([]string, 20)
	for i := range members {
		members[i] = "'valuevaluevalue'"
	}
	long := unionDecl("T", members)
	if !strings.Contains(long, "\n  | 'valuevaluevalue'") {
		t.Errorf("expected one member per line, got %q", long)
	}
}

func TestPascalCase(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"$tags", "Tags"},
		{"issue_date", "IssueDate"},
		{"credit-note", "CreditNote"},
		{"type", "Type"},
	} {
		if got := pascalCase(tc.in); got != tc.want {
			t.Errorf("pascalCase(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRejectsUnknownKeywords guards the partial schema model: a keyword the
// emitter does not understand must fail the build rather than be dropped.
func TestRejectsUnknownKeywords(t *testing.T) {
	var s JSONSchema
	err := s.UnmarshalJSON([]byte(`{"type":"object","additionalProperties":false}`))
	if err == nil {
		t.Fatal("expected an error for an unmodelled keyword")
	}
	if !strings.Contains(err.Error(), "additionalProperties") {
		t.Errorf("error should name the keyword, got %v", err)
	}
}

// TestIgnoresMalformedEnum covers the upstream bug in bill/delivery.json, where
// "enum" is a bare string. Reading it as a slice would crash the generator.
func TestIgnoresMalformedEnum(t *testing.T) {
	var s JSONSchema
	if err := s.UnmarshalJSON([]byte(`{"type":"string","enum":"advice"}`)); err != nil {
		t.Fatalf("a malformed enum must not fail the load: %v", err)
	}
}
