import GOBL from '@invopop/gobl.ts';

const uuid = new GOBL.Uuid.Uuid('550e8400-e29b-41d4-a716-446655440000');
const invoice = new GOBL.Bill.Invoice({ type: 'standard', uuid });

const invoice2 = GOBL.Bill.Invoice.fromJSON(
  '{"type":"credit-note","uuid":"550e8400-e29b-41d4-a716-446655440001"}'
);

console.log(invoice.SCHEMA_ID);
console.log(invoice.type);
invoice.type = 'corrective';
console.log(invoice.toJSON());
console.log(invoice2);
invoice.callWasmMethod();
