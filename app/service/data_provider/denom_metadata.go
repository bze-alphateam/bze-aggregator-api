package data_provider

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/types/query"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/sirupsen/logrus"
)

type metadataClientProvider interface {
	GetBankQueryClient() (banktypes.QueryClient, error)
}

type DenomMetadata struct {
	provider metadataClientProvider
	logger   logrus.FieldLogger
}

func NewDenomMetadataProvider(cl metadataClientProvider, logger logrus.FieldLogger) (*DenomMetadata, error) {
	if cl == nil || logger == nil {
		return nil, fmt.Errorf("invalid dependencies provided to NewDenomMetadataProvider")
	}

	return &DenomMetadata{
		provider: cl,
		logger:   logger.WithField("service", "DataProvider.DenomMetadata"),
	}, nil
}

func (d *DenomMetadata) GetAllDenomsMetadata() ([]banktypes.Metadata, error) {
	qc, err := d.provider.GetBankQueryClient()
	if err != nil {
		return nil, err
	}

	params := d.getDenomsMetadataParams(500, "", false)
	d.logger.Info("fetching denoms metadata from blockchain")

	res, err := qc.DenomsMetadata(context.Background(), params)
	if err != nil {
		return nil, err
	}
	d.logger.Info("fetched denoms metadata")

	return res.GetMetadatas(), nil
}

func (d *DenomMetadata) getDenomsMetadataParams(limit uint64, key string, reverse bool) *banktypes.QueryDenomsMetadataRequest {
	res := banktypes.QueryDenomsMetadataRequest{
		Pagination: &query.PageRequest{
			Limit:      limit,
			Reverse:    reverse,
			CountTotal: false,
		},
	}

	if key != "" {
		res.Pagination.Key = []byte(key)
	}

	return &res
}
