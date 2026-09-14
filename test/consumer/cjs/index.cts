// Compiled only inside the throwaway project created by
// scripts/verify-package.mjs, against the installed tarball — never by this
// repo's own tsconfig, which excludes test/consumer.
//
// The CommonJS half: the package is "type": "module", so this exercises the
// `require` export condition and the generated .d.cts declarations.
import { GOBLClient, bill, GOBL_VERSION, Amount, type org } from '@invopop/gobl';

const party: org.Party = { name: 'Provide One' };

if (!GOBL_VERSION.startsWith('v')) throw new Error('GOBL_VERSION missing');
if (Object.keys(bill.InvoiceTypeLabels).length !== 6) throw new Error('label map wrong');
if (new GOBLClient().baseUrl !== 'https://gobl.dev/v0') throw new Error('client wrong');
if (party.name !== 'Provide One') throw new Error('types wrong');
if (Amount.parse('1.01').multiply(Amount.parse('1.01')).toString() !== '1.02') {
  throw new Error('arithmetic wrong');
}

console.log('cjs consumer ok');
