package powchain

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	gethRPC "github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/config/params"
	contracts "github.com/prysmaticlabs/prysm/contracts/deposit"
	"github.com/prysmaticlabs/prysm/io/logs"
	"github.com/prysmaticlabs/prysm/network"
	"github.com/prysmaticlabs/prysm/network/authorization"
)

func (s *Service) setupExecutionClientConnections(ctx context.Context, currEndpoint network.Endpoint) error {
	client, err := s.newRPCClientWithAuth(ctx, currEndpoint)
	if err != nil {
		return errors.Wrap(err, "could not dial execution node")
	}
	// Attach the clients to the service struct.
	fetcher := ethclient.NewClient(client)
	s.rpcClient = client
	s.httpLogger = fetcher
	s.eth1DataFetcher = fetcher

	depositContractCaller, err := contracts.NewDepositContractCaller(s.cfg.depositContractAddr, fetcher)
	if err != nil {
		client.Close()
		return errors.Wrap(err, "could not initialize deposit contract caller")
	}
	s.depositContractCaller = depositContractCaller

	// Ensure we have the correct chain and deposit IDs.
	if err := ensureCorrectExecutionChain(ctx, fetcher); err != nil {
		client.Close()
		return errors.Wrap(err, "could not make initial request to verify execution chain ID")
	}
	s.updateConnectedETH1(true)
	s.runError = nil
	return nil
}

// Every N seconds, defined as a backoffPeriod, attempts to re-establish an execution client
// connection and if this does not work, we fallback to the next endpoint if defined.
func (s *Service) pollConnectionStatus(ctx context.Context) {
	// Use a custom logger to only log errors
	logCounter := 0
	errorLogger := func(err error, msg string) {
		if logCounter > logThreshold {
			log.Errorf("%s: %v", msg, err)
			logCounter = 0
		}
		logCounter++
	}
	ticker := time.NewTicker(backOffPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			log.Debugf("Trying to dial endpoint: %s", logs.MaskCredentialsLogging(s.cfg.currHttpEndpoint.Url))
			if err := s.setupExecutionClientConnections(ctx, s.cfg.currHttpEndpoint); err != nil {
				errorLogger(err, "Could not connect to execution client endpoint")
				s.runError = err
				s.fallbackToNextEndpoint()
			}
		case <-s.ctx.Done():
			log.Debug("Received cancelled context,closing existing powchain service")
			return
		}
	}
}

// Forces to retry an execution client connection.
func (s *Service) retryExecutionClientConnection(ctx context.Context, err error) {
	s.runError = err
	s.updateConnectedETH1(false)
	// Back off for a while before redialing.
	time.Sleep(backOffPeriod)
	if err := s.setupExecutionClientConnections(ctx, s.cfg.currHttpEndpoint); err != nil {
		s.runError = err
		return
	}
	// Reset run error in the event of a successful connection.
	s.runError = nil
}

// This performs a health check on our primary endpoint, and if it
// is ready to serve we connect to it again. This method is only
// relevant if we are on our backup endpoint.
func (s *Service) checkDefaultEndpoint(ctx context.Context) {
	primaryEndpoint := s.cfg.httpEndpoints[0]
	// Return early if we are running on our primary
	// endpoint.
	if s.cfg.currHttpEndpoint.Equals(primaryEndpoint) {
		return
	}

	if err := s.setupExecutionClientConnections(ctx, primaryEndpoint); err != nil {
		log.Debugf("Primary endpoint not ready: %v", err)
		return
	}
	s.updateCurrHttpEndpoint(primaryEndpoint)
}

// This is an inefficient way to search for the next endpoint, but given N is
// expected to be small, it is fine to search this way.
func (s *Service) fallbackToNextEndpoint() {
	currEndpoint := s.cfg.currHttpEndpoint
	currIndex := 0
	totalEndpoints := len(s.cfg.httpEndpoints)

	for i, endpoint := range s.cfg.httpEndpoints {
		if endpoint.Equals(currEndpoint) {
			currIndex = i
			break
		}
	}
	nextIndex := currIndex + 1
	if nextIndex >= totalEndpoints {
		nextIndex = 0
	}
	s.updateCurrHttpEndpoint(s.cfg.httpEndpoints[nextIndex])
	if nextIndex != currIndex {
		log.Infof("Falling back to alternative endpoint: %s", logs.MaskCredentialsLogging(s.cfg.currHttpEndpoint.Url))
	}
}

// Initializes an RPC connection with authentication headers.
func (s *Service) newRPCClientWithAuth(ctx context.Context, endpoint network.Endpoint) (*gethRPC.Client, error) {
	// Need to handle ipc and http
	var client *gethRPC.Client
	u, err := url.Parse(endpoint.Url)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "http", "https":
		client, err = gethRPC.DialHTTPWithClient(endpoint.Url, endpoint.HttpClient())
		if err != nil {
			return nil, err
		}
	case "":
		client, err = gethRPC.DialIPC(ctx, endpoint.Url)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("no known transport for URL scheme %q", u.Scheme)
	}
	if endpoint.Auth.Method != authorization.None {
		header, err := endpoint.Auth.ToHeaderValue()
		if err != nil {
			return nil, err
		}
		client.SetHeader("Authorization", header)
	}
	return client, nil
}

// Checks the chain ID of the execution client to ensure
// it matches local parameters of what Prysm expects.
func ensureCorrectExecutionChain(ctx context.Context, client *ethclient.Client) error {
	cID, err := client.ChainID(ctx)
	if err != nil {
		return err
	}
	wantChainID := params.BeaconConfig().DepositChainID
	if cID.Uint64() != wantChainID {
		return fmt.Errorf("wanted chain ID %d, got %d", wantChainID, cID.Uint64())
	}
	return nil
}
