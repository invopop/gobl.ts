import type { Envelope } from '../helpers.js';

/** An ES256 key pair as returned by the GOBL API, in JWK form. */
export interface Keypair {
  private: JsonWebKey;
  public: Omit<JsonWebKey, 'd'>;
}

/** Options accepted by {@link GOBLBackend.build}. */
export interface BuildOptions {
  /** Wrap the calculated document in an envelope. */
  envelop?: boolean;
  /**
   * Schema ID to tag the document with. GOBL identifies a document by its
   * `$schema`, so either set it on the document itself or pass it here.
   */
  schema?: string;
  /** Explicit document type, when it cannot be inferred from `$schema`. */
  type?: string;
  /** Partial document merged over the input before building. */
  template?: object;
}

/** Summary of a tax regime, as listed by the API. */
export interface RegimeSummary {
  country: string;
  name: Record<string, string>;
  description?: Record<string, string>;
  currency?: string;
}

/** Summary of a tax addon, as listed by the API. */
export interface AddonSummary {
  key: string;
  name: Record<string, string>;
  description?: Record<string, string>;
  requires?: string[];
}

/**
 * The GOBL operations this library exposes.
 *
 * Declared as an interface so that a WebAssembly-backed implementation can be
 * added later without changing any call site. `GOBLClient` is the HTTP
 * implementation and is the only one shipped today; for in-browser or offline
 * use, the `@invopop/gobl-worker` package already wraps the CDN-hosted GOBL
 * wasm binary.
 */
export interface GOBLBackend {
  /** Parses, calculates and validates a document, optionally enveloping it. */
  build<T extends object>(doc: T, opts?: BuildOptions): Promise<Envelope<T>>;
  /** Builds a document, envelopes it and signs it with the given private key. */
  sign<T extends object>(doc: T, privatekey: JsonWebKey): Promise<Envelope<T>>;
  /** Validates a document or envelope without modifying it. */
  validate(doc: object): Promise<void>;
  /** Verifies an envelope's signatures. */
  verify(doc: object, publickey?: JsonWebKey): Promise<void>;
  /** Produces a corrective document, such as a credit note. */
  correct<T extends object>(doc: object, options?: object): Promise<Envelope<T>>;
  /** Fetches the JSON Schema describing a document's correction options. */
  correctionOptionsSchema(doc: object): Promise<Record<string, unknown> | null>;
  /** Copies a document with a fresh UUID and no signatures. */
  replicate<T extends object>(doc: object): Promise<Envelope<T>>;
  /** Generates a new ES256 key pair for signing. */
  keygen(): Promise<Keypair>;
  /** Lists the `$id` of every schema this backend knows. */
  schemas(): Promise<string[]>;
  /** Fetches one JSON Schema by path, e.g. `"bill/invoice"`. */
  schema(path: string, opts?: { bundle?: boolean }): Promise<Record<string, unknown>>;
  /** Lists the available tax regimes. */
  regimes(): Promise<RegimeSummary[]>;
  /** Fetches one tax regime definition by country code. */
  regime(code: string): Promise<Record<string, unknown>>;
  /** Lists the available tax addons. */
  addons(): Promise<AddonSummary[]>;
  /** Fetches one addon definition by key. */
  addon(key: string): Promise<Record<string, unknown>>;
}
