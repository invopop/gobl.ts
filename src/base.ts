import { InvoiceValue } from './schemas/bill/types';

type GOBLSchema = InvoiceValue;

export abstract class BaseSchema<T extends GOBLSchema> {
  protected _value: T;

  SCHEMA: string = 'http://json-schema.org/draft/2020-12/schema';

  abstract SCHEMA_ID: string;

  constructor(value: T) {
    this._value = value;
  }

  abstract toJSON(): string;
}
