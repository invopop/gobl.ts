import { defineConfig } from 'tsup';
import { globSync } from 'node:fs';

// Every module barrel and registry data file is its own entry point so that the
// `exports` subpaths in package.json resolve, and so a consumer importing one
// addon's extension data does not pull in the rest.
const entry = [
  'src/index.ts',
  ...globSync('src/gen/*/index.ts'),
  ...globSync('src/gen/registry/**/*.ts'),
];

export default defineConfig({
  entry,
  outDir: 'dist',
  format: ['esm', 'cjs'],
  // Pinned rather than left at esbuild's `esnext` default. The published
  // JavaScript has to run under goja (Invopop's embedded scripting engine),
  // which is a separate implementation rather than a slightly older V8, so
  // letting the output drift to whatever syntax the toolchain emits next would
  // break it silently. es2020 covers everything this library uses — BigInt,
  // optional chaining, nullish coalescing — and `cmd/gojacheck` proves the
  // result actually loads. Private class fields are transpiled away by this
  // target, which also widens browser support.
  target: 'es2020',
  // Declarations come from tsc, not from tsup's dts bundler. The bundler
  // flattens `export * as bill from './bill/index.js'` into a rolled-up
  // namespace and emits its type-only members as value re-exports, so every
  // consumer typecheck failed with TS2693 ("only refers to a type, but is
  // being used as a value"). tsc understands the construct natively and its
  // per-file output mirrors src, which the exports map already matches.
  dts: false,
  clean: true,
  sourcemap: true,
  splitting: true,
  treeshake: true,
});
