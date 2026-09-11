/**
 * Builds and signs an invoice against the public GOBL API.
 *
 *   npx tsx examples/invoice.ts
 */
import { GOBLClient, GOBLError, document, type bill, type Calculated } from '../src/index.js';

const gobl = new GOBLClient();

// `document` tags the invoice with the schema ID GOBL uses to identify it, and
// types the body against that schema: pass a field that bill.Invoice does not
// declare, or a `type` outside its enum, and this will not compile.
const invoice = document('https://gobl.org/draft-0/bill/invoice', {
  $regime: 'ES',
  $addons: ['es-verifactu-v1'],
  type: 'standard',
  series: 'SAMPLE',
  code: '004',
  issue_date: '2026-01-15',
  currency: 'EUR',
  supplier: {
    name: 'Provide One S.L.',
    tax_id: { country: 'ES', code: 'B98602642' },
    addresses: [{ street: 'Calle Mayor 1', locality: 'Madrid', code: '28013', country: 'ES' }],
  },
  customer: {
    name: 'Sample Consumer',
    tax_id: { country: 'ES', code: '54387763P' },
  },
  lines: [
    {
      quantity: '20',
      item: { name: 'Development services', price: '90.00' },
      taxes: [{ cat: 'VAT', rate: 'standard' }],
    },
  ],
});

try {
  // Calculated fields (totals, tax breakdowns) are optional on input because
  // GOBL fills them in; after a build they are guaranteed present.
  const env = await gobl.build(invoice, { envelop: true });
  const built = env.doc as Calculated<bill.Invoice, bill.InvoiceCalculatedKeys>;

  console.log(`invoice ${built.series}-${built.code}`);
  console.log(`  total   ${built.totals.total} ${built.currency}`);
  console.log(`  payable ${built.totals.payable} ${built.currency}`);
  console.log(`  digest  ${env.head?.dig?.val?.slice(0, 16)}...`);

  const keys = await gobl.keygen();
  const signed = await gobl.sign(invoice, keys.private);
  console.log(`  signed with ${signed.sigs?.length} signature(s)`);
} catch (err) {
  if (err instanceof GOBLError) {
    console.error(`GOBL rejected the document (${err.status} ${err.key ?? ''})`);
    for (const fault of err.faults) {
      console.error(`  ${fault.paths?.join(', ') ?? '?'}: ${fault.message} [${fault.code}]`);
    }
    process.exit(1);
  }
  throw err;
}
