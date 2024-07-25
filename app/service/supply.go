package service

import "errors"

type Cache interface {
}

type Supply struct {
	cache Cache
}

func NewSupplyService(cache Cache) (*Supply, error) {
	if cache == nil {
		return nil, errors.New("invalid cache instance provided to supply service")
	}

	return &Supply{cache}, nil
}
