/**
 * Port of GOBL's `num.Percentage` (github.com/invopop/gobl/num/percentage.go).
 *
 * A percentage is stored as a **factor**, not as a "per hundred" number: 16% is
 * held internally as `0.160`. The `%` sign exists only at the string boundary,
 * which is why `"16.0%"` and `"0.160"` parse to the same value.
 *
 * That distinction is easy to get wrong and expensive when you do — a tax rate
 * entered as `"16"` rather than `"16%"` is 1600%.
 */
import { Amount } from './amount.js';

const ONE = Amount.make(1, 0);
const HUNDRED = Amount.make(100, 0);

export class Percentage {
  readonly #amount: Amount;

  private constructor(amount: Amount) {
    this.#amount = amount;
  }

  /** Zero percent. */
  static readonly ZERO = new Percentage(Amount.make(0, 0));

  /**
   * Builds a percentage from a raw mantissa and exponent **of the factor**.
   * Mirrors GOBL's `MakePercentage`, so 16% is `make(16, 2)` — the factor 0.16.
   */
  static make(value: bigint | number, exp: number): Percentage {
    return new Percentage(Amount.make(value, exp));
  }

  /**
   * Parses a percentage. A trailing `%` divides by 100, so both spellings of
   * 16% are accepted and equivalent:
   *
   * ```ts
   * Percentage.parse('16.0%').base().toString(); // "0.160"
   * Percentage.parse('0.160').base().toString(); // "0.160"
   * ```
   *
   * An empty string is zero, without error, matching GOBL.
   */
  static parse(value: string): Percentage {
    if (value.length === 0) return Percentage.ZERO;
    const percent = value.endsWith('%');
    const amount = Amount.parse(percent ? value.slice(0, -1) : value);
    return percent ? Percentage.fromAmount(amount) : new Percentage(amount);
  }

  /** Treats an amount as a percentage figure, scaling it down by 100. */
  static fromAmount(amount: Amount): Percentage {
    return new Percentage(amount.rescale(amount.exp() + 2).divide(HUNDRED));
  }

  /** The raw mantissa of the underlying factor. */
  value(): bigint {
    return this.#amount.value();
  }

  /** The exponent of the underlying factor. */
  exp(): number {
    return this.#amount.exp();
  }

  /**
   * The underlying factor: 16% has a base of `"0.160"`.
   * Use {@link Percentage.amount} for the human-facing figure.
   */
  base(): Amount {
    return this.#amount;
  }

  /** The display figure, scaled back up by 100: 16% gives `"16.0"`. */
  amount(): Amount {
    const exp = Math.max(this.#amount.exp() - 2, 0);
    return this.#amount.multiply(HUNDRED).rescale(exp);
  }

  /**
   * Applies this percentage to an amount, keeping **the amount's** exponent.
   *
   * ```ts
   * Percentage.parse('21%').of(Amount.parse('1800.00')); // "378.00"
   * ```
   */
  of(amount: Amount): Amount {
    return amount.multiply(this.#amount);
  }

  /**
   * Extracts the part of an amount that this percentage accounts for, assuming
   * it has already been applied — the tax included in a gross figure.
   *
   * ```ts
   * Percentage.parse('16%').from(Amount.parse('116.00')); // "16.00"
   * ```
   */
  from(amount: Amount): Amount {
    return amount.subtract(amount.divide(this.factor()));
  }

  /** The factor with 1 added, so 16% gives `"1.160"`. */
  factor(): Amount {
    return this.#amount.add(ONE);
  }

  /** Changes the precision of the underlying factor. */
  rescale(exp: number): Percentage {
    return new Percentage(this.#amount.rescale(exp));
  }

  /** Flips the sign. */
  negate(): Percentage {
    return new Percentage(this.#amount.negate());
  }

  /**
   * Flips the sign.
   * @deprecated Use {@link Percentage.negate}, matching GOBL's own deprecation.
   */
  invert(): Percentage {
    return this.negate();
  }

  /** Compares numerically, ignoring precision. */
  compare(other: Percentage): number {
    return this.#amount.compare(other.#amount);
  }

  equals(other: Percentage): boolean {
    return this.#amount.equals(other.#amount);
  }

  isZero(): boolean {
    return this.#amount.isZero();
  }

  isNegative(): boolean {
    return this.#amount.isNegative();
  }

  isPositive(): boolean {
    return this.#amount.isPositive();
  }

  /** The GOBL string form, including the `%` sign. */
  toString(): string {
    return `${this.stringWithoutSymbol()}%`;
  }

  /** The display figure without the `%` sign. */
  stringWithoutSymbol(): string {
    return this.amount().toString();
  }

  /** Serialises to the GOBL string form, so it can go straight into a document. */
  toJSON(): string {
    return this.toString();
  }
}
