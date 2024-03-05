import { BaseSchema } from 'src/base';
import { InvoiceValue } from './types';

/**
 * @description Invoice represents a payment claim for goods or services supplied under conditions agreed between the supplier and the customer.
 */
export class Invoice extends BaseSchema<InvoiceValue> {
  /**
   * @description The Schema ID of the GOBL Invoice structure.
   */
  SCHEMA_ID: string = 'https://gobl.org/draft-0/bill/invoice';
}
