import { Uuid } from './schemas/uuid/uuid';
import { Invoice } from './schemas/bill/invoice';

export interface BillType {
  Invoice: typeof Invoice;
}

export interface UuidType {
  Uuid: typeof Uuid;
}

export interface GOBLType {
  Bill: BillType;
  Uuid: UuidType;
}

const GOBL: GOBLType = {
  Bill: {
    Invoice,
  },
  Uuid: {
    Uuid,
  },
};

export default GOBL;
