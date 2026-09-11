import type { Fault } from '../gen/bill/fault.js';

/**
 * One validation failure reported by GOBL.
 *
 * This is GOBL's own `bill.Fault` type, so it stays in step with the schemas
 * rather than drifting. Note that `gobl.dev`'s published OpenAPI document
 * describes an older shape (`properties`/`description`); the wire format is
 * `{code, message, paths}`, where `paths` are JSON paths into the envelope,
 * e.g. `"$.supplier.tax_id"`.
 */
export type GOBLFault = Fault;

/** The error body returned by the GOBL API. */
export interface GOBLErrorBody {
  /** GOBL's error key, e.g. `"validation"` or `"unknown-schema"`. */
  key?: string;
  message?: string;
  faults?: GOBLFault[];
}

/**
 * An error returned by a GOBL backend.
 *
 * Validation problems arrive as a structured body, so this parses it rather
 * than carrying an opaque string: `faults` is what a UI needs in order to
 * highlight the offending fields.
 */
export class GOBLError extends Error {
  /** HTTP status code, or 0 when the failure was not an HTTP response. */
  readonly status: number;
  /** GOBL's own error key, e.g. `"validation"`. */
  readonly key?: string;
  readonly faults: GOBLFault[];
  /** The raw response body, retained for diagnostics. */
  readonly body?: string;

  constructor(
    message: string,
    opts: { status?: number; key?: string; faults?: GOBLFault[]; body?: string } = {}
  ) {
    super(message);
    this.name = 'GOBLError';
    this.status = opts.status ?? 0;
    this.key = opts.key;
    this.faults = opts.faults ?? [];
    this.body = opts.body;
  }

  /** Builds a GOBLError from a response body, falling back to the raw text. */
  static fromBody(body: string, status: number): GOBLError {
    let parsed: GOBLErrorBody | undefined;
    try {
      parsed = JSON.parse(body) as GOBLErrorBody;
    } catch {
      // Not JSON: keep the raw text as the message.
    }
    if (!parsed || typeof parsed !== 'object') {
      return new GOBLError(body || `request failed with status ${status}`, { status, body });
    }
    return new GOBLError(describe(parsed, status), {
      status,
      key: parsed.key,
      faults: parsed.faults,
      body,
    });
  }
}

/**
 * Produces a readable message. A validation response carries no top-level
 * message, only the faults, so summarise those rather than reporting a bare
 * status code.
 */
function describe(body: GOBLErrorBody, status: number): string {
  if (body.message) return body.message;

  const faults = body.faults ?? [];
  if (faults.length > 0) {
    const summary = faults
      .map((f) => {
        const where = f.paths?.length ? `${f.paths.join(', ')}: ` : '';
        return `${where}${f.message ?? f.code}`;
      })
      .join('; ');
    return body.key ? `${body.key}: ${summary}` : summary;
  }

  if (body.key) return body.key;
  return `request failed with status ${status}`;
}
