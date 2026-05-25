package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	RPCURL         string
	ContractAddr   string
	NFTAddr        string
	OracleAddr     string
	ProxyAdminAddr string
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	ServerPort     string
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	return &Config{
		RPCURL:         getEnv("RPC_URL", "https://sepolia.infura.io/v3/your-infura-key"),
		ContractAddr:   getEnv("AUCTION_CONTRACT", "0x652526a13f8f808cb6db604020a788c693ad719f"),
		NFTAddr:        getEnv("NFT_CONTRACT", "0x311aae3d9327c77b6d8f4a78dc9f564359808ac0"),
		OracleAddr:     getEnv("ORACLE_CONTRACT", "0xf301b2639238ae39b27291de67bdfebacb3ffe86"),
		ProxyAdminAddr: getEnv("PROXY_ADMIN", "0xe532f0471636704f0ce4ca93405bfdd0a0dc5592"),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "3306"),
		DBUser:         getEnv("DB_USER", "root"),
		DBPassword:     getEnv("DB_PASSWORD", "password"),
		DBName:         getEnv("DB_NAME", "auction_db"),
		ServerPort:     getEnv("SERVER_PORT", "8080"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}