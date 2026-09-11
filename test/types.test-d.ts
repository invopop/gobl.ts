import type { bill, tax, registry } from '../src/index.js';
import type { Calculated, Envelope } from '../src/index.js';
// Label maps are runtime values, so they need a value import rather than
// `import type`. Both forms of the same namespace must work.
import { bill as billValues } from '../src/index.js';

// A document is built from the fields a caller supplies. Everything GOBL
// computes is optional, so this minimal invoice must compile.
const minimal: bill.Invoice = {
  supplier: { name: 'Provide One' },
};
void minimal;

const invoice: bill.Invoice = {
  $regime: 'ES',
  $addons: ['es-verifactu-v1'],
  $tags: ['simplified'],
  type: 'standard',
  code: '001',
  issue_date: '2026-01-15',
  currency: 'EUR',
  supplier: { name: 'Provide One', tax_id: { country: 'ES', code: 'B85905495' } },
  lines: [{ quantity: '20', item: { name: 'Development', price: '90.00' } }],
};
void invoice;

// @ts-expect-error 'invalid' is not a member of InvoiceType.
const badType: bill.Invoice = { supplier: {}, type: 'invalid' };
void badType;

// @ts-expect-error misspelled addon keys are rejected by the generated union.
const badAddon: bill.Invoice = { supplier: {}, $addons: ['es-verifctu-v1'] };
void badAddon;

// Extension keys come from the tax regimes, addons and catalogues registered
// in this build, so typos are caught rather than silently ignored.
const ext: tax.Extensions = { 'es-verifactu-doc-type': 'F1' };
void ext;

// @ts-expect-error no such extension key.
const badExt: tax.Extensions = { 'es-verifactu-doc-typo': 'F1' };
void badExt;

// @ts-expect-error 'ZZ' is not a value of the es-verifactu-doc-type extension.
const badExtValue: tax.Extensions = { 'es-verifactu-doc-type': 'ZZ' };
void badExtValue;

// The open escape hatch stays available for keys newer than this package.
const anyExt: tax.ExtensionsAny = { 'some-future-key': 'X' };
void anyExt;

// Open enums keep autocomplete for known values while accepting others.
const knownTag: bill.InvoiceTag = 'simplified';
const customTag: bill.InvoiceTag = 'some-custom-tag';
void knownTag;
void customTag;

// After a build, the calculated fields are guaranteed present.
declare const built: Calculated<bill.Invoice, bill.InvoiceCalculatedKeys>;
const payable: bill.Totals = built.totals;
void payable;

// Envelopes narrow to the document type they carry.
declare const env: Envelope<bill.Invoice>;
const total: string | undefined = env.doc.totals?.payable;
void total;

// Label maps are keyed by the union they describe.
const label: string | undefined = billValues.InvoiceTypeLabels['credit-note'].title;
void label;

declare const catalogue: registry.CatalogueKey;
void catalogue;
