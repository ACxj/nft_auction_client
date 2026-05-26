package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auction-backend/config"
	"auction-backend/database"
	"auction-backend/handlers"
	"auction-backend/service"

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

	client, err := service.NewRetryClient(cfg.RPCURL)
	if err != nil {
		log.Fatalf("Failed to connect to Ethereum client: %v", err)
	}

	log.Println("Connected to Ethereum client with auto-reconnect enabled")

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

	log.Println("Event listener started successfully")

	r := gin.Default()

	r.GET("/health", handlers.HealthCheck)
	r.GET("/health/detailed", detailedHealthCheck(client))

	api := r.Group("/api/v1")
	{
		api.GET("/auctions", handlers.GetAuctions)
		api.GET("/auctions/:id", handlers.GetAuctionByID)
		api.GET("/auctions/:id/bids", handlers.GetBidHistory)
		api.GET("/bidders/:bidder/bids", handlers.GetBidsByBidder)
		api.GET("/stats", handlers.GetMarketStats)
	}

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on port %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Stopping event listener...")
	eventListener.Stop()

	log.Println("Closing Ethereum client...")
	client.Close()

	log.Println("Closing database connection...")
	sqlDB, err := database.DB.DB()
	if err == nil {
		sqlDB.Close()
	}

	log.Println("Server exited gracefully")
}

func detailedHealthCheck(client *service.RetryClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		status := "ok"
		httpStatus := http.StatusOK
		details := make(map[string]interface{})

		if sqlDB, err := database.DB.DB(); err == nil {
			if err := sqlDB.PingContext(ctx); err != nil {
				status = "degraded"
				details["database"] = "unhealthy: " + err.Error()
			} else {
				details["database"] = "healthy"
			}
		} else {
			status = "degraded"
			details["database"] = "unavailable"
		}

		if client.IsConnected() {
			if _, err := client.BlockNumber(ctx); err != nil {
				status = "degraded"
				details["ethereum"] = "unhealthy: " + err.Error()
			} else {
				details["ethereum"] = "healthy"
			}
		} else {
			status = "degraded"
			details["ethereum"] = "disconnected"
		}

		if status != "ok" {
			httpStatus = http.StatusServiceUnavailable
		}

		c.JSON(httpStatus, gin.H{
			"status":  status,
			"details": details,
		})
	}
}
