package health

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"cosmossdk.io/math"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/query"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto/request"
	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	cmtjson "github.com/tendermint/tendermint/libs/json"
)

const (
	balancesRoute = "/cosmos/bank/v1beta1/balances"
)

type BalancesHealth struct {
	endpoints config.PrefixedEndpoints
}

func NewBalancesHealth(endpoints config.PrefixedEndpoints) (*BalancesHealth, error) {
	return &BalancesHealth{
		endpoints: endpoints,
	}, nil
}

func (b *BalancesHealth) CheckBalances(params *request.BalanceHealthParams) []dto.AddressHealthCheck {
	wg := sync.WaitGroup{}
	mx := sync.RWMutex{}
	var response []dto.AddressHealthCheck
	for _, p := range params.Addresses {
		wg.Add(1)
		go func(p request.AddressBalanceParams) {
			defer wg.Done()
			result := dto.AddressHealthCheck{
				Address:   p.Address,
				IsHealthy: true,
				Error:     "",
			}

			minAmt := math.NewInt(p.MinAmount)
			balance, err := b.getAddressDenomBalance(p.Address, p.Denom)
			if err != nil {
				result.IsHealthy = false
				result.Error = err.Error()
			} else if balance.LT(minAmt) {
				result.IsHealthy = false
				result.Error = fmt.Sprintf("balance [%s] is less than min amount [%s]", balance.String(), minAmt.String())
			}

			mx.Lock()
			defer mx.Unlock()
			response = append(response, result)
		}(p)
	}

	wg.Wait()

	return response
}

func (b *BalancesHealth) getAddressDenomBalance(address, denom string) (math.Int, error) {
	allBalances, err := b.getAddressBalance(address)
	zero := math.ZeroInt()
	if err != nil {
		return zero, err
	}

	for _, balance := range allBalances.Balances {
		if balance.Denom == denom {
			amt, ok := math.NewIntFromString(balance.Amount)
			if !ok {
				return zero, fmt.Errorf("error parsing balance amount")
			}

			return amt, nil
		}
	}

	return zero, nil
}

func (b *BalancesHealth) getAddressBalance(address string) (*query.BalancesResponse, error) {
	url := b.getAddressEndpoint(address)
	if url == "" {
		return nil, fmt.Errorf("no endpoint found for provided address")
	}

	resp, err := http.Get(fmt.Sprintf("%s/%s/%s", url, balancesRoute, address))
	if err != nil {

		return nil, fmt.Errorf("error making request to Cosmos SDK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		return nil, fmt.Errorf("received non-OK status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var data query.BalancesResponse
	err = cmtjson.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling response data: %w", err)
	}

	return &data, nil
}

func (b *BalancesHealth) getAddressEndpoint(address string) string {
	for k, v := range b.endpoints {
		if strings.HasPrefix(address, k) {
			return v
		}
	}

	return ""
}
