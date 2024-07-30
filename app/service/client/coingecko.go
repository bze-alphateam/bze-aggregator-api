package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"io/ioutil"
	"net/http"
)

const (
	defaultPricesDenomination = "bzedge"
	againstCurrency           = "usd"

	pricesPath = "/api/v3/simple/price?ids=%s&vs_currencies=%s"
)

type Coingecko struct {
	ids  string
	host string
}

func NewCoingeckoClient(host, ids string) (*Coingecko, error) {
	if len(host) == 0 {
		return nil, errors.New("invalid host provided to Coingecko client")
	}

	//use default value if none provided
	if len(ids) == 0 {
		ids = defaultPricesDenomination
	}

	return &Coingecko{ids: ids, host: host}, nil
}

func (c *Coingecko) GetDenominationsPrices() ([]dto.CoinPrice, error) {
	url := fmt.Sprintf("%s%s", c.host, fmt.Sprintf(pricesPath, c.ids, againstCurrency))
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error making request to coingecko: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-OK status code from coingeko: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body from coingecko: %w", err)
	}

	//do not use a structure because we might want to use Eur or other "against" currencies in the future
	data := make(map[string]map[string]float64)
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling response data from coingecko: %w", err)
	}

	var prices []dto.CoinPrice
	for id, priceData := range data {
		price, ok := priceData[againstCurrency]
		if !ok {
			continue
		}

		cp := dto.CoinPrice{
			Denom:      id,
			Price:      price,
			PriceDenom: againstCurrency,
		}

		prices = append(prices, cp)
	}

	return prices, nil
}
