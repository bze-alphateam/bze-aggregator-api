package compare_test

import (
	"testing"

	"github.com/bze-alphateam/bze-aggregator-api/app/service/compare"
)

func TestDiffEqual(t *testing.T) {
	v := `{"a":[1,"x",true,null,{"b":"2"}]}`
	if d := compare.Diff(decodeJSON(v), decodeJSON(v), nil); len(d) != 0 {
		t.Fatalf("expected no difference, got %v", d)
	}
}

func TestDiffStringsExact(t *testing.T) {
	d := compare.Diff(decodeJSON(`{"price":"1.0"}`), decodeJSON(`{"price":"1"}`), nil)
	if len(d) != 1 || d[0].Path != "$.price" || d[0].Aggregator != `"1.0"` || d[0].Scan != `"1"` {
		t.Fatalf("unexpected differences %+v", d)
	}
}

func TestDiffDefaultNumberTolerance(t *testing.T) {
	if d := compare.Diff(decodeJSON(`{"x":1.0000000000001}`), decodeJSON(`{"x":1}`), nil); len(d) != 0 {
		t.Fatalf("1e-13 apart must match, got %v", d)
	}

	if d := compare.Diff(decodeJSON(`{"x":1.00000000001}`), decodeJSON(`{"x":1}`), nil); len(d) != 1 {
		t.Fatalf("1e-11 apart must differ, got %v", d)
	}

	if d := compare.Diff(decodeJSON(`{"x":0}`), decodeJSON(`{"x":0.0}`), nil); len(d) != 0 {
		t.Fatalf("0 and 0.0 must match, got %v", d)
	}
}

func TestDiffNumberAgainstString(t *testing.T) {
	if d := compare.Diff(decodeJSON(`{"x":1}`), decodeJSON(`{"x":"1"}`), nil); len(d) != 1 {
		t.Fatalf("a number and a string must differ, got %v", d)
	}
}

func TestDiffFieldTolerance(t *testing.T) {
	tol := map[string]float64{"liquidity_in_usd": 1e-6}
	agg := decodeJSON(`[{"liquidity_in_usd":1000000,"last_price":1}]`)

	if d := compare.Diff(agg, decodeJSON(`[{"liquidity_in_usd":1000000.5,"last_price":1}]`), tol); len(d) != 0 {
		t.Fatalf("5e-7 apart must match with the override, got %v", d)
	}

	if d := compare.Diff(agg, decodeJSON(`[{"liquidity_in_usd":1000002,"last_price":1}]`), tol); len(d) != 1 {
		t.Fatalf("2e-6 apart must differ, got %v", d)
	}

	// the override applies to its key only
	if d := compare.Diff(agg, decodeJSON(`[{"liquidity_in_usd":1000000,"last_price":1.0000005}]`), tol); len(d) != 1 || d[0].Path != "$[0].last_price" {
		t.Fatalf("last_price keeps the default tolerance, got %v", d)
	}
}

func TestDiffStructure(t *testing.T) {
	d := compare.Diff(
		decodeJSON(`{"a":1,"list":[1,2,3],"o":{"k":"v"}}`),
		decodeJSON(`{"b":1,"list":[1,5],"o":[]}`),
		nil,
	)

	want := map[string][2]string{
		"$.a":           {"1", "<missing>"},
		"$.b":           {"<missing>", "1"},
		"$.list.length": {"3", "2"},
		"$.list[1]":     {"2", "5"},
		"$.o":           {`{"k":"v"}`, "[]"},
	}

	if len(d) != len(want) {
		t.Fatalf("expected %d differences, got %+v", len(want), d)
	}

	for _, x := range d {
		w, ok := want[x.Path]
		if !ok || w[0] != x.Aggregator || w[1] != x.Scan {
			t.Errorf("unexpected difference %+v", x)
		}
	}
}
