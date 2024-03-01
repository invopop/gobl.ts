import type { JSONSchema7 } from 'json-schema';
import { Uuid } from './schemas/uuid/uuid';
import { BaseSchema } from './base';
import { SchemaType } from './schemas/types';
import { Invoice } from './schemas/bill/invoice';

const CLASS_MAP: Record<SchemaType, typeof BaseSchema> = {
  'bill/invoice': Invoice,
  'uuid/uuid': Uuid,
};

export class GOBL {
  json: JSONSchema7;
  schema: BaseSchema;

  constructor(schemaType: SchemaType, json: JSONSchema7) {
    this.json = json;

    const schemaClass = CLASS_MAP[schemaType] as typeof schemaClass;

    if (!schemaClass) {
      throw Error('Invalid schema type.');
    }

    this.schema = new schemaClass();
  }
}
