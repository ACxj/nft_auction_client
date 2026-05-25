package models

import (
	"time"

	"gorm.io/gorm"
)

type Auction struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	AuctionID     uint64    `gorm:"uniqueIndex;not null" json:"auction_id"`
	NFTAddress    string    `gorm:"type:varchar(42);index;not null" json:"nft_address"`
	TokenID       uint64    `gorm:"not null" json:"token_id"`
	Seller        string    `gorm:"type:varchar(42);index;not null" json:"seller"`
	StartTime     time.Time `gorm:"not null" json:"start_time"`
	EndTime       time.Time `gorm:"not null" json:"end_time"`
	StartingPrice float64   `gorm:"not null" json:"starting_price"`
	HighestBidder string    `gorm:"type:varchar(42)" json:"highest_bidder"`
	HighestBid    float64   `json:"highest_bid"`
	HighestBidUSD float64   `json:"highest_bid_usd"`
	IsActive      bool      `gorm:"index;default:true" json:"is_active"`
	Winner        string    `gorm:"type:varchar(42)" json:"winner"`
	FinalPrice    float64   `json:"final_price"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Auction) TableName() string {
	return "auctions"
}

type Bid struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AuctionID   uint64    `gorm:"index;not null" json:"auction_id"`
	Bidder      string    `gorm:"type:varchar(42);index;not null" json:"bidder"`
	Amount      float64   `gorm:"not null" json:"amount"`
	AmountUSD   float64   `gorm:"not null" json:"amount_usd"`
	IsETH       bool      `gorm:"not null" json:"is_eth"`
	BlockNumber uint64    `gorm:"not null" json:"block_number"`
	TxHash      string    `gorm:"type:varchar(66);uniqueIndex;not null" json:"tx_hash"`
	CreatedAt   time.Time `gorm:"not null" json:"created_at"`
}

func (Bid) TableName() string {
	return "bids"
}

type PriceUpdate struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Asset     string    `gorm:"type:varchar(20);not null" json:"asset"`
	PriceUSD  float64   `gorm:"not null" json:"price_usd"`
	Timestamp time.Time `gorm:"not null" json:"timestamp"`
}

func (PriceUpdate) TableName() string {
	return "price_updates"
}
