import { describe, expect, it, vi } from 'vitest';
import { GOBLClient, GOBLError, isEnvelope, DEFAULT_BASE_URL } from '../src/index.js';
import type { bill } from '../src/index.js';

/** A fetch stub returning one canned response and recording the call. */
function stubFetch(body: unknown, init: { status?: number; text?: string } = {}) {
  const calls: Array<{ url: string; init: RequestInit }> = [];
  const fetch = vi.fn(async (url: string | URL | Request, reqInit?: RequestInit) => {
    calls.push({ url: String(url), init: reqInit ?? {} });
    const status = init.status ?? 200;
    return {
      ok: status >= 200 && status < 300,
      status,
      json: async () => body,
      text: async () => init.text ?? JSON.stringify(body),
    } as Response;
  });
  return { fetch: fetch as unknown as typeof globalThis.fetch, calls };
}

const invoice: bill.Invoice = {
  supplier: { name: 'Provide One' },
  lines: [{ quantity: '1', item: { name: 'Dev', price: '90.00' } }],
};

describe('GOBLClient', () => {
  it('defaults to the public API and strips trailing slashes', () => {
    expect(new GOBLClient().baseUrl).toBe(DEFAULT_BASE_URL);
    expect(new GOBLClient({ baseUrl: 'http://localhost:8080/v0/' }).baseUrl).toBe(
      'http://localhost:8080/v0'
    );
  });

  it('posts the document inline as JSON, not base64', async () => {
    const { fetch, calls } = stubFetch({ $schema: 'x', head: {}, doc: {} });
    await new GOBLClient({ fetch }).build(invoice, { envelop: true });

    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe(`${DEFAULT_BASE_URL}/build`);
    expect(calls[0]!.init.method).toBe('POST');
    const body = JSON.parse(String(calls[0]!.init.body));
    expect(body).toMatchObject({ data: invoice, envelop: true });
  });

  it('sends extra headers, so a proxy can add authentication', async () => {
    const { fetch, calls } = stubFetch({});
    await new GOBLClient({ fetch, headers: { Authorization: 'Bearer t' } }).validate(invoice);
    expect(calls[0]!.init.headers).toMatchObject({ Authorization: 'Bearer t' });
  });

  it('unwraps the list responses', async () => {
    const { fetch } = stubFetch({ schemas: ['https://gobl.org/draft-0/bill/invoice'] });
    await expect(new GOBLClient({ fetch }).schemas()).resolves.toEqual([
      'https://gobl.org/draft-0/bill/invoice',
    ]);
  });

  it('requests the bundled form of a schema when asked', async () => {
    const { fetch, calls } = stubFetch({});
    await new GOBLClient({ fetch }).schema('bill/invoice', { bundle: true });
    expect(calls[0]!.url).toBe(`${DEFAULT_BASE_URL}/schemas/bill/invoice?bundle`);
    expect(calls[0]!.init.method).toBe('GET');
  });

  it('parses a structured error body into faults', async () => {
    const body = JSON.stringify({
      key: 'validation',
      message: 'document is invalid',
      faults: [{ code: 'GOBL-BILL-INVOICE-10', paths: ['$.lines'], message: 'lines are required' }],
    });
    const { fetch } = stubFetch(null, { status: 422, text: body });

    const err = await new GOBLClient({ fetch }).build(invoice).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(GOBLError);
    const goblErr = err as GOBLError;
    expect(goblErr.status).toBe(422);
    expect(goblErr.key).toBe('validation');
    expect(goblErr.message).toBe('document is invalid');
    expect(goblErr.faults[0]?.code).toBe('GOBL-BILL-INVOICE-10');
    expect(goblErr.faults[0]?.paths).toEqual(['$.lines']);
  });

  it('summarises faults when GOBL sends no top-level message', async () => {
    const body = JSON.stringify({
      key: 'validation',
      faults: [{ code: 'GOBL-X-01', paths: ['$.totals'], message: 'totals are required' }],
    });
    const { fetch } = stubFetch(null, { status: 422, text: body });
    const err = (await new GOBLClient({ fetch })
      .build(invoice)
      .catch((e: unknown) => e)) as GOBLError;
    expect(err.message).toBe('validation: $.totals: totals are required');
  });

  it('tags the document with a schema when one is given', async () => {
    const { fetch, calls } = stubFetch({});
    await new GOBLClient({ fetch }).build(invoice, {
      schema: 'https://gobl.org/draft-0/bill/invoice',
    });
    const body = JSON.parse(String(calls[0]!.init.body));
    expect(body.data.$schema).toBe('https://gobl.org/draft-0/bill/invoice');
  });

  it('falls back to the raw body when the error is not JSON', async () => {
    const { fetch } = stubFetch(null, { status: 502, text: 'upstream unavailable' });
    const err = (await new GOBLClient({ fetch }).keygen().catch((e: unknown) => e)) as GOBLError;
    expect(err.message).toBe('upstream unavailable');
    expect(err.status).toBe(502);
    expect(err.faults).toEqual([]);
  });
});

describe('isEnvelope', () => {
  it('distinguishes envelopes from bare documents', () => {
    expect(isEnvelope({ $schema: 'https://gobl.org/draft-0/envelope' })).toBe(true);
    expect(isEnvelope({ $schema: 'https://gobl.org/draft-0/bill/invoice' })).toBe(false);
    expect(isEnvelope(null)).toBe(false);
  });
});
