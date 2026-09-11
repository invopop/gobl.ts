#!/usr/bin/env node
// Mirrors the emitted .d.ts tree to .d.cts.
//
// The package is `"type": "module"`, so under nodenext resolution TypeScript
// reads a .d.ts as ESM declarations. A CommonJS `require('@invopop/gobl')`
// resolves the `require` condition and needs .d.cts alongside, or it reports
// the package as ESM-only. The declarations are structurally identical; only
// the relative specifiers change, from .js to .cjs, to match what tsup emits.
import { readdirSync, readFileSync, writeFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

let written = 0;

function convert(dir) {
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) {
      convert(path);
    } else if (entry.endsWith('.d.ts')) {
      const src = readFileSync(path, 'utf8')
        // Relative specifiers only; bare package names are left alone.
        .replace(/(from\s*['"]\.[^'"]*)\.js(['"])/g, '$1.cjs$2')
        .replace(/(import\(['"]\.[^'"]*)\.js(['"])/g, '$1.cjs$2')
        // The .d.ts declaration map does not describe this file.
        .replace(/^\/\/# sourceMappingURL=.*$/m, '');
      writeFileSync(path.replace(/\.d\.ts$/, '.d.cts'), src);
      written++;
    }
  }
}

convert('dist');
console.log(`wrote ${written} .d.cts files`);
