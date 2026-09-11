// Command generate produces the TypeScript type definitions in src/gen from
// the JSON Schemas embedded in the pinned github.com/invopop/gobl module.
//
// Because the schemas come from the Go module graph rather than a filesystem
// path, output depends only on go.mod: upgrading GOBL is a version bump, a
// re-run and a reviewable diff. No sibling checkout is required.
//
// Usage:
//
//	go run ./cmd/generate [-out src/gen] [-version vX.Y.Z]
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	// Blank imports register the tax regimes, addons and catalogues whose
	// extension definitions the registry pass reads. gobl.dev/bundle pulls in
	// the external addon modules (mx-cfdi, pt-saft, sa-zatca, ...) so the
	// generated keys match what the public gobl.dev API accepts.
	_ "github.com/invopop/gobl"
	_ "github.com/invopop/gobl.dev/bundle"
)

func main() {
	outDir := flag.String("out", "src/gen", "directory to write the generated TypeScript into")
	version := flag.String("version", "", "GOBL version to record (defaults to the build info)")
	flag.Parse()

	if err := run(*outDir, *version); err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
}

func run(outDir, version string) error {
	if version == "" {
		version = goblVersion()
	}

	store, err := newSchemaStore()
	if err != nil {
		return err
	}

	files := map[string]string{}
	reg := newNameRegistry()

	for _, f := range store.files {
		src, err := newFileEmitter(store, f, reg).emit()
		if err != nil {
			return fmt.Errorf("%s: %w", f.Path, err)
		}
		files[f.TSPath()] = src
	}

	modules := store.Modules()
	for _, m := range modules {
		files[filepath.Join(m, "index.ts")] = emitModuleBarrel(store.ModuleFiles(m))
	}

	registryFiles, err := emitRegistry()
	if err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	for name, src := range registryFiles {
		files[filepath.Join("registry", name)] = src
	}

	files["schemas.ts"] = emitSchemaMap(store)
	files["label.ts"] = emitLabel()
	files["version.ts"] = emitVersion(version)
	files["index.ts"] = emitRootBarrel(modules, len(registryFiles) > 0)

	if err := assertOwnedTree(outDir); err != nil {
		return err
	}
	if err := os.RemoveAll(outDir); err != nil {
		return err
	}

	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		full := filepath.Join(outDir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(files[p]), 0o644); err != nil {
			return err
		}
	}

	fmt.Printf(
		"generated %d files (%d schemas, %d modules, %d registry) from GOBL %s into %s\n",
		len(files), len(store.files), len(modules), len(registryFiles), version, outDir,
	)
	return nil
}

// assertOwnedTree refuses to delete a directory that holds files this
// generator did not write, so that a mistyped -out cannot destroy hand-written
// source.
func assertOwnedTree(dir string) error {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(string(body), genHeader) {
			return fmt.Errorf(
				"refusing to wipe %s: %s was not written by this generator",
				dir, p,
			)
		}
		return nil
	})
}

// goblVersion reads the pinned GOBL version from the build info. Under a local
// replace directive this reports "(devel)", which is why -version exists.
func goblVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/invopop/gobl" {
			if dep.Replace != nil {
				return dep.Replace.Version
			}
			return dep.Version
		}
	}
	return "unknown"
}
