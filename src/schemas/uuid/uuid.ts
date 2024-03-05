import { ValueObject } from 'src/vo';

/**
 * @description Universally Unique Identifier. We only recommend using versions 1 and 4 within GOBL.
 */
export class Uuid extends ValueObject<string> {
  /**
   * @description The Schema ID of the GOBL UUID structure.
   */
  SCHEMA_ID: string = 'https://gobl.org/draft-0/uuid/uuid';
}
