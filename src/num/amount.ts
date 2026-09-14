/**
 * Port of GOBL's `num.Amount` (github.com/invopop/gobl/num/amount.go).
 *
 * A fixed-point decimal whose number of decimal places is part of its
 * identity. GOBL serialises amounts as strings precisely so that `"90.00"`
 * keeps its two decimal places, and those places determine the precision of
 * anything derived from the value.
 *
 * Two properties make this different from every general-purpose decimal
 * library, and both are deliberate:
 *
 *   1. Binary operations adopt the **receiver's** exponent, not the operand's
 *      and not the maximum. `Amount.parse('100.00').add(Amount.parse('0.0001'))`
 *      is `"100.00"` — the operand's extra precision is discarded. Call
 *      {@link Amount.matchPrecision} first when you want to keep it, as GOBL's
 *      own code does before nearly every addition.
 *   2. Rounding is half **away from zero**, not banker's rounding.
 *
 * Instances are immutable; every method returns a new Amount.
 */

/** Maximum total significant digits in a parsed amount, matching GOBL. */
export const AMOUNT_MAX_DIGITS = 18;

const MAX_INT64 = 9223372036854775807n;
const MIN_INT64 = -9223372036854775808n;

const TEN = 10n;

/** 10^exp. */
function pow10(exp: number): bigint {
  return TEN ** BigInt(exp);
}

/**
 * Divides rounding half away from zero — GOBL's only rounding primitive.
 * `-62.5` becomes `-63`, not `-62` and not `-62.5 -> -62` as banker's rounding
 * would give.
 */
function roundedDiv(num: bigint, den: bigint): bigint {
  let n = num;
  let d = den;
  if (d < 0n) {
    n = -n;
    d = -d;
  }
  const q = n / d; // BigInt division truncates toward zero, like Go's Quo
  const r = n % d; // remainder carries the sign of n, like Go's QuoRem
  const twiceRem = (r < 0n ? -r : r) * 2n;
  if (twiceRem >= d) {
    return n < 0n ? q - 1n : q + 1n;
  }
  return q;
}

/** Rescales a raw value from one exponent to another, rounding when reducing. */
function rescaleValue(v: bigint, from: number, to: number): bigint {
  if (from < to) return v * pow10(to - from);
  if (from > to) return roundedDiv(v, pow10(from - to));
  return v;
}

/**
 * Constrains a value to what Go's int64 mantissa can hold.
 *
 * Reproduced faithfully rather than left to BigInt's unbounded range: the
 * point of this port is that client-side results match the server's. Decimal
 * places are dropped (lowering the exponent) to make the value fit, and if the
 * integer part alone will not fit it saturates at the int64 boundary.
 */
function fitToInt64(v: bigint, exp: number): [bigint, number] {
  if (v >= MIN_INT64 && v <= MAX_INT64) return [v, exp];

  let drop = 0;
  let probe = v;
  while ((probe < MIN_INT64 || probe > MAX_INT64) && drop < exp) {
    probe = probe / TEN; // truncating, matching Go's probe loop
    drop++;
  }
  let out = v;
  let outExp = exp;
  if (drop > 0) {
    out = roundedDiv(v, pow10(drop));
    outExp = exp - drop;
  }
  if (out < MIN_INT64) return [MIN_INT64, outExp];
  if (out > MAX_INT64) return [MAX_INT64, outExp];
  return [out, outExp];
}

export class Amount {
  readonly #value: bigint;
  readonly #exp: number;

  private constructor(value: bigint, exp: number) {
    this.#value = value;
    this.#exp = exp;
  }

  /** The zero amount, with no decimal places. */
  static readonly ZERO = new Amount(0n, 0);

  /**
   * Builds an amount from a raw mantissa and exponent, so the numeric value is
   * `value / 10^exp`. Mirrors GOBL's `MakeAmount`.
   *
   * ```ts
   * Amount.make(1267, 2).toString(); // "12.67"
   * ```
   */
  static make(value: bigint | number, exp: number): Amount {
    assertExp(exp);
    const [v, e] = fitToInt64(BigInt(value), exp);
    return new Amount(v, e);
  }

  /**
   * Parses a GOBL amount string: an optional `-`, digits, and optionally a `.`
   * followed by more digits. The number of decimal places written is retained,
   * so `"90"`, `"90.0"` and `"90.00"` are numerically equal but not identical.
   *
   * Decimal digits beyond the 18 significant-digit budget are **truncated**,
   * not rounded, matching GOBL.
   *
   * @throws if the string is not a valid amount.
   */
  static parse(value: string): Amount {
    const negative = value.startsWith('-');
    const body = negative ? value.slice(1) : value;
    const parts = body.split('.');
    if (parts.length > 2) {
      throw new SyntaxError(`amount must contain 0 or 1 decimal separators: ${value}`);
    }

    const major = parts[0] ?? '';
    // Leading zeros carry no overflow risk, so they do not count against the
    // digit budget, but they are dropped from the parsed value.
    const sigMajor = major.replace(/^0+/, '').length;
    if (sigMajor > AMOUNT_MAX_DIGITS) {
      throw new RangeError(
        `amount '${value}' has too many digits (${sigMajor}), maximum is ${AMOUNT_MAX_DIGITS}`
      );
    }
    if (!/^[0-9]+$/.test(major)) {
      throw new SyntaxError(`invalid major number '${value}'`);
    }

    let minor = parts.length === 2 ? (parts[1] ?? '') : '';
    if (parts.length === 2) {
      minor = minor.slice(0, AMOUNT_MAX_DIGITS - sigMajor);
      // GOBL hands the fractional part to strconv.ParseInt, which accepts a
      // sign, so "1.-5" parses there as "0.95". The published JSON Schema
      // pattern forbids it and no GOBL output contains it, so it is rejected
      // here instead of reproducing the accident.
      if (minor !== '' && !/^[0-9]+$/.test(minor)) {
        throw new SyntaxError(`invalid decimal number '${value}'`);
      }
    }

    const exp = minor.length;
    let v = BigInt(major);
    if (exp > 0) {
      v = v * pow10(exp) + BigInt(minor);
    }
    return new Amount(negative ? -v : v, exp);
  }

  /**
   * Builds an amount from a JavaScript number at the given precision.
   *
   * Prefer {@link Amount.parse} wherever the value came from a GOBL document:
   * a float cannot represent every decimal exactly, which is the problem this
   * type exists to avoid.
   */
  static fromNumber(value: number, exp: number): Amount {
    assertExp(exp);
    if (!Number.isFinite(value)) {
      throw new RangeError(`cannot build an amount from ${value}`);
    }
    const scaled = value * Number(pow10(exp));
    // Go uses math.Round, which rounds half away from zero; JavaScript's
    // Math.round rounds half toward +Infinity, so -0.5 would differ.
    const rounded = scaled < 0 ? -Math.round(-scaled) : Math.round(scaled);
    return Amount.make(BigInt(rounded), exp);
  }

  /** The raw mantissa, where the numeric value is `value / 10^exp`. */
  value(): bigint {
    return this.#value;
  }

  /** The number of decimal places. */
  exp(): number {
    return this.#exp;
  }

  /** Adds another amount, keeping **this** amount's exponent. */
  add(other: Amount): Amount {
    const sum = this.#value + rescaleValue(other.#value, other.#exp, this.#exp);
    const [v, e] = fitToInt64(sum, this.#exp);
    return new Amount(v, e);
  }

  /** Subtracts another amount, keeping **this** amount's exponent. */
  subtract(other: Amount): Amount {
    const diff = this.#value - rescaleValue(other.#value, other.#exp, this.#exp);
    const [v, e] = fitToInt64(diff, this.#exp);
    return new Amount(v, e);
  }

  /**
   * Multiplies by another amount, keeping **this** amount's exponent. The
   * result is therefore rounded to this amount's precision:
   * `1.01 × 1.01` is `"1.02"`, not `"1.0201"`.
   */
  multiply(other: Amount): Amount {
    let product = this.#value * other.#value;
    if (other.#exp > 0) {
      product = roundedDiv(product, pow10(other.#exp));
    }
    const [v, e] = fitToInt64(product, this.#exp);
    return new Amount(v, e);
  }

  /**
   * Divides by another amount, keeping **this** amount's exponent — division
   * never increases precision, so `10.00 / 11` is `"0.91"`.
   *
   * Dividing by zero returns zero at this amount's exponent rather than
   * throwing, matching GOBL.
   */
  divide(other: Amount): Amount {
    if (other.#value === 0n) return new Amount(0n, this.#exp);
    const num = this.#value * pow10(other.#exp);
    const q = roundedDiv(num, other.#value);
    const [v, e] = fitToInt64(q, this.#exp);
    return new Amount(v, e);
  }

  /**
   * Divides into `x` parts, returning the quotient and a remainder part that
   * absorbs the rounding error, so the parts sum back to the original.
   *
   * ```ts
   * Amount.parse('10.00').split(11); // ["0.91", "0.90"]
   * ```
   */
  split(x: number): [Amount, Amount] {
    const quotient = this.divide(Amount.make(BigInt(x), 0));
    const remainder = this.subtract(quotient.multiply(Amount.make(BigInt(x - 1), 0)));
    return [quotient, remainder];
  }

  /** Removes a percentage that was already applied to this amount. */
  remove(percent: { factor(): Amount }): Amount {
    return this.divide(percent.factor());
  }

  /**
   * Compares numerically, ignoring precision: `-1`, `0` or `1`.
   * `"90"` and `"90.00"` compare equal.
   */
  compare(other: Amount): number {
    const exp = Math.max(this.#exp, other.#exp);
    const a = rescaleValue(this.#value, this.#exp, exp);
    const b = rescaleValue(other.#value, other.#exp, exp);
    return a < b ? -1 : a > b ? 1 : 0;
  }

  /** True if the two amounts are numerically equal, whatever their precision. */
  equals(other: Amount): boolean {
    return this.compare(other) === 0;
  }

  /**
   * Forces the amount to the given number of decimal places, rounding when
   * reducing. Increasing is capped where the mantissa would no longer fit.
   */
  rescale(exp: number): Amount {
    assertExp(exp);
    if (this.#exp > exp) {
      return new Amount(roundedDiv(this.#value, pow10(this.#exp - exp)), exp);
    }
    if (this.#exp < exp) {
      const [v, e] = fitToInt64(this.#value * pow10(exp - this.#exp), exp);
      return new Amount(v, e);
    }
    return this;
  }

  /** Increases precision to `exp`, never decreases it. */
  rescaleUp(exp: number): Amount {
    assertExp(exp);
    return exp > this.#exp ? this.rescale(exp) : this;
  }

  /** Decreases precision to `exp`, never increases it. */
  rescaleDown(exp: number): Amount {
    assertExp(exp);
    return exp < this.#exp ? this.rescale(exp) : this;
  }

  /** Constrains precision to a range, applying the minimum before the maximum. */
  rescaleRange(minimum: number, maximum: number): Amount {
    return this.rescaleUp(minimum).rescaleDown(maximum);
  }

  /**
   * Raises this amount's precision to the other's, if the other is more
   * precise. GOBL calls this before nearly every addition, because `add`
   * would otherwise discard the operand's extra decimal places.
   */
  matchPrecision(other: Amount): Amount {
    return this.rescaleUp(other.#exp);
  }

  /** Adds `increase` decimal places. */
  upscale(increase: number): Amount {
    return this.rescale(this.#exp + increase);
  }

  /** Removes `decrease` decimal places, stopping at zero. */
  downscale(decrease: number): Amount {
    return this.rescale(decrease > this.#exp ? 0 : this.#exp - decrease);
  }

  /** Flips the sign, keeping precision. */
  negate(): Amount {
    // Negating the int64 minimum has no positive counterpart, so it is clamped
    // to the maximum. GOBL wraps back to the minimum there via two's
    // complement; the case is untested upstream and clamping is the saner
    // behaviour for a value that is already saturated.
    const v = -this.#value;
    if (v > MAX_INT64) return new Amount(MAX_INT64, this.#exp);
    if (v < MIN_INT64) return new Amount(MIN_INT64, this.#exp);
    return new Amount(v, this.#exp);
  }

  /**
   * Flips the sign.
   * @deprecated Use {@link Amount.negate}, matching GOBL's own deprecation.
   */
  invert(): Amount {
    return this.negate();
  }

  /** The absolute value, keeping precision. */
  abs(): Amount {
    return this.#value < 0n ? this.negate() : this;
  }

  isZero(): boolean {
    return this.#value === 0n;
  }

  isNegative(): boolean {
    return this.#value < 0n;
  }

  isPositive(): boolean {
    return this.#value > 0n;
  }

  /**
   * The GOBL string form, always with exactly `exp` decimal places, so
   * trailing zeros are preserved.
   */
  toString(): string {
    if (this.#exp === 0) return this.#value.toString();
    // GOBL emits "NA" beyond 1000 decimal places rather than attempt it.
    if (this.#exp > 1000) return 'NA';
    const negative = this.#value < 0n;
    const v = negative ? -this.#value : this.#value;
    const p = pow10(this.#exp);
    const major = v / p;
    const minor = v - major * p;
    return `${negative ? '-' : ''}${major}.${minor.toString().padStart(this.#exp, '0')}`;
  }

  /** The string form with trailing zeros, and any bare `.`, removed. */
  minimalString(): string {
    const s = this.toString();
    if (!s.includes('.')) return s;
    return s.replace(/0+$/, '').replace(/\.$/, '');
  }

  /**
   * The value as a JavaScript number.
   *
   * Lossy for values needing more than 15–17 significant digits; use it for
   * display or charting, never to compute a monetary result.
   */
  toNumber(): number {
    return Number(this.#value) / Number(pow10(this.#exp));
  }

  /** Serialises to the GOBL string form, so an Amount can go straight into a document. */
  toJSON(): string {
    return this.toString();
  }
}

function assertExp(exp: number): void {
  if (!Number.isInteger(exp) || exp < 0) {
    throw new RangeError(`exponent must be a non-negative integer, got ${exp}`);
  }
}
