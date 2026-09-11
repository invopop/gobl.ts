import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    // Only runtime tests run here. The `.test-d.ts` files are compile-time
    // assertions (`satisfies`, `@ts-expect-error`) with no runtime body, and
    // `tsc --noEmit` already checks them via the `test/**/*` include in
    // tsconfig.json — so they are covered by `npm run typecheck` rather than
    // by vitest's still-experimental typecheck mode.
    include: ['test/**/*.test.ts'],
  },
});
