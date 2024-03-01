import { BaseSchema } from 'src/base';

/**
 * @description Universally Unique Identifier. We only recommend using versions 1 and 4 within GOBL.
 */
export class Uuid extends BaseSchema {
  /**
   * @description The Schema ID of the GOBL UUID structure.
   */
  SCHEMA_ID: string = 'https://gobl.org/draft-0/uuid/uuid';

  /**
   * @description Universally Unique Identifier. We only recommend using versions 1 and 4 within GOBL.
   */
  uuid: string;
}
