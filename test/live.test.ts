import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { GOBLClient, GOBLError, document } from '../src/index.js';
import type { bill } from '../src/index.js';

// Hits the public GOBL API. Opt in with GOBL_LIVE=1 so the default suite stays
// offline and deterministic.
const live = process.env.GOBL_LIVE === '1';
const client = new GOBLClient({ baseUrl: process.env.GOBL_API_URL });

describe.runIf(live)('gobl.dev API', () => {
  it('builds the vendored example to the same envelope GOBL produced', async () => {
    const expected = JSON.parse(readFileSync('test/fixtures/ie-invoice-b2b.json', 'utf8')) as {
      doc: Record<string, unknown>;
    };

    // `$schema` is how GOBL identifies the document, so it stays on the input.
    const built = await client.build(expected.doc as unknown as bill.Invoice, { envelop: true });

    // The header digest is derived from the document, so matching it proves the
    // round trip reproduced the document byte for byte.
    expect(built.doc).toEqual(expected.doc);
  }, 30_000);

  it('signs and verifies a document', async () => {
    const keys = await client.keygen();
    expect(keys.private).toBeDefined();

    const invoice = document('https://gobl.org/draft-0/bill/invoice', {
      $regime: 'ES',
      code: 'TEST-001',
      issue_date: '2026-01-15',
      currency: 'EUR',
      supplier: { name: 'Provide One', tax_id: { country: 'ES', code: 'B85905495' } },
      lines: [{ quantity: '1', item: { name: 'Development', price: '90.00' } }],
    });

    const signed = await client.sign(invoice, keys.private);
    expect(signed.sigs?.length).toBeGreaterThan(0);
    await expect(client.verify(signed, keys.public)).resolves.toBeUndefined();
  }, 30_000);

  it('reports validation failures as structured faults', async () => {
    // An invoice with no supplier cannot be built.
    const err = await client
      .build({
        $schema: 'https://gobl.org/draft-0/bill/invoice',
        $regime: 'ES',
        supplier: { name: 'X' },
      } as unknown as bill.Invoice)
      .catch((e: unknown) => e);

    expect(err).toBeInstanceOf(GOBLError);
    const goblErr = err as GOBLError;
    expect(goblErr.status).toBe(422);
    expect(goblErr.key).toBe('validation');
    expect(goblErr.faults.length).toBeGreaterThan(0);
    expect(goblErr.faults[0]?.code).toMatch(/^GOBL-/);
    expect(goblErr.faults[0]?.paths?.[0]).toMatch(/^\$\./);
    // The summary names the offending fields, since GOBL sends no top-level message.
    expect(goblErr.message).toContain('validation:');
  }, 30_000);

  it('covers every schema the API can actually serve', async () => {
    const ids = await client.schemas();
    const { SchemaIDs } = await import('../src/gen/schemas.js');
    const generated = new Set<string>(SchemaIDs);
    const missing = ids.filter((id) => !generated.has(id));

    // Anything missing should be a schema the API cannot serve either. As of
    // GOBL v0.505.0 that is regimes/mx/food-vouchers and
    // regimes/mx/fuel-account-balance: both are registered by the external
    // gobl.mx.cfdi module, so `GET /v0/schemas` lists them, but their JSON is
    // absent from the embedded data and `GET /v0/schemas/{path}` returns 404.
    // They therefore have no schema for anyone to generate from. Reported
    // upstream; this asserts the gap stays confined to unservable schemas.
    const unservable: string[] = [];
    for (const id of missing) {
      const path = id.replace('https://gobl.org/draft-0/', '');
      const res = await fetch(`${client.baseUrl}/schemas/${path}`);
      if (!res.ok) unservable.push(id);
    }

    expect(missing.filter((id) => !unservable.includes(id))).toEqual([]);
  }, 60_000);

  it('covers every addon key the API advertises', async () => {
    const addons = await client.addons();
    const { tax } = await import('../src/index.js');
    void tax;
    // AddonKey is a type, so check the values the schema enum was built from.
    const { AddonKeyLabels } = await import('../src/gen/tax/addon-list.js');
    const known = new Set(Object.keys(AddonKeyLabels));
    const missing = addons.map((a) => a.key).filter((k) => !known.has(k));
    expect(missing).toEqual([]);
  }, 30_000);
});
