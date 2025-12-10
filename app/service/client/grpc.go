package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/bze-alphateam/bze-aggregator-api/server/config"
	tradebinTypes "github.com/bze-alphateam/bze/x/tradebin/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
)

const (
	lockName = "grpc:client:connection"
)

type ConnectionLocker interface {
	Lock(key string)
	Unlock(key string)
}

type GrpcClient struct {
	host   string
	locker ConnectionLocker
	conn   *grpc.ClientConn
	logger logrus.FieldLogger
	useTLS bool
}

func NewGrpcClient(cfg *config.AppConfig, locker ConnectionLocker, logger logrus.FieldLogger) (*GrpcClient, error) {
	if cfg.Blockchain.GrpcHost == "" {
		return nil, fmt.Errorf("grpc host is required")
	}

	if locker == nil {
		return nil, fmt.Errorf("grpc client requires locker")
	}

	if logger == nil {
		return nil, fmt.Errorf("grpc client requires logger")
	}

	return &GrpcClient{
		host:   cfg.Blockchain.GrpcHost,
		locker: locker,
		logger: logger,
		useTLS: cfg.Blockchain.UseGrpcTls,
	}, nil
}

// LoadTLSCredentials loads TLS credentials with ALPN support for HTTP/2
func (c *GrpcClient) loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Load system CA certificates or specific certs
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	// Create the TLS config, explicitly specifying HTTP/2 via ALPN
	tlsConfig := &tls.Config{
		RootCAs:            certPool,       // Use system CAs
		NextProtos:         []string{"h2"}, // HTTP/2 (gRPC requires this)
		InsecureSkipVerify: false,          // Verify server certificate
	}

	// Return the transport credentials for gRPC to use
	return credentials.NewTLS(tlsConfig), nil
}

func (c *GrpcClient) getConnection() (*grpc.ClientConn, error) {
	//make it thread safe
	c.locker.Lock(lockName)
	defer c.locker.Unlock(lockName)
	if c.conn != nil && c.conn.GetState() == connectivity.Ready {
		c.logger.Debug("grpc client connection ready")

		return c.conn, nil
	}

	if c.conn != nil {
		c.logger.Info("grpc client connection exists with status:", c.conn.GetState().String())
	}

	c.logger.Info("connecting to grpc host:", c.host)

	var dialOptions []grpc.DialOption
	if c.useTLS {
		cred, err := c.loadTLSCredentials()
		if err != nil {
			return nil, err
		}

		dialOptions = append(dialOptions, grpc.WithTransportCredentials(cred))
	} else {
		dialOptions = append(dialOptions, grpc.WithInsecure())
	}

	grpcConn, err := grpc.Dial(
		c.host,
		dialOptions...,
	)

	if err != nil {
		return nil, err
	}

	c.conn = grpcConn

	return grpcConn, nil
}

func (c *GrpcClient) GetTradebinQueryClient() (tradebinTypes.QueryClient, error) {
	grpcConn, err := c.getConnection()
	if err != nil {
		return nil, err
	}

	queryClient := tradebinTypes.NewQueryClient(grpcConn)

	return queryClient, nil
}

func (c *GrpcClient) GetBankQueryClient() (banktypes.QueryClient, error) {
	grpcConn, err := c.getConnection()
	if err != nil {
		return nil, err
	}

	queryClient := banktypes.NewQueryClient(grpcConn)

	return queryClient, nil
}

func (c *GrpcClient) CloseConnection() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
