import { BaseSchema } from 'src/base';
import { Uuid } from '../uuid/uuid';
import { InvoiceType } from './types';

/**
 * @description Invoice represents a payment claim for goods or services supplied under conditions agreed between the supplier and the customer.
 */
export class Invoice extends BaseSchema {
  /**
   * @description The Schema ID of the GOBL Invoice structure.
   */
  SCHEMA_ID: string = 'https://gobl.org/draft-0/bill/invoice';

  /**
   * @description Unique document ID. Not required, but always recommended in addition to the Code.
   */
  uuid: Uuid | undefined;

  /**
   * @description Type of invoice document subject to the requirements of the local tax regime.
   */
  type: InvoiceType | undefined;
}
