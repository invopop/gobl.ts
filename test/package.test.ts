import { describe, expect, it } from 'vitest';
import { existsSync, readFileSync } from 'node:fs';

// These guard the shape of the published package. They need `npm run build`,
// so they skip when dist is absent rather than failing a source-only run.
const built = existsSync('dist/index.js');

/** Follows a module's static relative imports and returns the source of each. */
function closure(entry: string): Map<string, string> {
  const out = new Map<string, string>();
  const visit = (file: string) => {
    if (out.has(file)) return;
    const src = readFileSync(`dist/${file}`, 'utf8');
    out.set(file, src);
    for (const m of src.matchAll(/from\s*["']\.\/([^"']+)["']/g)) visit(m[1]!);
  };
  visit(entry);
  return out;
}

describe.runIf(built)('published package', () => {
  it('keeps the bulky extension labels out of the root import', () => {
    const sources = [...closure('index.js').values()].join('');
    // ~90 KB of display strings in every language GOBL defines. Useful, but
    // only to applications that render extension pickers, so it lives on the
    // `@invopop/gobl.ts/registry/extension-keys` subpath instead.
    expect(sources).not.toContain('untdid-tax-category');
  });

  it('still ships the extension labels on their own subpath', () => {
    const src = readFileSync('dist/gen/registry/extension-keys.js', 'utf8');
    expect(src).toContain('untdid-tax-category');
  });

  it('exposes a module subpath for every GOBL package', () => {
    const modules = [
      'bill',
      'cal',
      'cbc',
      'currency',
      'dsig',
      'gobl',
      'head',
      'i18n',
      'l10n',
      'note',
      'num',
      'org',
      'pay',
      'schema',
      'tax',
    ];
    for (const m of modules) {
      expect(existsSync(`dist/gen/${m}/index.js`), `${m} entry`).toBe(true);
      expect(existsSync(`dist/gen/${m}/index.d.ts`), `${m} types`).toBe(true);
    }
  });

  it('ships both ESM and CommonJS entry points', () => {
    for (const f of ['index.js', 'index.cjs', 'index.d.ts', 'index.d.cts']) {
      expect(existsSync(`dist/${f}`), f).toBe(true);
    }
  });
});
