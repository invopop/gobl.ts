import { Uuid } from '../uuid/uuid';

export type InvoiceValue = {
  /**
   * @description Type of invoice document subject to the requirements of the local tax regime.
   */
  type?: InvoiceType;
  /**
   * @description Unique document ID. Not required, but always recommended in addition to the Code.
   */
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
