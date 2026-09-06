package converter

import (
	"testing"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/entity"
)

func TestPoolIdToDenoms(t *testing.T) {
	for _, tc := range []struct {
		name        string
		poolId      string
		base, quote string
		wantErr     bool
	}{
		{name: "plain denoms", poolId: "aaa_bbb", base: "aaa", quote: "bbb"},
		{name: "factory base keeps its slashes", poolId: factoryDenom + "_" + nativeDenom, base: factoryDenom, quote: nativeDenom},
		{name: "no separator", poolId: "aaa", wantErr: true},
		// a factory denom on both sides is not representable: the id then holds
		// two separators and cannot be split back
		{name: "two separators", poolId: "aaa_bbb_ccc", wantErr: true},
		{name: "empty", poolId: "", wantErr: true},
		// an empty half still splits into exactly two parts
		{name: "empty quote", poolId: "aaa_", base: "aaa", quote: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, quote, err := PoolIdToDenoms(tc.poolId)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error for pool id %q", tc.poolId)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if base != tc.base || quote != tc.quote {
				t.Fatalf("expected %q/%q, got %q/%q", tc.base, tc.quote, base, quote)
			}
		})
	}
}

func swapEventAttributes() []entity.EventAttribute {
	// the node's event index stores attribute values JSON encoded, so plain
	// strings arrive wrapped in quotes and coins as objects
	return []entity.EventAttribute{
		{Key: "pool_id", Value: `"aaa_bbb"`},
		{Key: "creator", Value: `"bze1creator"`},
		{Key: "in", Value: `{"denom":"aaa","amount":"2000000"}`},
		{Key: "out", Value: `{"denom":"bbb","amount":"4000000"}`},
	}
}

func swapEvent() *entity.Event {
	return &entity.Event{
		RowID:     77,
		Type:      "bze.tradebin.SwapEvent",
		CreatedAt: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

func TestConvertEventToSwapData(t *testing.T) {
	event := swapEvent()

	data, err := ConvertEventToSwapData(event, swapEventAttributes())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.EventID != event.RowID {
		t.Fatalf("expected the event row id to be carried over, got %d", data.EventID)
	}

	if !data.ExecutedAt.Equal(event.CreatedAt) {
		t.Fatalf("expected the event time to be carried over, got %s", data.ExecutedAt)
	}

	// the surrounding JSON quotes must not survive into the stored values
	if data.PoolID != "aaa_bbb" {
		t.Fatalf("expected pool id %q, got %q", "aaa_bbb", data.PoolID)
	}

	if data.Creator != "bze1creator" {
		t.Fatalf("expected creator %q, got %q", "bze1creator", data.Creator)
	}

	if data.Input.Denom != "aaa" || data.Input.Amount.String() != "2000000" {
		t.Fatalf("unexpected input coin: %v", data.Input)
	}

	if data.Output.Denom != "bbb" || data.Output.Amount.String() != "4000000" {
		t.Fatalf("unexpected output coin: %v", data.Output)
	}
}

func TestConvertEventToSwapDataIgnoresUnknownAttributes(t *testing.T) {
	attributes := append(swapEventAttributes(), entity.EventAttribute{Key: "unrelated", Value: `"noise"`})

	data, err := ConvertEventToSwapData(swapEvent(), attributes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.PoolID != "aaa_bbb" {
		t.Fatalf("expected the known attributes to still be parsed, got %+v", data)
	}
}

// a zero amount is a valid coin, so it is accepted — only invalid coins are
// rejected
func TestConvertEventToSwapDataAcceptsZeroAmounts(t *testing.T) {
	attributes := swapEventAttributes()
	attributes[2].Value = `{"denom":"aaa","amount":"0"}`

	data, err := ConvertEventToSwapData(swapEvent(), attributes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !data.Input.Amount.IsZero() {
		t.Fatalf("expected a zero input amount, got %v", data.Input.Amount)
	}
}

func TestConvertEventToSwapDataErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		attributes func() []entity.EventAttribute
	}{
		{
			name:       "no attributes at all",
			attributes: func() []entity.EventAttribute { return nil },
		},
		{
			name: "missing creator",
			attributes: func() []entity.EventAttribute {
				return swapEventAttributes()[:1]
			},
		},
		{
			name: "missing out coin",
			attributes: func() []entity.EventAttribute {
				return swapEventAttributes()[:3]
			},
		},
		{
			name: "empty pool id",
			attributes: func() []entity.EventAttribute {
				a := swapEventAttributes()
				a[0].Value = `""`
				return a
			},
		},
		{
			name: "malformed in coin",
			attributes: func() []entity.EventAttribute {
				a := swapEventAttributes()
				a[2].Value = `not-json`
				return a
			},
		},
		{
			name: "malformed out coin",
			attributes: func() []entity.EventAttribute {
				a := swapEventAttributes()
				a[3].Value = `{"denom":"bbb","amount":`
				return a
			},
		},
		{
			name: "negative amount is not a valid coin",
			attributes: func() []entity.EventAttribute {
				a := swapEventAttributes()
				a[2].Value = `{"denom":"aaa","amount":"-5"}`
				return a
			},
		},
		{
			name: "denom too short to be valid",
			attributes: func() []entity.EventAttribute {
				a := swapEventAttributes()
				a[2].Value = `{"denom":"a","amount":"5"}`
				return a
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := ConvertEventToSwapData(swapEvent(), tc.attributes())
			if err == nil {
				t.Fatalf("expected an error, got %+v", data)
			}

			if data != nil {
				t.Fatalf("expected no data alongside the error, got %+v", data)
			}
		})
	}
}
