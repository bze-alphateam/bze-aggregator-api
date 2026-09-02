package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultPort         = "8000"
	defaultLoggingLevel = "info"
)

type PrefixedEndpoints map[string]string

type CoingeckoConfig struct {
	Host string
}

type PricesConfig struct {
	Denominations string
	NativeDenom   string
	UsdcDenom     string
}

type BlockchainConfig struct {
	RestHost    string
	RpcHost     string
	GrpcHost    string
	HealthNodes map[string]string

	UseGrpcTls bool
}

type Logging struct {
	Level string
}

type Server struct {
	Port string
}

type AppConfig struct {
	Server            Server
	Logging           Logging
	Blockchain        BlockchainConfig
	Prices            PricesConfig
	Coingecko         CoingeckoConfig
	PrefixedEndpoints PrefixedEndpoints
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

	useGrpcTls, ok := envFile["BLOCKCHAIN_GRPC_USE_TLS"]
	if ok {
		cfg.Blockchain.UseGrpcTls = useGrpcTls == "true"
	}

	healthNodes, err := parseEnvVarMap(envFile, "HEALTH_NODES")
	if err != nil {
		return nil, err
	}

	cfg.Blockchain.HealthNodes = healthNodes
	cfg.Blockchain.RestHost = rest
	cfg.Blockchain.RpcHost = rpc
	cfg.Blockchain.GrpcHost = grpc

	cfg.Coingecko = CoingeckoConfig{
		Host: cg,
	}

	cfg.PrefixedEndpoints, err = parseEnvVarMap(envFile, "PREFIXED_REST_HOSTS")
	if err != nil {
		return nil, err
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

	nativeDenom, ok := env["NATIVE_DENOM"]
	if !ok {
		nativeDenom = ""
	}

	usdcDenom, ok := env["USDC_DENOM"]
	if !ok {
		usdcDenom = ""
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
			NativeDenom:   nativeDenom,
			UsdcDenom:     usdcDenom,
		},
	}
}

func parseEnvVarMap(envFile map[string]string, envVar string) (map[string]string, error) {
	envValue, ok := envFile[envVar]
	var result map[string]string
	if ok {
		nodes := strings.Split(envValue, ",")
		if len(nodes) > 0 {
			result = make(map[string]string)
			for _, node := range nodes {
				nodeSplit := strings.Split(node, "=")
				if len(nodeSplit) != 2 || nodeSplit[0] == "" || nodeSplit[1] == "" {
					return nil, errors.New(fmt.Sprintf("env var %s contains an unknown format: %s", envVar, envValue))
				}

				result[nodeSplit[0]] = nodeSplit[1]
			}
		}
	}

	return result, nil
}
