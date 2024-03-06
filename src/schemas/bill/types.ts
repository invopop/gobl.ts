import { Uuid } from '../uuid/uuid';

export type InvoiceValue = {
  type?: InvoiceType;
  uuid?: Uuid;
};

export type InvoiceType =
  /**
   * A regular commercial invoice document between a supplier and customer.
   */
  | 'standard'
  /**
   * For a clients validation before sending a final invoice.
   */
  | 'proforma'
  /**
   * Corrected invoice that completely replaces the preceding document.
   */
  | 'corrective'
  /**
   * Reflects a refund either partial or complete of the preceding document.
   */
  | 'credit-note'
  /**
   * An additional set of charges to be added to the preceding document.
   */
  | 'debit-note';
