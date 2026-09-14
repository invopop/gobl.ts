// Command numfixtures records the behaviour of github.com/invopop/gobl/num as
// JSON fixtures, so the TypeScript port in src/num can be replayed against the
// real Go implementation.
//
// This is what makes porting num defensible. The semantics are unusual —
// operations adopt the receiver's exponent, rounding is half away from zero,
// and overflow drops decimal places rather than wrapping — so "looks right" is
// not good enough. Every case here is produced by executing the Go package
// pinned in go.mod, and CI fails on any diff, so a GOBL release that changes
// num surfaces as a failing build rather than as quietly wrong money.
//
//	go run ./cmd/numfixtures [-out test/fixtures/num.json]
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"

	"github.com/invopop/gobl/num"
)

// pair is an amount as {value, exp}, the representation the TypeScript side
// reconstructs with Amount.make().
//
// The mantissa is emitted as a *string*. int64 values above 2^53 cannot be
// represented exactly by a JSON number, and JSON.parse silently rounds them —
// 999999999999999999 would arrive in TypeScript as 1000000000000000000, making
// every overflow case compare against the wrong expectation.
type pair struct {
	Value string `json:"v"`
	Exp   uint32 `json:"e"`
}

func p(a num.Amount) pair { return pair{strconv.FormatInt(a.Value(), 10), a.Exp()} }

// maxReliableExp is the largest exponent whose String() output can be trusted.
//
// Above 18, Go's Amount.String() computes 10^exp with an int64 helper that
// overflows silently, so it emits garbage: 6690990000000000000e31 prints as
// "-1.0000000000002120200481923981312" rather than
// "0.0000000000006690990000000000000". No upstream test covers this. The
// TypeScript port uses BigInt and is simply correct, so recording Go's output
// here would assert a bug. The mantissa and exponent stay correct either way,
// so those are still compared; only the string form is dropped.
const maxReliableExp = 18

// str returns the string form, or "" (omitted from the fixture) where Go's
// own formatting cannot be trusted.
func str(a num.Amount) string {
	if a.Exp() > maxReliableExp {
		return ""
	}
	return a.String()
}

// testCase is one recorded operation. Only the fields relevant to the op are
// emitted, so the file stays readable.
type testCase struct {
	Op string `json:"op"`

	A *pair `json:"a,omitempty"`
	B *pair `json:"b,omitempty"`
	N *int  `json:"n,omitempty"` // integer argument: rescale target, split parts

	In *string `json:"in,omitempty"` // string input, for parse cases; a pointer so "" survives

	R   *pair  `json:"r,omitempty"`   // amount result
	R2  *pair  `json:"r2,omitempty"`  // second result, for split
	S   string `json:"s,omitempty"`   // String() of the result
	I   *int   `json:"i,omitempty"`   // integer result, for compare
	Err bool   `json:"err,omitempty"` // the operation fails
}

type fixtures struct {
	GOBL  string     `json:"gobl"`
	Cases []testCase `json:"cases"`
}

// Operand values chosen to cover the behaviours the port must reproduce:
// zero, signs, the half-way rounding cases from GOBL's own tables, and values
// near the int64 boundary where arithmetic drops decimal places or saturates.
var values = []int64{
	0, 1, -1, 101, 133, 250, 1000, -1000, 10010, -12345, 669099, 1002002,
	999999999999999999, math.MaxInt64,
}

// Exponents stay at or below 18. Beyond that Go's String() overflows its
// intPow helper and emits garbage that no upstream test pins, so recording it
// would assert a bug rather than a behaviour.
var exps = []uint32{0, 1, 2, 4, 18}

// Smaller operand set for the right-hand side, keeping the cross product
// manageable while still covering signs, zero and rounding boundaries.
var rhsValues = []int64{0, 1, -1, 11, 16, 21, -4, 999999999999999999}
var rhsExps = []uint32{0, 1, 2, 4}

func main() {
	out := flag.String("out", filepath.Join("test", "fixtures", "num.json"), "file to write")
	flag.Parse()

	f := fixtures{GOBL: goblVersion()}
	f.Cases = append(f.Cases, arithmetic()...)
	f.Cases = append(f.Cases, rescaling()...)
	f.Cases = append(f.Cases, unary()...)
	f.Cases = append(f.Cases, parsing()...)
	f.Cases = append(f.Cases, percentages()...)

	// One compact case per line: indenting 10k nested objects multiplies the
	// file size for no benefit, but a line per case keeps diffs readable when
	// a GOBL release does change something.
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "{\n  \"gobl\": %q,\n  \"cases\": [\n", f.GOBL)
	for i, c := range f.Cases {
		line, err := json.Marshal(c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "numfixtures: %v\n", err)
			os.Exit(1)
		}
		buf.Write(line)
		if i < len(f.Cases)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("  ]\n}\n")
	body := buf.Bytes()

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "numfixtures: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, body, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "numfixtures: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d cases from GOBL %s to %s\n", len(f.Cases), f.GOBL, *out)
}

func arithmetic() []testCase {
	var cases []testCase
	for _, av := range values {
		for _, ae := range exps {
			a := num.MakeAmount(av, ae)
			for _, bv := range rhsValues {
				for _, be := range rhsExps {
					b := num.MakeAmount(bv, be)
					ap, bp := p(a), p(b)

					for _, op := range []struct {
						name string
						fn   func(x, y num.Amount) num.Amount
					}{
						{"add", num.Amount.Add},
						{"subtract", num.Amount.Subtract},
						{"multiply", num.Amount.Multiply},
						{"divide", num.Amount.Divide},
					} {
						r := op.fn(a, b)
						rp := p(r)
						cases = append(cases, testCase{
							Op: op.name, A: &ap, B: &bp, R: &rp, S: str(r),
						})
					}

					cmp := a.Compare(b)
					cases = append(cases, testCase{Op: "compare", A: &ap, B: &bp, I: &cmp})

					mp := a.MatchPrecision(b)
					mpp := p(mp)
					cases = append(cases, testCase{
						Op: "matchPrecision", A: &ap, B: &bp, R: &mpp, S: str(mp),
					})
				}
			}
		}
	}

	// Split, which returns a quotient and a remainder that absorbs the
	// rounding error so the parts sum back to the original.
	for _, av := range []int64{1000, 10000, -1000, 7, 999999999999999999} {
		for _, ae := range []uint32{0, 2, 4} {
			for _, n := range []int{2, 3, 7, 11} {
				a := num.MakeAmount(av, ae)
				q, rem := a.Split(n)
				ap, qp, remp := p(a), p(q), p(rem)
				nn := n
				cases = append(cases, testCase{
					Op: "split", A: &ap, N: &nn, R: &qp, R2: &remp, S: str(q),
				})
			}
		}
	}
	return cases
}

func rescaling() []testCase {
	var cases []testCase
	targets := []uint32{0, 1, 2, 4, 9, 18}
	for _, av := range values {
		for _, ae := range exps {
			a := num.MakeAmount(av, ae)
			ap := p(a)
			for _, t := range targets {
				tt := int(t)
				for _, op := range []struct {
					name string
					fn   func(x num.Amount, e uint32) num.Amount
				}{
					{"rescale", num.Amount.Rescale},
					{"rescaleUp", num.Amount.RescaleUp},
					{"rescaleDown", num.Amount.RescaleDown},
					{"upscale", num.Amount.Upscale},
					{"downscale", num.Amount.Downscale},
				} {
					r := op.fn(a, t)
					rp := p(r)
					cases = append(cases, testCase{
						Op: op.name, A: &ap, N: &tt, R: &rp, S: str(r),
					})
				}
			}
			// RescaleRange applies the minimum before the maximum, so an
			// inverted range legitimately yields the maximum.
			for _, lo := range []uint32{0, 2, 4} {
				for _, hi := range []uint32{0, 2, 6} {
					r := a.RescaleRange(lo, hi)
					rp := p(r)
					n := int(lo)*100 + int(hi) // encoded, decoded on the TS side
					cases = append(cases, testCase{
						Op: "rescaleRange", A: &ap, N: &n, R: &rp, S: str(r),
					})
				}
			}
		}
	}
	return cases
}

func unary() []testCase {
	var cases []testCase
	for _, av := range values {
		for _, ae := range exps {
			a := num.MakeAmount(av, ae)
			ap := p(a)
			neg, abs := a.Negate(), a.Abs()
			negp, absp := p(neg), p(abs)
			cases = append(cases,
				testCase{Op: "negate", A: &ap, R: &negp, S: str(neg)},
				testCase{Op: "abs", A: &ap, R: &absp, S: str(abs)},
			)
			// String forms only where Go's formatting is trustworthy.
			if a.Exp() <= maxReliableExp {
				cases = append(cases,
					testCase{Op: "string", A: &ap, S: a.String()},
					testCase{Op: "minimalString", A: &ap, S: a.MinimalString()},
				)
			}
		}
	}
	return cases
}

func parsing() []testCase {
	inputs := []string{
		"0", "1", "-1", "90", "90.0", "90.00", "0.00", "-0.02",
		"12.67", "-12.67", "1234.56", "000123.456", "100.10",
		"999999999999999999", "-999999999999999999",
		"999999999999999999.1",   // decimal truncated away entirely
		"1.99999999999999999999", // truncated to the 18-digit budget
		"0.000000000000000001",
		"1234567890123456789", // 19 significant digits: error
		"-1234567890123456789",
		"", "bad", "1.2.3", "1,000", "1 000", "1e5",
		// "+5" is deliberately absent: Go's strconv.ParseInt accepts a leading
		// sign so it parses there, but the published JSON Schema pattern
		// forbids it and no GOBL output contains it. src/num rejects it, and
		// test/num.test.ts covers that divergence explicitly.
	}
	var cases []testCase
	for _, in := range inputs {
		in := in
		a, err := num.AmountFromString(in)
		if err != nil {
			cases = append(cases, testCase{Op: "parse", In: &in, Err: true})
			continue
		}
		ap := p(a)
		cases = append(cases, testCase{Op: "parse", In: &in, R: &ap, S: str(a)})
	}
	return cases
}

func percentages() []testCase {
	inputs := []string{
		"", "0%", "16%", "16.0%", "0.160", "21%", "5.5%", "100%", "-10%",
		"2000%", "0.5%", "0.05%",
	}
	amounts := []struct {
		v int64
		e uint32
	}{
		{0, 2}, {10000, 2}, {11600, 2}, {180000, 2}, {-10000, 2}, {123456, 4}, {100, 0},
	}

	var cases []testCase
	for _, in := range inputs {
		in := in
		pc, err := num.PercentageFromString(in)
		if err != nil {
			cases = append(cases, testCase{Op: "percentParse", In: &in, Err: true})
			continue
		}
		base, disp, factor := pc.Base(), pc.Amount(), pc.Factor()
		bp, dp, fp := p(base), p(disp), p(factor)
		cases = append(cases,
			testCase{Op: "percentParse", In: &in, R: &bp, S: pc.String()},
			testCase{Op: "percentAmount", In: &in, R: &dp, S: str(disp)},
			testCase{Op: "percentFactor", In: &in, R: &fp, S: str(factor)},
		)
		for _, am := range amounts {
			a := num.MakeAmount(am.v, am.e)
			ap := p(a)
			of, from := pc.Of(a), pc.From(a)
			ofp, fromp := p(of), p(from)
			rem := a.Remove(pc)
			remp := p(rem)
			cases = append(cases,
				testCase{Op: "percentOf", In: &in, A: &ap, R: &ofp, S: str(of)},
				testCase{Op: "percentFrom", In: &in, A: &ap, R: &fromp, S: str(from)},
				testCase{Op: "remove", In: &in, A: &ap, R: &remp, S: str(rem)},
			)
		}
	}
	return cases
}

func goblVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/invopop/gobl" {
			if dep.Replace != nil {
				return dep.Replace.Version
			}
			return dep.Version
		}
	}
	return "unknown"
}
