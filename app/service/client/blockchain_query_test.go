package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
)

// nodeDefaultPageSize mirrors the Cosmos REST default when pagination.limit is not sent.
const nodeDefaultPageSize = 100

// newSupplyServer serves supplyPath like a node would: the given coins, cut to
// the first page unless pagination.limit asks for more.
func newSupplyServer(t *testing.T, coins []dto.Coin) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != supplyPath {
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		limit := nodeDefaultPageSize
		if l := r.URL.Query().Get("pagination.limit"); l != "" {
			if l != fmt.Sprintf("%d", supplyPageLimit) {
				t.Errorf("expected pagination.limit=%d, got %s", supplyPageLimit, l)
			}
			fmt.Sscanf(l, "%d", &limit)
		}

		page := coins
		if len(page) > limit {
			page = page[:limit]
		}
		_ = json.NewEncoder(w).Encode(supplyResponse{Amount: page})
	}))
	t.Cleanup(srv.Close)

	return srv
}

func manyCoins(n int) []dto.Coin {
	coins := make([]dto.Coin, 0, n)
	for i := 0; i < n; i++ {
		coins = append(coins, dto.Coin{Denom: fmt.Sprintf("udenom%03d", i), Amount: fmt.Sprintf("%d", i+1)})
	}

	return coins
}

func newTestClient(t *testing.T, host string) *BlockchainQueryClient {
	t.Helper()
	c, err := NewBlockchainQueryClient(host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return c
}

func TestGetTotalSupply_DenomAfterFirstPage(t *testing.T) {
	coins := append(manyCoins(150), dto.Coin{Denom: "ulp_late", Amount: "4242"})
	c := newTestClient(t, newSupplyServer(t, coins).URL)

	got, err := c.GetTotalSupply("ulp_late")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "4242" {
		t.Fatalf("expected 4242, got %s", got)
	}
}

func TestGetTotalSupply_AboveInt64Max(t *testing.T) {
	const huge = "10000000000000000000"
	c := newTestClient(t, newSupplyServer(t, []dto.Coin{{Denom: "ulp_big", Amount: huge}}).URL)

	got, err := c.GetTotalSupply("ulp_big")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != huge {
		t.Fatalf("expected %s, got %s", huge, got)
	}
}

func TestGetTotalSupply_DenomNotFound(t *testing.T) {
	c := newTestClient(t, newSupplyServer(t, manyCoins(3)).URL)

	_, err := c.GetTotalSupply("umissing")
	if err == nil || !strings.Contains(err.Error(), "denom umissing not found") {
		t.Fatalf("expected a not found error, got %v", err)
	}
}

func TestGetTotalSupply_InvalidAmount(t *testing.T) {
	c := newTestClient(t, newSupplyServer(t, []dto.Coin{{Denom: "ubad", Amount: "12x"}}).URL)

	if _, err := c.GetTotalSupply("ubad"); err == nil {
		t.Fatalf("expected a parse error")
	}
}

func TestGetTotalSupply_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	c := newTestClient(t, srv.URL)

	_, err := c.GetTotalSupply("ubze")
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("expected a non-OK status error, got %v", err)
	}
}
