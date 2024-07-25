package config

import (
	"errors"
	"github.com/joho/godotenv"
)

const (
	defaultPort         = "8000"
	defaultLoggingLevel = "info"
)

type BlockchainConfig struct {
	RestHost string
	RpcHost  string
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

	cfg.Blockchain = BlockchainConfig{
		RestHost: rest,
		RpcHost:  rpc,
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

	return &AppConfig{
		Server: Server{
			Port: port,
		},
		Logging: Logging{
			Level: logLevel,
		},
	}
}
