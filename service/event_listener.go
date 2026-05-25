package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"auction-backend/config"
	"auction-backend/database"
	"auction-backend/models"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

type EventListener struct {
	cfg        *config.Config
	client     *ethclient.Client
	auctionABI abi.ABI
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

var AuctionABI = `[{"type":"event","name":"AuctionCreated","inputs":[{"name":"auctionId","type":"uint256","indexed":false},{"name":"nftContract","type":"address","indexed":false},{"name":"tokenId","type":"uint256","indexed":false},{"name":"seller","type":"address","indexed":false},{"name":"startingPrice","type":"uint256","indexed":false},{"name":"endTime","type":"uint256","indexed":false},{"name":"acceptedToken","type":"address","indexed":false}]},{"type":"event","name":"BidPlaced","inputs":[{"name":"auctionId","type":"uint256","indexed":false},{"name":"bidder","type":"address","indexed":false},{"name":"amount","type":"uint256","indexed":false},{"name":"token","type":"address","indexed":false},{"name":"amountInUsd","type":"uint256","indexed":false}]},{"type":"event","name":"AuctionEnded","inputs":[{"name":"auctionId","type":"uint256","indexed":false},{"name":"winner","type":"address","indexed":false},{"name":"winningBid","type":"uint256","indexed":false},{"name":"token","type":"address","indexed":false}]},{"type":"event","name":"AuctionCanceled","inputs":[{"name":"auctionId","type":"uint256","indexed":false}]},{"type":"event","name":"WithdrawCompleted","inputs":[{"name":"user","type":"address","indexed":false},{"name":"amount","type":"uint256","indexed":false}]}]`

func NewEventListener(cfg *config.Config, client *ethclient.Client) (*EventListener, error) {
	parsedABI, err := abi.JSON(strings.NewReader(AuctionABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	return &EventListener{
		cfg:        cfg,
		client:     client,
		auctionABI: parsedABI,
		stopChan:   make(chan struct{}),
	}, nil
}

func (el *EventListener) Start() error {
	contractAddr := common.HexToAddress(el.cfg.ContractAddr)

	log.Printf("Starting event listener for contract: %s", contractAddr.Hex())
	log.Printf("Expected event signatures:")
	log.Printf("  AuctionCreated: %s", crypto.Keccak256Hash([]byte("AuctionCreated(uint256,address,uint256,address,uint256,uint256,address)")).Hex())
	log.Printf("  BidPlaced: %s", crypto.Keccak256Hash([]byte("BidPlaced(uint256,address,uint256,address,uint256)")).Hex())
	log.Printf("  AuctionEnded: %s", crypto.Keccak256Hash([]byte("AuctionEnded(uint256,address,uint256,address)")).Hex())
	log.Printf("  AuctionCanceled: %s", crypto.Keccak256Hash([]byte("AuctionCanceled(uint256)")).Hex())
	log.Printf("  WithdrawCompleted: %s", crypto.Keccak256Hash([]byte("WithdrawCompleted(address,uint256)")).Hex())

	el.wg.Add(1)
	go el.pollEvents(contractAddr)

	return nil
}

func (el *EventListener) pollEvents(contractAddr common.Address) {
	defer el.wg.Done()

	var lastBlock uint64

	for {
		select {
		case <-el.stopChan:
			log.Println("Event listener stopped")
			return
		default:
			query := ethereum.FilterQuery{
				Addresses: []common.Address{contractAddr},
				FromBlock: new(big.Int).SetUint64(lastBlock),
			}

			logs, err := el.client.FilterLogs(context.Background(), query)
			if err != nil {
				log.Printf("Failed to filter logs: %v", err)
				time.Sleep(15 * time.Second)
				continue
			}

			for _, vLog := range logs {
				el.handleLog(vLog)
				if vLog.BlockNumber > lastBlock {
					lastBlock = vLog.BlockNumber
				}
			}

			if len(logs) > 0 {
				log.Printf("Processed %d events up to block %d", len(logs), lastBlock)
			}

			time.Sleep(15 * time.Second)
		}
	}
}

func (el *EventListener) Stop() {
	close(el.stopChan)
	el.wg.Wait()
}

func (el *EventListener) handleLog(vLog types.Log) {
	eventID := vLog.Topics[0].Hex()

	switch eventID {
	case crypto.Keccak256Hash([]byte("AuctionCreated(uint256,address,uint256,address,uint256,uint256,address)")).Hex():
		el.handleAuctionCreated(vLog)
	case crypto.Keccak256Hash([]byte("BidPlaced(uint256,address,uint256,address,uint256)")).Hex():
		el.handleBidPlaced(vLog)
	case crypto.Keccak256Hash([]byte("AuctionEnded(uint256,address,uint256,address)")).Hex():
		el.handleAuctionEnded(vLog)
	case crypto.Keccak256Hash([]byte("AuctionCanceled(uint256)")).Hex():
		el.handleAuctionCanceled(vLog)
	case crypto.Keccak256Hash([]byte("WithdrawCompleted(address,uint256)")).Hex():
		el.handleWithdrawCompleted(vLog)
	default:
		log.Printf("Unknown event: %s (block: %d, tx: %s)", eventID, vLog.BlockNumber, vLog.TxHash.Hex())
	}
}

func (el *EventListener) handleAuctionCreated(vLog types.Log) {
	auctionData, err := el.auctionABI.Unpack("AuctionCreated", vLog.Data)
	if err != nil {
		log.Printf("Failed to unpack AuctionCreated event: %v", err)
		return
	}

	if len(auctionData) < 7 {
		log.Printf("Invalid AuctionCreated data length: %d", len(auctionData))
		return
	}

	auctionID := auctionData[0].(*big.Int)
	nftContract := auctionData[1].(common.Address)
	tokenId := auctionData[2].(*big.Int)
	seller := auctionData[3].(common.Address)
	startingPrice := auctionData[4].(*big.Int)
	endTime := auctionData[5].(*big.Int)

	auctionModel := &models.Auction{
		AuctionID:     auctionID.Uint64(),
		NFTAddress:    nftContract.Hex(),
		TokenID:       tokenId.Uint64(),
		Seller:        seller.Hex(),
		StartTime:     time.Now(),
		EndTime:       parseTimestamp(endTime),
		StartingPrice: float64(startingPrice.Int64()) / 1e18,
		IsActive:      true,
	}

	var count int64
	database.DB.Model(&models.Auction{}).Where("auction_id = ?", auctionID.Uint64()).Count(&count)
	if count > 0 {
		log.Printf("AuctionCreated (duplicate skipped): ID=%d", auctionID.Uint64())
		return
	}

	err = database.DB.Create(auctionModel).Error

	if err != nil {
		log.Printf("Failed to save auction: %v", err)
	} else {
		log.Printf("AuctionCreated: ID=%d, NFT=%s, TokenID=%d, Seller=%s", auctionID.Uint64(), nftContract.Hex(), tokenId.Uint64(), seller.Hex())
	}
}

func (el *EventListener) handleBidPlaced(vLog types.Log) {
	bidData, err := el.auctionABI.Unpack("BidPlaced", vLog.Data)
	if err != nil {
		log.Printf("Failed to unpack BidPlaced event: %v", err)
		return
	}

	if len(bidData) < 5 {
		log.Printf("Invalid BidPlaced data length: %d", len(bidData))
		return
	}

	auctionID := bidData[0].(*big.Int)
	bidder := bidData[1].(common.Address)
	amount := bidData[2].(*big.Int)
	token := bidData[3].(common.Address)
	amountUSD := bidData[4].(*big.Int)

	zeroAddress := common.HexToAddress("0x0000000000000000000000000000000000000000")
	isETH := (token == zeroAddress)

	bidModel := &models.Bid{
		AuctionID:   auctionID.Uint64(),
		Bidder:      bidder.Hex(),
		Amount:      float64(amount.Int64()) / 1e18,
		AmountUSD:   float64(amountUSD.Int64()) / 1e8,
		IsETH:       isETH,
		BlockNumber: vLog.BlockNumber,
		TxHash:      vLog.TxHash.Hex(),
		CreatedAt:   time.Now(),
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(bidModel).Error; err != nil {
			return fmt.Errorf("failed to save bid: %w", err)
		}

		if err := tx.Model(&models.Auction{}).Where("auction_id = ?", auctionID.Uint64()).Updates(map[string]interface{}{
			"highest_bidder":  bidder.Hex(),
			"highest_bid":     bidModel.Amount,
			"highest_bid_usd": bidModel.AmountUSD,
		}).Error; err != nil {
			return fmt.Errorf("failed to update auction: %w", err)
		}

		return nil
	})

	if err != nil {
		log.Printf("Failed to process bid: %v", err)
	} else {
		log.Printf("BidPlaced: AuctionID=%d, Bidder=%s, Amount=%.4f ETH, AmountUSD=%.2f", auctionID.Uint64(), bidder.Hex(), bidModel.Amount, bidModel.AmountUSD)
	}
}

func (el *EventListener) handleAuctionEnded(vLog types.Log) {
	resultData, err := el.auctionABI.Unpack("AuctionEnded", vLog.Data)
	if err != nil {
		log.Printf("Failed to unpack AuctionEnded event: %v", err)
		return
	}

	if len(resultData) < 4 {
		log.Printf("Invalid AuctionEnded data length: %d", len(resultData))
		return
	}

	auctionID := resultData[0].(*big.Int)
	winner := resultData[1].(common.Address)
	winningBid := resultData[2].(*big.Int)

	finalPrice := float64(winningBid.Int64()) / 1e18

	if err := database.DB.Model(&models.Auction{}).Where("auction_id = ?", auctionID.Uint64()).Updates(map[string]interface{}{
		"is_active":   false,
		"winner":      winner.Hex(),
		"final_price": finalPrice,
	}).Error; err != nil {
		log.Printf("Failed to update auction ended: %v", err)
	} else {
		log.Printf("AuctionEnded: ID=%d, Winner=%s, FinalPrice=%.4f ETH", auctionID.Uint64(), winner.Hex(), finalPrice)
	}
}

func (el *EventListener) handleAuctionCanceled(vLog types.Log) {
	data, err := el.auctionABI.Unpack("AuctionCanceled", vLog.Data)
	if err != nil {
		log.Printf("Failed to unpack AuctionCanceled event: %v", err)
		return
	}

	if len(data) < 1 {
		log.Printf("Invalid AuctionCanceled data length: %d", len(data))
		return
	}

	auctionID := data[0].(*big.Int)

	if err := database.DB.Model(&models.Auction{}).Where("auction_id = ?", auctionID.Uint64()).Update("is_active", false).Error; err != nil {
		log.Printf("Failed to update auction canceled: %v", err)
	} else {
		log.Printf("AuctionCanceled: ID=%d", auctionID.Uint64())
	}
}

func (el *EventListener) handleWithdrawCompleted(vLog types.Log) {
	data, err := el.auctionABI.Unpack("WithdrawCompleted", vLog.Data)
	if err != nil {
		log.Printf("Failed to unpack WithdrawCompleted event: %v", err)
		return
	}

	if len(data) < 2 {
		log.Printf("Invalid WithdrawCompleted data length: %d", len(data))
		return
	}

	user := data[0].(common.Address)
	amount := data[1].(*big.Int)

	log.Printf("WithdrawCompleted: User=%s, Amount=%.4f", user.Hex(), float64(amount.Int64())/1e18)
}

func parseTimestamp(ts *big.Int) time.Time {
	if ts == nil || ts.Int64() == 0 {
		return time.Now()
	}
	return time.Unix(ts.Int64(), 0)
}

func parseTimestampFromUint64(ts uint64) time.Time {
	return time.Unix(int64(ts), 0)
}

func (el *EventListener) SyncHistoricalEvents(startBlock uint64) error {
	contractAddr := common.HexToAddress(el.cfg.ContractAddr)

	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddr},
		FromBlock: new(big.Int).SetUint64(startBlock),
	}

	logs, err := el.client.FilterLogs(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to filter logs: %w", err)
	}

	log.Printf("Found %d historical events", len(logs))

	for _, vLog := range logs {
		el.handleLog(vLog)
	}

	return nil
}

func SyncAuctionsFromChain(el *EventListener) error {
	db := database.DB

	var latestBlock models.Bid
	if err := db.Order("block_number desc").First(&latestBlock).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Println("No bids found, starting from block 0")
			return el.SyncHistoricalEvents(0)
		}
		return err
	}

	log.Printf("Syncing from block %d", latestBlock.BlockNumber+1)
	return el.SyncHistoricalEvents(latestBlock.BlockNumber + 1)
}
