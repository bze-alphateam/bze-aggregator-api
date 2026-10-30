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

type BlockchainConfig struct {
	RestHost    string
	HealthNodes map[string]string
}

// ScanConfig is BZE Scan's base URL including its /aggregator prefix
// (SCAN_API_HOST).
type ScanConfig struct {
	Host string
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
	PrefixedEndpoints PrefixedEndpoints
	Scan              ScanConfig
}

func NewAppConfig() (*AppConfig, error) {
	envFile, err := godotenv.Read(".env")
	cfg := loadDefaultConfig(envFile, err)

	rest, ok := envFile["BLOCKCHAIN_REST_HOST"]
	if !ok {
		return nil, errors.New("BLOCKCHAIN_REST_HOST not found in .env")
	}

	scan, ok := envFile["SCAN_API_HOST"]
	if !ok {
		return nil, errors.New("SCAN_API_HOST not found in .env")
	}

	healthNodes, err := parseEnvVarMap(envFile, "HEALTH_NODES")
	if err != nil {
		return nil, err
	}

	cfg.Blockchain.HealthNodes = healthNodes
	cfg.Blockchain.RestHost = rest
	cfg.Scan.Host = scan

	cfg.PrefixedEndpoints, err = parseEnvVarMap(envFile, "PREFIXED_REST_HOSTS")
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func loadDefaultConfig(env map[string]string, err error) *AppConfig {
	port := defaultPort
	logLevel := defaultLoggingLevel
	if err == nil {
		if v, ok := env["HTTP_PORT"]; ok {
			port = v
		}
		if v, ok := env["LOG_LEVEL"]; ok {
			logLevel = v
		}
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
