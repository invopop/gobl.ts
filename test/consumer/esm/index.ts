// Compiled against the packed tarball, not the source tree. This is what
// catches declaration-emit bugs: an in-repo `tsc --noEmit` checks src/, and
// will happily pass while the published .d.ts files are unusable.
import {
  GOBLClient,
  document,
  bill,
  GOBL_VERSION,
  Amount,
  Percentage,
  type org,
  type tax,
} from '@invopop/gobl';
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

// Arithmetic must survive the build: the receiver's precision wins, and a
// percentage stores a factor.
const line = Amount.parse('90.00').multiply(Amount.parse('20'));
if (line.toString() !== '1800.00') throw new Error('multiply wrong');
if (Percentage.parse('21%').of(line).toString() !== '378.00') throw new Error('percent wrong');
if (Amount.parse('100.00').add(Amount.parse('0.0001')).toString() !== '100.00') {
  throw new Error('exponent rule wrong');
}

console.log('esm consumer ok');
