package config

import (
	"errors"
	"github.com/joho/godotenv"
	"strings"
)

const (
	defaultPort         = "8000"
	defaultLoggingLevel = "info"
)

type CoingeckoConfig struct {
	Host string
}

type PricesConfig struct {
	Denominations string
}

type BlockchainConfig struct {
	RestHost    string
	RpcHost     string
	GrpcHost    string
	HealthNodes map[string]string
}

type Logging struct {
	Level string
}

type Server struct {
	Port string
}

type AppConfig struct {
	Server     Server
	Logging    Logging
	Blockchain BlockchainConfig
	Prices     PricesConfig
	Coingecko  CoingeckoConfig
}

func NewAppConfig() (*AppConfig, error) {
	envFile, err := godotenv.Read(".env")
	cfg := loadDefaultConfig(envFile, err)
	rpc, ok := envFile["BLOCKCHAIN_RPC_HOST"]
	if !ok {
		return nil, errors.New("BLOCKCHAIN_RPC_HOST not found in .env")
	}

	rest, ok := envFile["BLOCKCHAIN_REST_HOST"]
	if !ok {
		return nil, errors.New("BLOCKCHAIN_REST_HOST not found in .env")
	}

	grpc, ok := envFile["BLOCKCHAIN_GRPC_HOST"]
	if !ok {
		return nil, errors.New("BLOCKCHAIN_GRPC_HOST not found in .env")
	}

	cg, ok := envFile["COINGECKO_HOST"]
	if !ok {
		return nil, errors.New("COINGECKO_HOST not found in .env")
	}

	hn, ok := envFile["HEALTH_NODES"]
	var healthNodes map[string]string
	if ok {
		nodes := strings.Split(hn, ",")
		if len(nodes) > 0 {
			healthNodes = make(map[string]string)
			for _, node := range nodes {
				nodeSplit := strings.Split(node, "=")
				if len(nodeSplit) != 2 || nodeSplit[0] == "" || nodeSplit[1] == "" {
					return nil, errors.New("HEALTH_NODES contains an unknown format")
				}

				healthNodes[nodeSplit[0]] = nodeSplit[1]
			}
		}
	}

	cfg.Blockchain = BlockchainConfig{
		RestHost:    rest,
		RpcHost:     rpc,
		GrpcHost:    grpc,
		HealthNodes: healthNodes,
	}

	cfg.Coingecko = CoingeckoConfig{
		Host: cg,
	}

	return cfg, nil
}

func loadDefaultConfig(env map[string]string, err error) *AppConfig {

	port := defaultPort
	logLevel := defaultLoggingLevel
	if err != nil {
		return &AppConfig{
			Server: Server{
				Port: port,
			},
			Logging: Logging{
				Level: logLevel,
			},
		}
	}

	port, ok := env["HTTP_PORT"]
	if !ok {
		port = defaultPort
	}

	logLevel, ok = env["LOG_LEVEL"]
	if !ok {
		logLevel = defaultLoggingLevel
	}

	prices, ok := env["COINGECKO_PRICE_IDS"]
	if !ok {
		prices = ""
	}

	return &AppConfig{
		Server: Server{
			Port: port,
		},
		Logging: Logging{
			Level: logLevel,
		},
		Prices: PricesConfig{
			Denominations: prices,
		},
	}
}
