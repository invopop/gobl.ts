/**
 * GOBL for TypeScript.
 *
 * Model types are generated from the GOBL JSON Schemas and exposed as one
 * namespace per GOBL package, because several type names (`Amount`, `Code`,
 * `Identity`, `Note`) exist in more than one:
 *
 * ```ts
 * import { GOBLClient, type bill, type org } from '@invopop/gobl';
 *
 * const invoice: bill.Invoice = {
 *   supplier: { name: 'Provide One' } as org.Party,
 *   lines: [{ item: { name: 'Development', price: '90.00' }, quantity: '20' }],
 * };
 *
 * const env = await new GOBLClient().build(invoice, { envelop: true });
 * ```
 */
export * from './gen/index.js';
export * from './client/index.js';
export * from './helpers.js';
