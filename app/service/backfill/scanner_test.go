package backfill

import (
	"testing"
	"time"

	"github.com/bze-alphateam/bze-aggregator-api/app/dto/chain_registry"
	abci "github.com/cometbft/cometbft/abci/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
)

type fakeAssets struct{}

func (f fakeAssets) GetAssetDetails(denom string) (*chain_registry.ChainRegistryAsset, error) {
	return &chain_registry.ChainRegistryAsset{
		Base:    denom,
		Display: "display" + denom,
		DenomUnits: []chain_registry.ChainRegistryAssetDenom{
			{Denom: denom, Exponent: 0},
			{Denom: "display" + denom, Exponent: 6},
		},
	}, nil
}

func swapEvent(poolId string) abci.Event {
	return abci.Event{
		Type: swapEventType,
		Attributes: []abci.EventAttribute{
			{Key: "pool_id", Value: `"` + poolId + `"`},
			{Key: "creator", Value: `"bze1creator"`},
			{Key: "in", Value: `{"denom":"aaa","amount":"2000000"}`},
			{Key: "out", Value: `{"denom":"bbb","amount":"4000000"}`},
		},
	}
}

func TestExtractSwapEventsSkipsFailedTransactions(t *testing.T) {
	res := &coretypes.ResultBlockResults{
		TxsResults: []*abci.ExecTxResult{
			{Code: 0, Events: []abci.Event{{Type: "message"}, swapEvent("aaa_bbb")}},
			{Code: 5, Events: []abci.Event{swapEvent("ccc_ddd")}},
			nil,
			{Code: 0, Events: []abci.Event{swapEvent("eee_fff"), swapEvent("ggg_hhh")}},
		},
	}

	found := extractSwapEvents(42, res)
	if len(found) != 3 {
		t.Fatalf("expected 3 swap events, got %d", len(found))
	}

	// provenance has to point at the exact event, so a staged row can be
	// checked against the chain later
	if found[0].Height != 42 || found[0].TxIndex != 0 || found[0].EventIndex != 1 {
		t.Fatalf("unexpected provenance for the first event: %+v", found[0])
	}

	if found[2].TxIndex != 3 || found[2].EventIndex != 1 {
		t.Fatalf("unexpected provenance for the last event: %+v", found[2])
	}

	for _, raw := range found {
		if len(raw.Attributes) != 4 {
			t.Fatalf("expected the attributes to be carried over, got %+v", raw.Attributes)
		}
	}
}

func TestExtractSwapEventsIgnoresEverythingElse(t *testing.T) {
	res := &coretypes.ResultBlockResults{
		TxsResults: []*abci.ExecTxResult{
			{Code: 0, Events: []abci.Event{{Type: "bze.tradebin.OrderExecutedEvent"}, {Type: "transfer"}}},
		},
	}

	if found := extractSwapEvents(1, res); len(found) != 0 {
		t.Fatalf("expected no swap events, got %d", len(found))
	}

	if found := extractSwapEvents(1, nil); len(found) != 0 {
		t.Fatalf("expected no swap events for a nil result, got %d", len(found))
	}
}

func TestToStagedSwapUsesTheBlockTime(t *testing.T) {
	executedAt := time.Date(2026, 8, 1, 9, 3, 0, 0, time.UTC)
	raw := extractSwapEvents(1234, &coretypes.ResultBlockResults{
		TxsResults: []*abci.ExecTxResult{{Code: 0, Events: []abci.Event{swapEvent("aaa_bbb")}}},
	})[0]

	row, err := toStagedSwap(fakeAssets{}, raw, executedAt)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if row.MarketID != "aaa_bbb" {
		t.Fatalf("expected the pool id as market id, got %s", row.MarketID)
	}

	if !row.ExecutedAt.Equal(executedAt) {
		t.Fatalf("expected the block time as executed_at, got %s", row.ExecutedAt)
	}

	if row.Height != 1234 || row.TxIndex != 0 || row.EventIndex != 0 {
		t.Fatalf("provenance was not carried over: %+v", row)
	}

	if row.Taker != "bze1creator" || row.Amount == "" || row.Price == "" || row.QuoteAmount == "" {
		t.Fatalf("row was not converted properly: %+v", row)
	}
}

func TestToStagedSwapFailsOnBrokenAttributes(t *testing.T) {
	raw := rawSwapEvent{Height: 1}
	if _, err := toStagedSwap(fakeAssets{}, raw, time.Now()); err == nil {
		t.Fatal("expected an error for an event without attributes")
	}
}

func TestExtractSwapEventsIncludesBlockHookSwaps(t *testing.T) {
	// the hourly tx-fee conversion and the burner swap through the pools from
	// block hooks: those events carry no tx, and the live sync stores them like
	// any other swap
	res := &coretypes.ResultBlockResults{
		FinalizeBlockEvents: []abci.Event{{Type: "coin_received"}, swapEvent("aaa_bbb")},
		TxsResults:          []*abci.ExecTxResult{{Code: 0, Events: []abci.Event{swapEvent("ccc_ddd")}}},
	}

	found := extractSwapEvents(7, res)
	if len(found) != 2 {
		t.Fatalf("expected the block swap and the tx swap, got %d", len(found))
	}

	// block events sort before the block's transactions, like the Postgres
	// indexer stores them
	if found[0].TxIndex != blockEventTxIndex || found[0].EventIndex != 1 {
		t.Fatalf("unexpected provenance for the block event: %+v", found[0])
	}

	if found[1].TxIndex != 0 {
		t.Fatalf("unexpected provenance for the tx event: %+v", found[1])
	}
}
