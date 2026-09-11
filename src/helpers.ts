import type * as gobl from './gen/gobl/index.js';
import type * as schema from './gen/schema/index.js';
import type { SchemaID, SchemaTypes } from './gen/schemas.js';

/**
 * A GOBL envelope wrapping a known document type.
 *
 * The generated `gobl.Envelope` is already generic over its `doc`; this alias
 * exists so the common case reads naturally at call sites:
 *
 * ```ts
 * const env: Envelope<bill.Invoice> = await client.build(invoice, { envelop: true });
 * env.doc.totals?.payable;
 * ```
 *
 * The enveloped document carries its own `$schema`, which the document
 * interfaces themselves do not declare, so it is added here.
 */
export type Envelope<T extends object = schema.Object> = gobl.Envelope<T & { $schema?: string }>;

/**
 * Marks the properties GOBL computes as present.
 *
 * Calculated properties are optional on every generated interface, because a
 * caller constructing a document must be able to omit them. After a build they
 * are always populated, so pair a document type with its generated
 * `<Name>CalculatedKeys` to get the read-side view:
 *
 * ```ts
 * type BuiltInvoice = Calculated<bill.Invoice, bill.InvoiceCalculatedKeys>;
 * const totals = built.totals; // no longer possibly undefined
 * ```
 */
export type Calculated<T, K extends keyof T> = T & { [P in K]-?: NonNullable<T[P]> };

/**
 * A document carrying the `$schema` GOBL uses to identify it.
 *
 * The generated interfaces deliberately omit `$schema`, because the JSON
 * Schemas do not declare it, but GOBL needs it to know what it is being given.
 */
export type WithSchema<T extends object, K extends string = string> = T & { $schema: K };

/** A bare GOBL document of unknown type. */
export type Document = WithSchema<{ [key: string]: unknown }>;

/**
 * Tags a document with its schema ID, ready to send to a GOBL backend.
 *
 * The schema ID drives the document type, so a mismatch is a compile error:
 *
 * ```ts
 * const doc = document('https://gobl.org/draft-0/bill/invoice', {
 *   supplier: { name: 'Provide One' },
 * });
 * ```
 */
export function document<K extends SchemaID>(
  schema: K,
  doc: SchemaTypes[K]
): WithSchema<SchemaTypes[K] & object, K> {
  return { ...(doc as object), $schema: schema } as WithSchema<SchemaTypes[K] & object, K>;
}

/** Reports whether a parsed object is a GOBL envelope rather than a bare document. */
export function isEnvelope(data: unknown): data is Envelope {
  return (
    typeof data === 'object' &&
    data !== null &&
    (data as { $schema?: unknown }).$schema === EnvelopeSchemaID
  );
}

/** The `$schema` value identifying a GOBL envelope. */
export const EnvelopeSchemaID = 'https://gobl.org/draft-0/envelope';
