import { Uuid } from './schemas/uuid/uuid';
import { Invoice } from './schemas/bill/invoice';

type Bill = {
  Invoice: typeof Invoice;
};

type UuidType = {
  Uuid: typeof Uuid;
};

type GOBL = {
  Bill: Bill;
  Uuid: UuidType;
};

export const GOBL: GOBL = {
  Bill: {
    Invoice,
  },
  Uuid: {
    Uuid,
  },
};
