// Compiled against the packed tarball, not the source tree. This is what
// catches declaration-emit bugs: an in-repo `tsc --noEmit` checks src/, and
// will happily pass while the published .d.ts files are unusable.
import { GOBLClient, document, bill, GOBL_VERSION, type org, type tax } from '@invopop/gobl';
import type * as billSub from '@invopop/gobl/bill';
import { ExtensionKeyLabels } from '@invopop/gobl/registry/extension-keys';

const party: org.Party = { name: 'Provide One' };
const regime: tax.RegimeCode = 'ES';
const invoice: billSub.Invoice = { supplier: party, $regime: regime };

const doc = document('https://gobl.org/draft-0/bill/invoice', invoice);

// @ts-expect-error the extension key union must still reject typos downstream
const badExt: tax.Extensions = { 'es-verifactu-doc-typo': 'F1' };
void badExt;

if (!GOBL_VERSION.startsWith('v')) throw new Error('GOBL_VERSION missing');
if (Object.keys(bill.InvoiceTypeLabels).length !== 6) throw new Error('label map wrong');
if (Object.keys(ExtensionKeyLabels).length < 100) throw new Error('extension labels missing');
if (doc.$schema !== 'https://gobl.org/draft-0/bill/invoice') throw new Error('document() wrong');
if (new GOBLClient().baseUrl !== 'https://gobl.dev/v0') throw new Error('client wrong');

console.log('esm consumer ok');
