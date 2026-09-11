// The namespace re-export chain is load-bearing for the whole public API:
// `export * as bill` in the generated barrel must survive
// `import type { bill }` and support `bill.Invoice` as a type. Several GOBL
// type names (Amount, Code, Identity, Note) exist in two or three packages, so
// a flat export surface is impossible and this pattern has no fallback.
import type { bill, cbc, currency, l10n, num, org, tax } from '../src/index.js';

const invoice: bill.Invoice = {
  supplier: { name: 'Provide One' },
};
void invoice;

// Names that collide across packages must stay distinct.
const amounts: [num.Amount, currency.Amount] = ['90.00', { currency: 'EUR', value: '90.00' }];
const codes: [cbc.Code, currency.Code, l10n.Code] = ['ABC', 'EUR', 'es'];
const identities: [org.Identity, tax.Identity] = [{ code: 'X' }, { country: 'ES' }];
void amounts;
void codes;
void identities;
