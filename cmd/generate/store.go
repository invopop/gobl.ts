package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/invopop/gobl/data"
)

// schemaFile is one loaded JSON Schema document, together with the facts
// derived from it once so the emitters never re-parse identifiers.
type schemaFile struct {
	// Path is the FS-relative source path, e.g. "schemas/bill/invoice.json".
	Path string
	// ID is the document's $id, e.g. "https://gobl.org/draft-0/bill/invoice".
	ID string
	// Module is the TypeScript module name, taken from the $defs key prefix.
	Module string
	// Stem is the output file name without extension, e.g. "payment-details".
	Stem string
	// DefKeys are the $defs keys, sorted, e.g. ["tax.Combo", "tax.Set"].
	DefKeys []string
	// PrimaryDefKey is the $defs key the document's root $ref points at.
	PrimaryDefKey string

	Root *JSONSchema
}

// TSPath is the generated file path relative to the output root.
func (f *schemaFile) TSPath() string {
	return path.Join(f.Module, f.Stem+".ts")
}

// Def returns one definition by its full $defs key.
func (f *schemaFile) Def(key string) *JSONSchema {
	return f.Root.Defs[key]
}

type schemaStore struct {
	files  []*schemaFile // sorted by Path
	byID   map[string]*schemaFile
	byPath map[string]*schemaFile
}

// newSchemaStore walks the schemas embedded in the pinned GOBL module. There is
// deliberately no filesystem path involved: the version of the schemas is
// whatever go.mod pins, so generation is reproducible from a clean checkout
// without a sibling gobl repository.
func newSchemaStore() (*schemaStore, error) {
	s := &schemaStore{
		byID:   map[string]*schemaFile{},
		byPath: map[string]*schemaFile{},
	}

	var paths []string
	err := fs.WalkDir(data.Content, "schemas", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || path.Ext(p) != ".json" {
			return nil
		}
		paths = append(paths, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	for _, p := range paths {
		raw, err := data.Content.ReadFile(p)
		if err != nil {
			return nil, err
		}
		root := new(JSONSchema)
		if err := json.Unmarshal(raw, root); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		f, err := newSchemaFile(p, root)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if prev, ok := s.byID[f.ID]; ok {
			return nil, fmt.Errorf("%s: duplicate $id %q, already declared by %s", p, f.ID, prev.Path)
		}
		if prev, ok := s.byPath[f.TSPath()]; ok {
			return nil, fmt.Errorf(
				"%s: output path %q collides with %s; two schema directories map to one module",
				p, f.TSPath(), prev.Path,
			)
		}
		s.files = append(s.files, f)
		s.byID[f.ID] = f
		s.byPath[f.TSPath()] = f
	}

	if len(s.files) == 0 {
		return nil, fmt.Errorf("no schemas found in the embedded GOBL data")
	}
	return s, nil
}

// newSchemaFile derives a file's identifiers and asserts the structural
// invariants the emitters rely on.
func newSchemaFile(p string, root *JSONSchema) (*schemaFile, error) {
	if root.ID == "" {
		return nil, fmt.Errorf("missing $id")
	}
	if len(root.Defs) == 0 {
		return nil, fmt.Errorf("no $defs")
	}

	keys := make([]string, 0, len(root.Defs))
	modules := map[string]bool{}
	for k := range root.Defs {
		pkg, name := splitDefKey(k)
		if pkg == "" || name == "" {
			return nil, fmt.Errorf("$defs key %q is not of the form <package>.<Type>", k)
		}
		modules[pkg] = true
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// The module always comes from the $defs key prefix, never from the $id
	// URL. envelope.json is the one file where they disagree: its $id path has
	// no package segment, so a URL-derived module would be empty, while its
	// $defs key "gobl.Envelope" correctly names the Go package.
	if len(modules) != 1 {
		names := make([]string, 0, len(modules))
		for m := range modules {
			names = append(names, m)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("expected one $defs package prefix, found %v", names)
	}
	var module string
	for m := range modules {
		module = m
	}

	primary, err := refDefKey(root.Ref)
	if err != nil {
		return nil, fmt.Errorf("root $ref: %w", err)
	}
	if _, ok := root.Defs[primary]; !ok {
		return nil, fmt.Errorf("root $ref points at %q, which is not in $defs", primary)
	}

	stem := strings.TrimSuffix(path.Base(p), ".json")

	return &schemaFile{
		Path:          p,
		ID:            root.ID,
		Module:        module,
		Stem:          stem,
		DefKeys:       keys,
		PrimaryDefKey: primary,
		Root:          root,
	}, nil
}

// Modules returns the sorted list of distinct module names.
func (s *schemaStore) Modules() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range s.files {
		if !seen[f.Module] {
			seen[f.Module] = true
			out = append(out, f.Module)
		}
	}
	sort.Strings(out)
	return out
}

// ModuleFiles returns the files belonging to a module, sorted by path.
func (s *schemaStore) ModuleFiles(module string) []*schemaFile {
	var out []*schemaFile
	for _, f := range s.files {
		if f.Module == module {
			out = append(out, f)
		}
	}
	return out
}

// splitDefKey splits "bill.Invoice" into ("bill", "Invoice").
func splitDefKey(key string) (pkg, name string) {
	i := strings.LastIndex(key, ".")
	if i < 0 {
		return "", ""
	}
	return key[:i], key[i+1:]
}

// refDefKey extracts the $defs key from a local reference or fragment.
// Accepts "#/$defs/bill.Invoice", "/$defs/bill.Invoice" and "bill.Invoice".
func refDefKey(ref string) (string, error) {
	r := strings.TrimPrefix(ref, "#")
	r = strings.TrimPrefix(r, "/$defs/")
	if r == "" || strings.ContainsAny(r, "/#") {
		return "", fmt.Errorf("cannot parse %q as a $defs reference", ref)
	}
	return r, nil
}
