package main

import (
	"log"

	"auction-backend/config"
	"auction-backend/database"
	"auction-backend/handlers"
	"auction-backend/service"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()

	if err := database.CreateDatabaseIfNotExists(cfg); err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}

	if err := database.InitDB(cfg); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := database.AutoMigrate(); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		log.Fatalf("Failed to connect to Ethereum client: %v", err)
	}
	defer client.Close()

	log.Println("Connected to Ethereum client")

	eventListener, err := service.NewEventListener(cfg, client)
	if err != nil {
		log.Fatalf("Failed to create event listener: %v", err)
	}

	if err := service.SyncAuctionsFromChain(eventListener); err != nil {
		log.Printf("Warning: Failed to sync historical events: %v", err)
	}

	if err := eventListener.Start(); err != nil {
		log.Fatalf("Failed to start event listener: %v", err)
	}

	defer eventListener.Stop()

	log.Println("Event listener started successfully")

	r := gin.Default()

	r.GET("/health", handlers.HealthCheck)

	api := r.Group("/api/v1")
	{
		api.GET("/auctions", handlers.GetAuctions)
		api.GET("/auctions/:id", handlers.GetAuctionByID)
		api.GET("/auctions/:id/bids", handlers.GetBidHistory)
		api.GET("/bidders/:bidder/bids", handlers.GetBidsByBidder)
		api.GET("/stats", handlers.GetMarketStats)
	}

	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}