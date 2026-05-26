package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"auction-backend/database"
	"auction-backend/models"

	"github.com/gin-gonic/gin"
)

type AuctionListResponse struct {
	Total int64             `json:"total"`
	Page  int               `json:"page"`
	Size  int               `json:"size"`
	Data  []AuctionResponse `json:"data"`
}

type AuctionResponse struct {
	ID            uint    `json:"id"`
	AuctionID     uint64  `json:"auction_id"`
	NFTAddress    string  `json:"nft_address"`
	TokenID       uint64  `json:"token_id"`
	Seller        string  `json:"seller"`
	StartTime     string  `json:"start_time"`
	EndTime       string  `json:"end_time"`
	StartingPrice float64 `json:"starting_price"`
	HighestBidder string  `json:"highest_bidder"`
	HighestBid    float64 `json:"highest_bid"`
	HighestBidUSD float64 `json:"highest_bid_usd"`
	IsActive      bool    `json:"is_active"`
	Winner        string  `json:"winner"`
	FinalPrice    float64 `json:"final_price"`
}

func toAuctionResponse(a models.Auction) AuctionResponse {
	return AuctionResponse{
		ID:            a.ID,
		AuctionID:     a.AuctionID,
		NFTAddress:    a.NFTAddress,
		TokenID:       a.TokenID,
		Seller:        a.Seller,
		StartTime:     a.StartTime.Format("2006-01-02 15:04:05"),
		EndTime:       a.EndTime.Format("2006-01-02 15:04:05"),
		StartingPrice: a.StartingPrice,
		HighestBidder: a.HighestBidder,
		HighestBid:    a.HighestBid,
		HighestBidUSD: a.HighestBidUSD,
		IsActive:      a.IsActive,
		Winner:        a.Winner,
		FinalPrice:    a.FinalPrice,
	}
}

func toBidResponse(b models.Bid) BidResponse {
	return BidResponse{
		ID:          b.ID,
		AuctionID:   b.AuctionID,
		Bidder:      b.Bidder,
		Amount:      b.Amount,
		AmountUSD:   b.AmountUSD,
		IsETH:       b.IsETH,
		BlockNumber: b.BlockNumber,
		TxHash:      b.TxHash,
		CreatedAt:   b.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func queryWithTimeout(ctx *gin.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx.Request.Context(), timeout)
}

func GetAuctions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	active := c.Query("active")

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 10
	}

	offset := (page - 1) * size

	dbCtx, cancel := queryWithTimeout(c, 5*time.Second)
	defer cancel()

	var total int64
	var auctions []models.Auction
	query := database.DB.Model(&models.Auction{}).WithContext(dbCtx)

	if active == "true" {
		query = query.Where("is_active = ?", true)
	} else if active == "false" {
		query = query.Where("is_active = ?", false)
	}

	query.Count(&total)
	query.Order("created_at DESC").Offset(offset).Limit(size).Find(&auctions)

	var response []AuctionResponse
	for _, a := range auctions {
		response = append(response, toAuctionResponse(a))
	}

	c.JSON(http.StatusOK, AuctionListResponse{
		Total: total,
		Page:  page,
		Size:  size,
		Data:  response,
	})
}

func GetAuctionByID(c *gin.Context) {
	auctionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid auction ID"})
		return
	}

	dbCtx, cancel := queryWithTimeout(c, 5*time.Second)
	defer cancel()

	var auction models.Auction
	if err := database.DB.WithContext(dbCtx).Where("auction_id = ?", auctionID).First(&auction).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "auction not found"})
		return
	}

	c.JSON(http.StatusOK, toAuctionResponse(auction))
}

type BidHistoryResponse struct {
	Total int64         `json:"total"`
	Page  int           `json:"page"`
	Size  int           `json:"size"`
	Data  []BidResponse `json:"data"`
}

type BidResponse struct {
	ID          uint    `json:"id"`
	AuctionID   uint64  `json:"auction_id"`
	Bidder      string  `json:"bidder"`
	Amount      float64 `json:"amount"`
	AmountUSD   float64 `json:"amount_usd"`
	IsETH       bool    `json:"is_eth"`
	BlockNumber uint64  `json:"block_number"`
	TxHash      string  `json:"tx_hash"`
	CreatedAt   string  `json:"created_at"`
}

func GetBidHistory(c *gin.Context) {
	auctionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid auction ID"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 10
	}

	offset := (page - 1) * size

	dbCtx, cancel := queryWithTimeout(c, 5*time.Second)
	defer cancel()

	var total int64
	var bids []models.Bid
	query := database.DB.Model(&models.Bid{}).WithContext(dbCtx).Where("auction_id = ?", auctionID)

	query.Count(&total)
	query.Order("created_at DESC").Offset(offset).Limit(size).Find(&bids)

	var response []BidResponse
	for _, b := range bids {
		response = append(response, toBidResponse(b))
	}

	c.JSON(http.StatusOK, BidHistoryResponse{
		Total: total,
		Page:  page,
		Size:  size,
		Data:  response,
	})
}

func GetBidsByBidder(c *gin.Context) {
	bidder := c.Param("bidder")
	if bidder == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bidder address is required"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 10
	}

	offset := (page - 1) * size

	dbCtx, cancel := queryWithTimeout(c, 5*time.Second)
	defer cancel()

	var total int64
	var bids []models.Bid
	query := database.DB.Model(&models.Bid{}).WithContext(dbCtx).Where("bidder = ?", bidder)

	query.Count(&total)
	query.Order("created_at DESC").Offset(offset).Limit(size).Find(&bids)

	var response []BidResponse
	for _, b := range bids {
		response = append(response, toBidResponse(b))
	}

	c.JSON(http.StatusOK, BidHistoryResponse{
		Total: total,
		Page:  page,
		Size:  size,
		Data:  response,
	})
}

func GetMarketStats(c *gin.Context) {
	dbCtx, cancel := queryWithTimeout(c, 10*time.Second)
	defer cancel()

	var stats struct {
		TotalAuctions  int64   `gorm:"column:total_auctions"`
		ActiveAuctions int64   `gorm:"column:active_auctions"`
		TotalBids      int64   `gorm:"column:total_bids"`
		HighestBid     float64 `gorm:"column:highest_bid"`
		HighestBidUSD  float64 `gorm:"column:highest_bid_usd"`
	}

	database.DB.WithContext(dbCtx).Raw(`
		SELECT
			COUNT(*) as total_auctions,
			COALESCE(SUM(CASE WHEN is_active = 1 THEN 1 ELSE 0 END), 0) as active_auctions,
			COALESCE((SELECT COUNT(*) FROM bids), 0) as total_bids,
			COALESCE((SELECT MAX(amount) FROM bids), 0) as highest_bid,
			COALESCE((SELECT MAX(amount_usd) FROM bids), 0) as highest_bid_usd
		FROM auctions
	`).Scan(&stats)

	c.JSON(http.StatusOK, gin.H{
		"total_auctions":  stats.TotalAuctions,
		"active_auctions": stats.ActiveAuctions,
		"total_bids":      stats.TotalBids,
		"highest_bid":     stats.HighestBid,
		"highest_bid_usd": stats.HighestBidUSD,
	})
}

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
