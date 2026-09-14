// Command gojacheck verifies that the published JavaScript runs under goja,
// the pure-Go engine used for embedded scripting.
//
// Invopop's scripting blocks run customer code in goja, so this library has to
// work there — and that is not something the Node test suite can tell us. goja
// is not a JavaScript engine with a couple of gaps; it is a distinct
// implementation, and the parts this library leans on hardest are the newest:
// BigInt (every Amount is built on it), private class fields, and BigInt's
// truncating division and remainder signs, which the half-away-from-zero
// rounding depends on.
//
// The check loads dist/index.cjs — the actual published artifact, chunks and
// all — and replays the same fixtures the Node tests use, so goja support is a
// guarantee rather than a happy accident.
//
//	go run ./cmd/gojacheck [-dist dist/index.cjs] [-fixtures test/fixtures/num.json]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

func main() {
	dist := flag.String("dist", filepath.Join("dist", "index.cjs"), "published CommonJS entry point")
	fixtures := flag.String("fixtures", filepath.Join("test", "fixtures", "num.json"), "num parity fixtures")
	flag.Parse()

	if err := run(*dist, *fixtures); err != nil {
		fmt.Fprintf(os.Stderr, "gojacheck: %v\n", err)
		os.Exit(1)
	}
}

func run(dist, fixtures string) error {
	distPath, err := filepath.Abs(dist)
	if err != nil {
		return err
	}
	if _, err := os.Stat(distPath); err != nil {
		return fmt.Errorf("%s not found; run `npm run build` first", dist)
	}
	body, err := os.ReadFile(fixtures)
	if err != nil {
		return err
	}

	vm := goja.New()
	require.NewRegistry().Enable(vm)

	start := time.Now()
	if _, err := vm.RunString(fmt.Sprintf("var G = require(%q);", distPath)); err != nil {
		return fmt.Errorf("loading the published bundle: %w", err)
	}
	loadTime := time.Since(start)

	if err := vm.Set("FIXTURES_RAW", string(body)); err != nil {
		return err
	}

	checks := []struct {
		name string
		fn   func(*goja.Runtime) error
	}{
		{"exports", checkExports},
		{"arithmetic semantics", checkSemantics},
		{"client seam", checkClient},
		{"num parity fixtures", checkParity},
	}
	for _, c := range checks {
		start := time.Now()
		if err := c.fn(vm); err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
		fmt.Printf("  ok  %-22s %v\n", c.name, time.Since(start).Round(time.Millisecond))
	}

	fmt.Printf("published bundle runs under goja (loaded in %v)\n", loadTime.Round(time.Millisecond))
	return nil
}

// eval runs a script that must return an empty string, or a description of what
// went wrong. Assertions live in JavaScript so they exercise the same surface a
// customer's script would.
func eval(vm *goja.Runtime, src string) error {
	v, err := vm.RunString(src)
	if err != nil {
		return err
	}
	if s := v.String(); s != "" {
		return fmt.Errorf("%s", s)
	}
	return nil
}

func checkExports(vm *goja.Runtime) error {
	return eval(vm, `
	  (function () {
	    var missing = [];
	    ['Amount', 'Percentage', 'GOBLClient', 'GOBLError', 'document', 'GOBL_VERSION',
	     'SchemaIDs', 'bill', 'tax', 'org', 'num'].forEach(function (k) {
	      if (G[k] === undefined) missing.push(k);
	    });
	    if (missing.length) return 'missing exports: ' + missing.join(', ');
	    if (G.SchemaIDs.length < 100) return 'SchemaIDs looks truncated: ' + G.SchemaIDs.length;
	    if (Object.keys(G.bill.InvoiceTypeLabels).length !== 6) return 'label map wrong';
	    return '';
	  })()
	`)
}

// checkSemantics pins the behaviours that would break silently if goja's BigInt
// diverged: the receiver-exponent rule, half-away-from-zero rounding, and
// saturation at the int64 boundary, which needs exact arithmetic beyond 2^53.
func checkSemantics(vm *goja.Runtime) error {
	return eval(vm, `
	  (function () {
	    var A = G.Amount, P = G.Percentage, bad = [];
	    function is(label, got, want) { if (got !== want) bad.push(label + ': got ' + got + ' want ' + want); }

	    is('multiply', A.parse('90.00').multiply(A.parse('20')).toString(), '1800.00');
	    is('receiver exponent', A.parse('100.00').add(A.parse('0.0001')).toString(), '100.00');
	    is('matchPrecision', A.parse('100.00').matchPrecision(A.parse('0.0001')).add(A.parse('0.0001')).toString(), '100.0001');
	    is('rounds half away from zero', A.parse('-1000').divide(A.parse('16')).toString(), '-63');
	    is('multiply rounds to receiver', A.parse('1.01').multiply(A.parse('1.01')).toString(), '1.02');
	    is('split remainder', A.parse('10.00').split(11).join(','), '0.91,0.90');
	    is('percent factor', P.parse('16.0%').base().toString(), '0.160');
	    is('percent of', P.parse('21%').of(A.parse('1800.00')).toString(), '378.00');
	    is('18-digit parse', A.parse('999999999999999999').toString(), '999999999999999999');
	    is('int64 saturation', A.make(BigInt('9223372036854775807'), 0).add(A.make(1n, 0)).toString(), '9223372036854775807');
	    is('toJSON', JSON.stringify({ price: A.parse('90.00') }), '{"price":"90.00"}');

	    return bad.join('; ');
	  })()
	`)
}

// checkClient confirms the embedding seam: the module loads without a fetch in
// scope, and an embedder can supply a host-backed one.
func checkClient(vm *goja.Runtime) error {
	if err := vm.Set("hostFetch", func(goja.FunctionCall) goja.Value { return goja.Undefined() }); err != nil {
		return err
	}
	return eval(vm, `
	  (function () {
	    var threw = false;
	    try { new G.GOBLClient(); } catch (e) { threw = true; }
	    if (!threw) return 'expected GOBLClient to refuse to construct without a fetch';

	    var c = new G.GOBLClient({ fetch: hostFetch, baseUrl: 'https://example.test/v0/' });
	    if (c.baseUrl !== 'https://example.test/v0') return 'baseUrl not normalised: ' + c.baseUrl;

	    var e = G.GOBLError.fromBody(JSON.stringify({
	      key: 'validation',
	      faults: [{ code: 'GOBL-BILL-INVOICE-09', paths: ['$.totals'], message: 'invoice totals are required' }]
	    }), 422);
	    if (e.faults.length !== 1) return 'faults not parsed';
	    if (e.faults[0].paths[0] !== '$.totals') return 'fault paths not parsed';
	    if (e.status !== 422) return 'status not parsed';
	    return '';
	  })()
	`)
}

// checkParity replays every case recorded from the Go num package, so goja gets
// exactly the guarantee Node does rather than a sampled approximation.
func checkParity(vm *goja.Runtime) error {
	v, err := vm.RunString(replayScript)
	if err != nil {
		return err
	}
	var result struct {
		Total    int      `json:"total"`
		Failures []string `json:"failures"`
	}
	if err := json.Unmarshal([]byte(v.String()), &result); err != nil {
		return err
	}
	if len(result.Failures) > 0 {
		return fmt.Errorf("%d of %d cases diverged under goja:\n    %v",
			len(result.Failures), result.Total, result.Failures)
	}
	if result.Total < 1000 {
		return fmt.Errorf("only %d cases replayed; fixtures look truncated", result.Total)
	}
	fmt.Printf("      %d parity cases replayed\n", result.Total)
	return nil
}

const replayScript = `
(function () {
  var F = JSON.parse(FIXTURES_RAW);
  var A = G.Amount, P = G.Percentage;
  var amt = function (p) { return A.make(BigInt(p.v), p.e); };
  var key = function (a) { return a.value() + 'e' + a.exp(); };
  var failures = [];

  for (var i = 0; i < F.cases.length; i++) {
    var c = F.cases[i];
    var a = c.a ? amt(c.a) : null, b = c.b ? amt(c.b) : null;
    var r = null, r2 = null, s = null, cmp = null, threw = false;
    try {
      switch (c.op) {
        case 'add': r = a.add(b); break;
        case 'subtract': r = a.subtract(b); break;
        case 'multiply': r = a.multiply(b); break;
        case 'divide': r = a.divide(b); break;
        case 'matchPrecision': r = a.matchPrecision(b); break;
        case 'compare': cmp = a.compare(b); break;
        case 'split': var sp = a.split(c.n); r = sp[0]; r2 = sp[1]; break;
        case 'rescale': r = a.rescale(c.n); break;
        case 'rescaleUp': r = a.rescaleUp(c.n); break;
        case 'rescaleDown': r = a.rescaleDown(c.n); break;
        case 'upscale': r = a.upscale(c.n); break;
        case 'downscale': r = a.downscale(c.n); break;
        case 'rescaleRange': r = a.rescaleRange(Math.floor(c.n / 100), c.n % 100); break;
        case 'negate': r = a.negate(); break;
        case 'abs': r = a.abs(); break;
        case 'string': s = a.toString(); break;
        case 'minimalString': s = a.minimalString(); break;
        case 'parse': r = A.parse(c.in); break;
        case 'percentParse': var pp = P.parse(c.in); r = pp.base(); s = pp.toString(); break;
        case 'percentAmount': r = P.parse(c.in).amount(); break;
        case 'percentFactor': r = P.parse(c.in).factor(); break;
        case 'percentOf': r = P.parse(c.in).of(a); break;
        case 'percentFrom': r = P.parse(c.in).from(a); break;
        case 'remove': r = a.remove(P.parse(c.in)); break;
        default: continue;
      }
    } catch (e) { threw = true; }

    var label = c.op + (c.in !== undefined ? ' ' + JSON.stringify(c.in) : '');
    if (c.err) {
      if (!threw) failures.push(label + ' should have thrown');
      continue;
    }
    if (threw) { failures.push(label + ' threw unexpectedly'); continue; }

    if (c.r && key(r) !== c.r.v + 'e' + c.r.e) {
      failures.push(label + ' r: got ' + key(r) + ' want ' + c.r.v + 'e' + c.r.e);
    }
    if (c.r2 && key(r2) !== c.r2.v + 'e' + c.r2.e) {
      failures.push(label + ' r2: got ' + key(r2) + ' want ' + c.r2.v + 'e' + c.r2.e);
    }
    if (c.s !== undefined) {
      var got = s !== null ? s : r.toString();
      if (got !== c.s) failures.push(label + ' s: got ' + got + ' want ' + c.s);
    }
    if (c.i !== undefined && cmp !== c.i) failures.push(label + ' i: got ' + cmp + ' want ' + c.i);
    if (failures.length > 10) break;
  }
  return JSON.stringify({ total: F.cases.length, failures: failures });
})()
`
