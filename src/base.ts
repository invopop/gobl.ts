import { InvoiceValue } from './schemas/bill/types';

type GOBLSchema = InvoiceValue;

export abstract class BaseSchema<T extends GOBLSchema> {
  private _value: T;

  SCHEMA: string = 'http://json-schema.org/draft/2020-12/schema';

  abstract SCHEMA_ID: string;

  constructor(value: T) {
    this._value = value;

    // Create a Proxy to intercept public property access
    return new Proxy(this, {
      get: (target, property: string) => {
        if (property in target) {
          return target[property];
        }

        if (!target._value) {
          console.warn(`Object is not intialized.`);
          return undefined;
        }

        if (property in target._value) {
          return target._value[property];
        }

        console.warn(`Property '${property}' doesn't exist.`);
        return undefined;
      },
    });
  }

  public toJSON(): string {
    return JSON.stringify(this._value);
  }
}
