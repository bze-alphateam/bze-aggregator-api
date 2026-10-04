package service

import (
	"errors"
	"io"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	"github.com/sirupsen/logrus"
)

type fakeCache struct {
	data map[string][]byte
}

func (f *fakeCache) Get(key string) ([]byte, error) { return f.data[key], nil }
func (f *fakeCache) Set(key string, data []byte, _ time.Duration) error {
	f.data[key] = data
	return nil
}

type fakeRestDataProvider struct {
	supply sdkmath.Int
	err    error
}

func (f *fakeRestDataProvider) GetTotalSupply(string) (sdkmath.Int, error) { return f.supply, f.err }
func (f *fakeRestDataProvider) GetCommunityPoolTotal(string) (float64, error) {
	return 0, nil
}

type fakeChainRegistry struct {
	exponent int
}

func (f *fakeChainRegistry) GetAssetDetails(denom string) (*chain_registry.ChainRegistryAsset, error) {
	return &chain_registry.ChainRegistryAsset{
		Base:    denom,
		Display: "display",
		DenomUnits: []chain_registry.ChainRegistryAssetDenom{
			{Denom: denom, Exponent: 0},
			{Denom: "display", Exponent: f.exponent},
		},
	}, nil
}

func newTestSupply(t *testing.T, supply string, exponent int) *Supply {
	t.Helper()
	amount, ok := sdkmath.NewIntFromString(supply)
	if !ok {
		t.Fatalf("invalid test supply %q", supply)
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	s, err := NewSupplyService(logger, &fakeCache{data: map[string][]byte{}}, &fakeRestDataProvider{supply: amount}, &fakeChainRegistry{exponent: exponent})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return s
}

func TestSupply_GetTotalSupply(t *testing.T) {
	tests := []struct {
		name     string
		supply   string
		exponent int
		want     string
	}{
		// same output as the old float64 + %.2f code
		{"rounds down", "123456789012345", 6, "123456789.01"},
		{"rounds up into the integer part", "1999999", 6, "2.00"},
		{"below one cent", "4", 6, "0.00"},
		{"zero", "0", 6, "0.00"},
		{"exponent zero", "12345", 0, "12345.00"},
		{"exponent one", "12345", 1, "1234.50"},
		{"LP exponent", "1836008062350109703", 12, "1836008.06"},
		// exact ties round half up (float64 + %.2f gave 1.00 and 2.67)
		{"tie 1.005", "1005000", 6, "1.01"},
		{"tie 2.675", "2675000", 6, "2.68"},
		// above int64 max, formatted exactly
		{"above int64 max", "10000000000000000000", 6, "10000000000000.00"},
		{"above int64 max with cents", "123456789012345678901234", 6, "123456789012345678.90"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newTestSupply(t, tt.supply, tt.exponent).GetTotalSupply("ufoo")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

func TestSupply_GetTotalSupply_ProviderError(t *testing.T) {
	s := newTestSupply(t, "0", 6)
	s.dataProvider = &fakeRestDataProvider{err: errors.New("boom")}

	got, err := s.GetTotalSupply("ufoo")
	if err != nil || got != "0" {
		t.Fatalf("expected \"0\" and no error, got %q, %v", got, err)
	}
}

func TestSupply_GetUTotalSupply(t *testing.T) {
	const huge = "123456789012345678901234"
	got, err := newTestSupply(t, huge, 12).GetUTotalSupply("ulp_foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != huge {
		t.Fatalf("expected %s, got %s", huge, got)
	}
}
