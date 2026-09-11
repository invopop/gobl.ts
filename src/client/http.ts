import type { Envelope } from '../helpers.js';
import { GOBLError } from './errors.js';
import type { AddonSummary, BuildOptions, GOBLBackend, Keypair, RegimeSummary } from './backend.js';

/** The public GOBL API. */
export const DEFAULT_BASE_URL = 'https://gobl.dev/v0';

/** Options accepted by the {@link GOBLClient} constructor. */
export interface GOBLClientOptions {
  /**
   * Base URL of the GOBL API. Point this at a same-origin proxy to add
   * authentication, or at a locally running `gobl serve`.
   */
  baseUrl?: string;
  /** Replacement fetch implementation, for tests or for custom transports. */
  fetch?: typeof globalThis.fetch;
  /** Extra headers sent with every request. */
  headers?: Record<string, string>;
}

/**
 * A typed client for the GOBL HTTP API.
 *
 * Documents and envelopes are sent as inline JSON, matching the API's
 * `json.RawMessage` request fields. (The WebAssembly build uses a different,
 * base64-encoded envelope — that difference is why the two live behind the
 * shared `GOBLBackend` interface.)
 */
export class GOBLClient implements GOBLBackend {
  readonly baseUrl: string;
  readonly #fetch: typeof globalThis.fetch;
  readonly #headers: Record<string, string>;

  constructor(opts: GOBLClientOptions = {}) {
    this.baseUrl = (opts.baseUrl ?? DEFAULT_BASE_URL).replace(/\/+$/, '');
    // Bound so that passing `fetch` from a Request context keeps working.
    const impl = opts.fetch ?? globalThis.fetch;
    if (typeof impl !== 'function') {
      throw new TypeError('no fetch implementation available; pass one via options.fetch');
    }
    this.#fetch = impl.bind(globalThis);
    this.#headers = opts.headers ?? {};
  }

  async #request<T>(path: string, opts: { method?: string; body?: unknown } = {}): Promise<T> {
    const hasBody = opts.body !== undefined;
    const res = await this.#fetch(`${this.baseUrl}${path}`, {
      method: opts.method ?? 'POST',
      headers: {
        ...this.#headers,
        ...(hasBody ? { 'Content-Type': 'application/json' } : {}),
      },
      body: hasBody ? JSON.stringify(opts.body) : undefined,
    });

    if (!res.ok) {
      throw GOBLError.fromBody(await res.text(), res.status);
    }
    if (res.status === 204) {
      return undefined as T;
    }
    return (await res.json()) as T;
  }

  build<T extends object>(doc: T, opts: BuildOptions = {}): Promise<Envelope<T>> {
    return this.#request('/build', {
      body: {
        data: opts.schema ? { $schema: opts.schema, ...doc } : doc,
        envelop: opts.envelop,
        type: opts.type,
        template: opts.template,
      },
    });
  }

  sign<T extends object>(doc: T, privatekey: JsonWebKey): Promise<Envelope<T>> {
    return this.#request('/sign', { body: { data: doc, privatekey } });
  }

  async validate(doc: object): Promise<void> {
    await this.#request('/validate', { body: { data: doc } });
  }

  async verify(doc: object, publickey?: JsonWebKey): Promise<void> {
    await this.#request('/verify', { body: { data: doc, publickey } });
  }

  correct<T extends object>(doc: object, options?: object): Promise<Envelope<T>> {
    return this.#request('/correct', { body: { data: doc, options } });
  }

  correctionOptionsSchema(doc: object): Promise<Record<string, unknown> | null> {
    return this.#request('/correct', { body: { data: doc, schema: true } });
  }

  replicate<T extends object>(doc: object): Promise<Envelope<T>> {
    return this.#request('/replicate', { body: { data: doc } });
  }

  keygen(): Promise<Keypair> {
    return this.#request('/keygen');
  }

  async schemas(): Promise<string[]> {
    const res = await this.#request<{ schemas: string[] }>('/schemas', { method: 'GET' });
    return res.schemas;
  }

  schema(path: string, opts: { bundle?: boolean } = {}): Promise<Record<string, unknown>> {
    const query = opts.bundle ? '?bundle' : '';
    return this.#request(`/schemas/${path}${query}`, { method: 'GET' });
  }

  async regimes(): Promise<RegimeSummary[]> {
    const res = await this.#request<{ regimes: RegimeSummary[] }>('/regimes', { method: 'GET' });
    return res.regimes;
  }

  regime(code: string): Promise<Record<string, unknown>> {
    return this.#request(`/regimes/${code}`, { method: 'GET' });
  }

  async addons(): Promise<AddonSummary[]> {
    const res = await this.#request<{ addons: AddonSummary[] }>('/addons', { method: 'GET' });
    return res.addons;
  }

  addon(key: string): Promise<Record<string, unknown>> {
    return this.#request(`/addons/${key}`, { method: 'GET' });
  }
}
