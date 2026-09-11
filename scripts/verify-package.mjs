#!/usr/bin/env node
// Packs the package and consumes it from a throwaway project, exactly as a
// user would: install the tarball, typecheck against the published .d.ts /
// .d.cts, and run the result.
//
// This exists because `tsc --noEmit` in this repo checks src/ and cannot see
// declaration-emit bugs. It caught tsup's dts bundler mangling
// `export * as bill` into value re-exports, which made every consumer
// typecheck fail (TS2693) while everything in-repo passed.
//
//   node scripts/verify-package.mjs
import { execFileSync } from 'node:child_process';
import { mkdtempSync, writeFileSync, cpSync, readdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const root = process.cwd();
const run = (cmd, args, cwd) =>
  execFileSync(cmd, args, { cwd, stdio: 'inherit', shell: process.platform === 'win32' });

console.log('> packing');
for (const f of readdirSync(root)) {
  if (f.startsWith('invopop-gobl-') && f.endsWith('.tgz')) rmSync(join(root, f));
}
execFileSync('npm', ['pack'], { cwd: root, stdio: 'pipe' });
const tarball = readdirSync(root).find((f) => f.startsWith('invopop-gobl-') && f.endsWith('.tgz'));
if (!tarball) throw new Error('npm pack produced no tarball');
const tarballPath = resolve(root, tarball);

const tsconfig = {
  compilerOptions: {
    module: 'nodenext',
    moduleResolution: 'nodenext',
    target: 'es2022',
    lib: ['es2022', 'dom'],
    strict: true,
    noEmit: true,
    skipLibCheck: false, // the point is to check the published declarations
  },
};

for (const [kind, type, entry] of [
  ['esm', 'module', 'index.ts'],
  ['cjs', 'commonjs', 'index.cts'],
]) {
  const dir = mkdtempSync(join(tmpdir(), `gobl-consumer-${kind}-`));
  try {
    writeFileSync(
      join(dir, 'package.json'),
      JSON.stringify({ name: `consumer-${kind}`, version: '1.0.0', type, private: true }, null, 2)
    );
    writeFileSync(join(dir, 'tsconfig.json'), JSON.stringify(tsconfig, null, 2));
    cpSync(join(root, 'test/consumer', kind, entry), join(dir, entry));

    console.log(`> ${kind}: installing`);
    run(
      'npm',
      ['install', '--silent', '--no-audit', '--no-fund', tarballPath, 'typescript@5', 'tsx@4'],
      dir
    );

    console.log(`> ${kind}: typecheck`);
    run(join(dir, 'node_modules/.bin/tsc'), ['--noEmit'], dir);

    console.log(`> ${kind}: run`);
    run(join(dir, 'node_modules/.bin/tsx'), [entry], dir);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

rmSync(tarballPath, { force: true });
console.log('package verified against the published artifact');
