import { BaseSchema } from 'src/base';
import type { InvoiceType, InvoiceValue } from './types';
import { Uuid } from '../uuid/uuid';
import { getProps } from 'src/helpers';

/**
 * @description Invoice represents a payment claim for goods or services supplied under conditions agreed between the supplier and the customer.
 */
export class Invoice extends BaseSchema<InvoiceValue> {
  /**
   * @description The Schema ID of the GOBL Invoice structure.
   */
  SCHEMA_ID: string = 'https://gobl.org/draft-0/bill/invoice';

  public constructor(value: InvoiceValue) {
    super(value);
  }

  /**
   * @description Type of invoice document subject to the requirements of the local tax regime.
   */
  get type(): InvoiceType | undefined {
    return this._value.type;
  }

  set type(value: InvoiceType) {
    this._value.type = value;
  }

  /**
   * @description Unique document ID. Not required, but always recommended in addition to the Code.
   */
  get uuid(): Uuid | undefined {
    return this._value.uuid;
  }

  set uuid(value: Uuid) {
    this._value.uuid = value;
  }

  /**
   * @description Casts root value to JSON.
   */
  public toJSON(): string {
    return JSON.stringify(this._value);
  }

  /**
   * @description Generates a new Invoice instance from a JSON.
   */
  public static fromJSON(json: string): Invoice {
    // Map of types for properly building the _value Object.
    const casts: Record<string, any> = {
      type: 'string',
      uuid: Uuid,
    };

    const props = getProps<InvoiceValue>(JSON.parse(json) as InvoiceValue, casts);

    return new this(props);
  }
}
