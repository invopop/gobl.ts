import GOBL from '@invopop/gobl.ts';

const uuid = new GOBL.Uuid.Uuid('550e8400-e29b-41d4-a716-446655440000');
const invoice = new GOBL.Bill.Invoice({ type: 'standard', uuid });

console.log(invoice.SCHEMA_ID);
console.log(invoice.type);
invoice.type = 'corrective';
console.log(invoice.toJSON());
