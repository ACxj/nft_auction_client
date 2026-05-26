package service

import (
	"context"
	"errors"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	ErrConnectionLost  = errors.New("ethereum connection lost")
	ErrReconnectFailed = errors.New("failed to reconnect to ethereum")
)

type RetryClient struct {
	rpcURL        string
	client        *ethclient.Client
	mu            sync.RWMutex
	connected     bool
	reconnectChan chan struct{}
	stopChan      chan struct{}
	wg            sync.WaitGroup

	baseDelay  time.Duration
	maxDelay   time.Duration
	maxRetries int
}

func NewRetryClient(rpcURL string) (*RetryClient, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}

	rc := &RetryClient{
		rpcURL:        rpcURL,
		client:        client,
		connected:     true,
		reconnectChan: make(chan struct{}, 1),
		stopChan:      make(chan struct{}),
		baseDelay:     5 * time.Second,
		maxDelay:      60 * time.Second,
		maxRetries:    -1,
	}

	rc.wg.Add(1)
	go rc.monitorConnection()

	return rc, nil
}

func (rc *RetryClient) monitorConnection() {
	defer rc.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	reconnecting := false

	for {
		select {
		case <-rc.stopChan:
			return
		case <-ticker.C:
			if !rc.isConnected() && !reconnecting {
				rc.triggerReconnect()
			}
		case <-rc.reconnectChan:
			if !reconnecting {
				reconnecting = true
				go func() {
					if err := rc.reconnect(); err != nil {
						log.Printf("Reconnection failed: %v", err)
					}
					reconnecting = false
				}()
			}
		}
	}
}

func (rc *RetryClient) isConnected() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if rc.client == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := rc.client.BlockNumber(ctx)
	if err != nil {
		return false
	}

	return true
}

func (rc *RetryClient) triggerReconnect() {
	select {
	case rc.reconnectChan <- struct{}{}:
		log.Println("Reconnection triggered")
	default:
	}
}

func (rc *RetryClient) reconnect() error {
	delay := rc.baseDelay
	attempt := 0

	for {
		select {
		case <-rc.stopChan:
			return errors.New("reconnect stopped")
		default:
		}

		log.Printf("Attempting to reconnect to Ethereum (attempt %d, delay %v)", attempt+1, delay)

		client, err := ethclient.Dial(rc.rpcURL)
		if err != nil {
			log.Printf("Failed to dial Ethereum client: %v", err)
			time.Sleep(delay)
			delay = min(delay*2, rc.maxDelay)
			attempt++
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = client.BlockNumber(ctx)
		cancel()

		if err != nil {
			log.Printf("Failed to get block number: %v", err)
			client.Close()
			time.Sleep(delay)
			delay = min(delay*2, rc.maxDelay)
			attempt++
			continue
		}

		rc.mu.Lock()
		if rc.client != nil {
			rc.client.Close()
		}
		rc.client = client
		rc.connected = true
		rc.mu.Unlock()

		log.Println("Successfully reconnected to Ethereum")
		return nil
	}
}

func (rc *RetryClient) FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error) {
	rc.mu.RLock()
	client := rc.client
	connected := rc.connected
	rc.mu.RUnlock()

	if client == nil {
		return nil, ErrConnectionLost
	}

	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		rc.handleConnectionError()
		return nil, err
	}

	if !connected {
		rc.mu.Lock()
		rc.connected = true
		rc.mu.Unlock()
		log.Println("Connection restored")
	}

	return logs, nil
}

func (rc *RetryClient) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	rc.mu.RLock()
	client := rc.client
	rc.mu.RUnlock()

	if client == nil {
		return nil, ErrConnectionLost
	}

	block, err := client.BlockByNumber(ctx, number)
	if err != nil {
		rc.handleConnectionError()
		return nil, err
	}

	return block, nil
}

func (rc *RetryClient) BlockNumber(ctx context.Context) (uint64, error) {
	rc.mu.RLock()
	client := rc.client
	rc.mu.RUnlock()

	if client == nil {
		return 0, ErrConnectionLost
	}

	number, err := client.BlockNumber(ctx)
	if err != nil {
		rc.handleConnectionError()
		return 0, err
	}

	return number, nil
}

func (rc *RetryClient) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	rc.mu.RLock()
	client := rc.client
	rc.mu.RUnlock()

	if client == nil {
		return nil, ErrConnectionLost
	}

	header, err := client.HeaderByNumber(ctx, number)
	if err != nil {
		rc.handleConnectionError()
		return nil, err
	}

	return header, nil
}

func (rc *RetryClient) handleConnectionError() {
	rc.mu.Lock()
	if rc.connected {
		rc.connected = false
		log.Println("Connection lost, will attempt to reconnect")
	}
	rc.mu.Unlock()

	go func() {
		if err := rc.reconnect(); err != nil {
			log.Printf("Reconnection failed: %v", err)
		}
	}()
}

func (rc *RetryClient) Close() {
	close(rc.stopChan)
	rc.wg.Wait()

	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.client != nil {
		rc.client.Close()
		rc.client = nil
	}
}

func (rc *RetryClient) IsConnected() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.connected && rc.client != nil
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
