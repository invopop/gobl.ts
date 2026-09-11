#!/usr/bin/env node
// Enforces the versioning convention: the package's major.minor tracks the
// GOBL release the types were generated from, with the patch reserved for
// changes here. Same scheme as gobl.dev and @invopop/gobl-worker.
//
// This runs in CI so the convention is checked rather than merely documented:
// publishing 0.505.x built from GOBL v0.504.0 would be silently misleading.
import { readFileSync } from 'node:fs';

const pkg = JSON.parse(readFileSync('package.json', 'utf8'));

const versionTs = readFileSync('src/gen/version.ts', 'utf8');
const match = versionTs.match(/GOBL_VERSION = '([^']+)'/);
if (!match) {
  console.error('could not read GOBL_VERSION from src/gen/version.ts');
  process.exit(1);
}

const gobl = match[1].replace(/^v/, '');
const [pkgMajor, pkgMinor] = pkg.version.split('.');
const [goblMajor, goblMinor] = gobl.split('.');

if (pkgMajor !== goblMajor || pkgMinor !== goblMinor) {
  console.error(
    `version mismatch: package.json is ${pkg.version} but the types were ` +
      `generated from GOBL v${gobl}.\n` +
      `The package major.minor must track GOBL's, so this should be ` +
      `${goblMajor}.${goblMinor}.x — run \`npm version ${goblMajor}.${goblMinor}.0 ` +
      `--no-git-tag-version\`, or regenerate if GOBL moved.`
  );
  process.exit(1);
}

console.log(`${pkg.name}@${pkg.version} matches GOBL v${gobl}`);
