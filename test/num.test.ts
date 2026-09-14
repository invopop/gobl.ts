import { describe, expect, it } from 'vitest';
import { readFileSync, readdirSync } from 'node:fs';
import { Amount, Percentage } from '../src/index.js';

/**
 * The semantics of GOBL's num package are unusual enough that "looks right" is
 * not good enough: operations adopt the receiver's exponent, rounding is half
 * away from zero, and overflow drops decimal places rather than wrapping.
 *
 * These fixtures are produced by executing the real Go package
 * (`make num-fixtures`), so this file checks the port against the
 * implementation it mirrors rather than against my reading of it. Every case
 * asserts the exact string form too, which pins the resulting exponent and not
 * just the numeric value.
 */
// The mantissa arrives as a string: int64 values above 2^53 lose precision as
// JSON numbers, which would silently weaken every overflow case.
type Pair = { v: string; e: number };
type Case = {
  op: string;
  a?: Pair;
  b?: Pair;
  n?: number;
  in?: string; // a pointer on the Go side, so an empty string survives
  r?: Pair;
  r2?: Pair;
  s?: string;
  i?: number;
  err?: boolean;
};

const fixtures = JSON.parse(readFileSync('test/fixtures/num.json', 'utf8')) as {
  gobl: string;
  cases: Case[];
};

const amt = (p: Pair): Amount => Amount.make(BigInt(p.v), p.e);
const describePair = (p?: Pair) => (p ? `${p.v}e${p.e}` : '-');

/** Renders a case compactly, so a failure names the operation that broke. */
function label(c: Case): string {
  const parts = [c.op];
  if (c.in !== undefined) parts.push(JSON.stringify(c.in));
  if (c.a) parts.push(describePair(c.a));
  if (c.b) parts.push(describePair(c.b));
  if (c.n !== undefined) parts.push(`n=${c.n}`);
  return parts.join(' ');
}

/** Applies one recorded case, returning a comparable summary. */
function run(c: Case): { r?: string; r2?: string; s?: string; i?: number; err?: boolean } {
  const a = c.a ? amt(c.a) : undefined;
  const b = c.b ? amt(c.b) : undefined;

  switch (c.op) {
    case 'add':
      return pack(a!.add(b!));
    case 'subtract':
      return pack(a!.subtract(b!));
    case 'multiply':
      return pack(a!.multiply(b!));
    case 'divide':
      return pack(a!.divide(b!));
    case 'matchPrecision':
      return pack(a!.matchPrecision(b!));
    case 'compare':
      return { i: a!.compare(b!) };
    case 'split': {
      const [q, rem] = a!.split(c.n!);
      return { r: key(q), r2: key(rem), s: q.toString() };
    }
    case 'rescale':
      return pack(a!.rescale(c.n!));
    case 'rescaleUp':
      return pack(a!.rescaleUp(c.n!));
    case 'rescaleDown':
      return pack(a!.rescaleDown(c.n!));
    case 'upscale':
      return pack(a!.upscale(c.n!));
    case 'downscale':
      return pack(a!.downscale(c.n!));
    case 'rescaleRange': {
      // The generator encodes the pair as lo*100 + hi.
      const lo = Math.floor(c.n! / 100);
      const hi = c.n! % 100;
      return pack(a!.rescaleRange(lo, hi));
    }
    case 'negate':
      return pack(a!.negate());
    case 'abs':
      return pack(a!.abs());
    case 'string':
      return { s: a!.toString() };
    case 'minimalString':
      return { s: a!.minimalString() };
    case 'parse':
      try {
        return pack(Amount.parse(c.in!));
      } catch {
        return { err: true };
      }
    case 'percentParse':
      try {
        const p = Percentage.parse(c.in!);
        return { r: key(p.base()), s: p.toString() };
      } catch {
        return { err: true };
      }
    case 'percentAmount':
      return pack(Percentage.parse(c.in!).amount());
    case 'percentFactor':
      return pack(Percentage.parse(c.in!).factor());
    case 'percentOf':
      return pack(Percentage.parse(c.in!).of(a!));
    case 'percentFrom':
      return pack(Percentage.parse(c.in!).from(a!));
    case 'remove':
      return pack(a!.remove(Percentage.parse(c.in!)));
    default:
      throw new Error(`unhandled fixture op: ${c.op}`);
  }
}

const key = (a: Amount) => `${a.value()}e${a.exp()}`;
const pack = (a: Amount) => ({ r: key(a), s: a.toString() });

function expected(c: Case) {
  if (c.err) return { err: true };
  const out: Record<string, unknown> = {};
  if (c.r) out.r = `${c.r.v}e${c.r.e}`;
  if (c.r2) out.r2 = `${c.r2.v}e${c.r2.e}`;
  if (c.s !== undefined) out.s = c.s;
  if (c.i !== undefined) out.i = c.i;
  return out;
}

describe(`num parity with GOBL ${fixtures.gobl}`, () => {
  it(`replays all ${fixtures.cases.length} recorded cases`, () => {
    const failures: string[] = [];

    for (const c of fixtures.cases) {
      let got: ReturnType<typeof run>;
      try {
        got = run(c);
      } catch (err) {
        failures.push(`${label(c)} -> threw ${String(err)}`);
        continue;
      }
      const want = expected(c);
      for (const [field, wantValue] of Object.entries(want)) {
        const gotValue = (got as Record<string, unknown>)[field];
        if (gotValue !== wantValue) {
          failures.push(
            `${label(c)} -> ${field}: got ${String(gotValue)}, want ${String(wantValue)}`
          );
        }
      }
      if (failures.length > 20) break; // a systematic break would otherwise flood
    }

    // Printed as well as asserted: vitest truncates a long array in its diff,
    // and the whole value of this test is in naming the case that broke.
    if (failures.length > 0) {
      console.error(`\n${failures.length} parity failure(s):\n  ${failures.join('\n  ')}\n`);
    }
    expect(failures).toEqual([]);
  });

  it('covers every operation the port implements', () => {
    const ops = new Set(fixtures.cases.map((c) => c.op));
    for (const op of [
      'add',
      'subtract',
      'multiply',
      'divide',
      'compare',
      'matchPrecision',
      'split',
      'rescale',
      'rescaleUp',
      'rescaleDown',
      'rescaleRange',
      'upscale',
      'downscale',
      'negate',
      'abs',
      'string',
      'minimalString',
      'parse',
      'percentParse',
      'percentAmount',
      'percentFactor',
      'percentOf',
      'percentFrom',
      'remove',
    ]) {
      expect(ops, `fixtures should cover ${op}`).toContain(op);
    }
  });
});

/**
 * Behaviours where the port deliberately differs from GOBL. These cannot come
 * from the fixtures by definition, so they are written by hand and each one
 * records why the divergence is intentional.
 */
describe('deliberate divergences from GOBL', () => {
  it('rejects a leading + that Go would accept', () => {
    // Go hands the major part to strconv.ParseInt, which accepts a sign, so
    // "+5" parses there. The published JSON Schema pattern is
    // ^\-?[0-9]+(\.[0-9]+)?$ and no GOBL output contains it.
    expect(() => Amount.parse('+5')).toThrow();
  });

  it('rejects a signed decimal part that Go silently misreads', () => {
    // In Go "1.-5" becomes "0.95", because the fraction also goes through
    // ParseInt. Rejecting is safer than reproducing the accident.
    expect(() => Amount.parse('1.-5')).toThrow();
    expect(() => Amount.parse('1.+5')).toThrow();
  });

  it('formats beyond 18 decimal places, where Go overflows its int64 helper', () => {
    // Go computes 10^exp with an int64 helper that wraps past 18, so this
    // prints as "-1.0000000000002120200481923981312" there. BigInt is correct.
    const a = Amount.make(6690990000000000000n, 31);
    expect(a.toString()).toBe('0.0000000000006690990000000000000');
  });

  it('still reports NA beyond 1000 decimal places, matching GOBL', () => {
    expect(Amount.make(1, 1001).toString()).toBe('NA');
  });

  it('clamps negate at the int64 boundary instead of wrapping', () => {
    // Go's two's-complement negation sends the int64 minimum back to itself.
    // Clamping to the maximum is saner for an already-saturated value, and the
    // case is untested upstream. The fixtures use MinInt64+1, so no recorded
    // case depends on either behaviour.
    const min = Amount.make(-9223372036854775808n, 0);
    expect(min.negate().value()).toBe(9223372036854775807n);
    expect(min.abs().value()).toBe(9223372036854775807n);
  });
});

/**
 * The 133 vendored envelopes are real GOBL output across 24 countries and every
 * addon. Round-tripping every amount and percentage in them proves the parser
 * and formatter handle the shapes GOBL actually emits — not just the shapes I
 * thought to test.
 */
describe('round-trips every amount in the example corpus', () => {
  const files = readdirSync('test/fixtures').filter((f) => f.endsWith('.json') && f !== 'num.json');

  /** Collects values under keys the GOBL schemas type as amounts or percentages. */
  function collect(node: unknown, out: { amounts: string[]; percentages: string[] }): void {
    if (Array.isArray(node)) {
      for (const item of node) collect(item, out);
      return;
    }
    if (node === null || typeof node !== 'object') return;
    for (const [, value] of Object.entries(node as Record<string, unknown>)) {
      if (typeof value === 'string') {
        if (/^-?\d+(\.\d+)?%$/.test(value)) out.percentages.push(value);
        else if (/^-?\d+\.\d+$/.test(value)) out.amounts.push(value);
      } else {
        collect(value, out);
      }
    }
  }

  it(`parses and re-renders every value across ${files.length} envelopes`, () => {
    const found = { amounts: [] as string[], percentages: [] as string[] };
    for (const file of files) {
      collect(JSON.parse(readFileSync(`test/fixtures/${file}`, 'utf8')), found);
    }

    // Guard against the walker silently matching nothing.
    expect(found.amounts.length).toBeGreaterThan(500);
    expect(found.percentages.length).toBeGreaterThan(50);

    const failures: string[] = [];
    for (const v of found.amounts) {
      const got = Amount.parse(v).toString();
      if (got !== v) failures.push(`amount ${v} -> ${got}`);
    }
    for (const v of found.percentages) {
      const got = Percentage.parse(v).toString();
      if (got !== v) failures.push(`percentage ${v} -> ${got}`);
    }
    expect(failures).toEqual([]);
  });
});

/**
 * Checks the arithmetic against GOBL's own calculated output rather than
 * against its unit tests: every line in the corpus was multiplied out by the
 * Go implementation, so recomputing the same product here is an independent
 * confirmation that multiply and the exponent rules are right.
 *
 * It also marks the boundary of this port. Line sums are reproducible from the
 * document; **tax amounts are not**, because GOBL derives them from a
 * high-precision accumulated base (currency subunits + 2, summed across lines)
 * that the finished document never exposes — only the rounded view of it. That
 * is why the totals pipeline stays on the server.
 */
describe('agrees with GOBL-calculated line totals', () => {
  it('reproduces every line sum in the corpus', () => {
    const files = readdirSync('test/fixtures').filter(
      (f) => f.endsWith('.json') && f !== 'num.json'
    );

    let exact = 0;
    let afterRounding = 0;
    const unexplained: string[] = [];

    for (const file of files) {
      const doc = (
        JSON.parse(readFileSync(`test/fixtures/${file}`, 'utf8')) as {
          doc?: { lines?: Array<{ quantity?: string; sum?: string; item?: { price?: string } }> };
        }
      ).doc;

      for (const line of doc?.lines ?? []) {
        const price = line.item?.price;
        if (!price || !line.quantity || !line.sum) continue;

        const got = Amount.parse(price).multiply(Amount.parse(line.quantity));
        const want = Amount.parse(line.sum);

        if (got.equals(want)) {
          exact++;
        } else if (got.rescale(want.exp()).equals(want)) {
          // GOBL applied its "currency" rounding rule to the line, which only
          // reduces precision — the product itself agrees.
          afterRounding++;
        } else {
          unexplained.push(
            `${file}: ${price} x ${line.quantity} -> ${got}, document says ${line.sum}`
          );
        }
      }
    }

    expect(exact + afterRounding).toBeGreaterThan(200); // the walker found lines
    expect(unexplained).toEqual([]);
  });
});
