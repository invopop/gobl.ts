type Primitives = string | number | boolean | Date;

export abstract class ValueObject<T extends Primitives> {
  private _value: T;

  public constructor(value: T) {
    this._value = value;
  }

  public toJSON(): string {
    return String(this._value);
  }
}
