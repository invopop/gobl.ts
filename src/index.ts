import { Uuid } from './schemas/uuid/uuid';
import { Invoice } from './schemas/bill/invoice';
import { InvoiceValue } from './schemas/bill/types';

export type BillType = {
  Invoice: typeof Invoice & InvoiceValue;
};

export type UuidType = {
  Uuid: typeof Uuid;
};

export type GOBLType = {
  Bill: BillType;
  Uuid: UuidType;
};

export default {
  Bill: {
    Invoice,
  },
  Uuid: {
    Uuid,
  },
} as GOBLType;
