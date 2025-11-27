package data_provider

import (
	"context"
	"github.com/bze-alphateam/bze-aggregator-api/internal"
	"github.com/cometbft/cometbft/rpc/client/http"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
)

type BlockchainProvider struct {
	client *http.HTTP
}

func NewBlockchainProvider(client *http.HTTP) (*BlockchainProvider, error) {
	if client == nil {
		return nil, internal.NewInvalidDependenciesErr("NewBlockchainProvider")
	}

	return &BlockchainProvider{client: client}, nil
}

func (b BlockchainProvider) GetStatus() (*coretypes.ResultStatus, error) {
	return b.client.Status(context.Background())
}
