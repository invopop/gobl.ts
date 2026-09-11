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
  dts: true,
  clean: true,
  sourcemap: true,
  splitting: true,
  treeshake: true,
});
