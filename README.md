# GOBL for TypeScript

TypeScript types and a typed API client for [GOBL](https://gobl.org), the Go
Business Language.

The model types are **generated** from GOBL's own JSON Schemas, so they track
the upstream definitions exactly rather than drifting by hand.

```bash
npm install @invopop/gobl
```

Ships ESM and CommonJS builds with full type declarations for both. Node 18+.

## Usage

```ts
import { GOBLClient, document, type bill } from '@invopop/gobl';

const gobl = new GOBLClient(); // https://gobl.dev/v0

const invoice = document('https://gobl.org/draft-0/bill/invoice', {
  $regime: 'ES',
  $addons: ['es-verifactu-v1'],
  code: '004',
  currency: 'EUR',
  supplier: { name: 'Provide One S.L.', tax_id: { country: 'ES', code: 'B98602642' } },
  lines: [
    {
      quantity: '20',
      item: { name: 'Development services', price: '90.00' },
      taxes: [{ cat: 'VAT', rate: 'standard' }],
    },
  ],
});

const env = await gobl.build(invoice, { envelop: true });
console.log(env.doc.totals?.payable);
```

See [`examples/invoice.ts`](examples/invoice.ts) for a complete build-and-sign flow.

### Types

Types are grouped into one namespace per GOBL package, because several names
(`Amount`, `Code`, `Identity`, `Note`) exist in more than one:

```ts
import type { bill, cbc, num, org, tax } from '@invopop/gobl';

const price: num.Amount = '90.00';
const country: tax.RegimeCode = 'ES';
const party: org.Party = { name: 'Provide One' };
```

Each package is also its own entry point, if you prefer:

```ts
import type * as bill from '@invopop/gobl/bill';
```

**Calculated fields are optional.** GOBL populates totals, tax breakdowns and
similar during a build, so they are optional on input. Pair a document with its
generated key list to get the read-side view:

```ts
import type { Calculated, bill } from '@invopop/gobl';

type BuiltInvoice = Calculated<bill.Invoice, bill.InvoiceCalculatedKeys>;
```

**Enums carry their labels.** Every generated union ships a label map, so you
can build a form control without re-reading the schema:

```ts
import { bill } from '@invopop/gobl';

Object.entries(bill.InvoiceTypeLabels).map(([value, { title }]) => ({ value, title }));
```

**Extension keys are typed.** Keys and their value domains come from every tax
regime, addon and catalogue registered in this build, so typos are caught at
compile time:

```ts
const ext: tax.Extensions = { 'es-verifactu-doc-type': 'F1' };
```

Use `tax.ExtensionsAny` when working with documents that may carry keys from a
newer GOBL release than this package was generated against.

### Client

`GOBLClient` covers the whole [gobl.dev](https://gobl.dev) API: `build`, `sign`,
`validate`, `verify`, `correct`, `correctionOptionsSchema`, `replicate`,
`keygen`, `schemas`, `schema`, `regimes`, `regime`, `addons` and `addon`.

```ts
new GOBLClient({
  baseUrl: 'https://example.com/gobl', // e.g. a proxy that adds authentication
  headers: { Authorization: 'Bearer …' },
  fetch: customFetch,
});
```

Validation failures throw a `GOBLError` carrying GOBL's structured faults:

```ts
catch (err) {
  if (err instanceof GOBLError) {
    for (const f of err.faults) console.error(f.paths, f.message, f.code);
  }
}
```

#### WebAssembly

The client is an implementation of the `GOBLBackend` interface, so a
WebAssembly backend can be added later without changing call sites. For
in-browser or offline use today, [`@invopop/gobl-worker`](https://www.npmjs.com/package/@invopop/gobl-worker)
already wraps the GOBL wasm binary distributed via `cdn.gobl.org`.

## Versioning

The package version tracks GOBL's `major.minor`, with the patch reserved for
changes here — the same convention `gobl.dev` and `@invopop/gobl-worker` use.
`0.505.x` is generated from GOBL `v0.505.0`, which `GOBL_VERSION` also reports
at runtime. `npm run check-version` enforces this, and CI runs it, so the
package can never claim a GOBL version it was not generated from.

## Releasing

Releases are automatic. Every push to `main` derives the next version, tags it,
and publishes to npm — so bumping the GOBL dependency and merging is the whole
release process.

The version is a function of `go.mod` plus the existing tags: `major.minor`
follows the GOBL release the types were generated from, and the patch is ours.

```
GOBL 0.505.x  ->  v0.505.0, v0.505.1, v0.505.2, ...
GOBL 0.506.0  ->  v0.506.0, v0.506.1, ...
```

A GOBL patch release is absorbed into the next patch here, so the exact core
version is not recoverable from the tag name alone. It is recorded in the
annotated tag, in the GitHub Release notes, and in `GOBL_VERSION` at runtime.

To see what the next push would release:

```bash
make next-version
```

To push to `main` without releasing, include `[skip release]` in the commit
message. `workflow_dispatch` triggers a release manually — leave the tag input
empty to derive the next version, or name an existing tag to retry a failed
publish.

Publishing uses npm **OIDC trusted publishing**, so there is no long-lived
token. The workflow regenerates the types and fails on drift, stamps the
version from the tag, runs the full suite, then packs a tarball and installs it
into a throwaway project to confirm the published declarations actually
compile — in both ESM and CommonJS — before anything reaches the registry.

A prerelease tag (`v0.506.0-rc.1`) publishes under a matching npm dist-tag
(`rc`) rather than `latest`.

## Development

The generator is a Go program that reads the JSON Schemas embedded in the
pinned `github.com/invopop/gobl` module. Output depends only on `go.mod`, so no
sibling checkout is needed and regeneration is reproducible.

```bash
make generate    # rewrite src/gen from the pinned GOBL version
make typecheck
make test
make test-live   # exercise the public gobl.dev API
make build
```

`src/gen` is committed and entirely generator-owned; never edit it by hand. CI
regenerates and fails on any diff, so a stale tree cannot ship.

To upgrade GOBL, bump the dependency and regenerate — or let the weekly
`Upgrade GOBL` workflow open the pull request for you. Merging it releases the
new version automatically:

```bash
go get github.com/invopop/gobl@latest && go mod tidy && make generate
```

To work against unreleased schemas, uncomment the `replace` directive in
`go.mod` and pass the version explicitly:

```bash
make generate VERSION=v0.506.0-dev
```

### Tests

- `test/fixtures.test-d.ts` type-checks **133 real envelopes** from GOBL's own
  example suite, covering 24 countries and every addon. Because `satisfies`
  applies excess property checking, this fails both when a type is missing a
  property GOBL emits and when an enum is narrowed too far. Refresh with
  `make fixtures` against a local GOBL checkout.
- `test/types.test-d.ts` asserts the hand-written ergonomics: optionality of
  calculated fields, extension key and value narrowing, open enums.
- `test/client.test.ts` covers the client against a stub fetch.
- `test/live.test.ts` runs against the real API (`make test-live`).
- `.github/scripts/next-version.test.sh` covers release-number derivation
  against synthetic tag histories — an off-by-one there either skips a version
  or tries to republish one npm has already taken.
- `npm run verify-package` packs the tarball and consumes it from a throwaway
  project in both ESM and CommonJS. In-repo type checking only sees `src/`, so
  this is the only check that exercises the **published** declarations — it is
  what caught tsup's dts bundler flattening `export * as bill` into value
  re-exports, which made every consumer typecheck fail while everything in this
  repo passed. Declarations are emitted by `tsc` for that reason.

## Known upstream issues

- `schemas/bill/delivery.json` declares `"enum": "advice"` — a bare string
  where JSON Schema requires an array, from a `jsonschema_extras` tag in
  `bill/delivery.go` that collapses to its first item. The generator ignores
  `enum` and reads the correct values from the sibling `oneOf`.
- `GET /v0/schemas` lists `regimes/mx/food-vouchers` and
  `regimes/mx/fuel-account-balance`, but `GET /v0/schemas/{path}` returns 404
  for both: they are registered by the external `gobl.mx.cfdi` module whose
  JSON is not in the embedded data. No schema is published for them, so no
  types are generated.
- `gobl.dev`'s published OpenAPI document describes error faults as
  `{properties, description}`. The wire format is `{code, message, paths}`,
  which is what `GOBLError` implements.

## License

[Apache 2.0](LICENSE)
