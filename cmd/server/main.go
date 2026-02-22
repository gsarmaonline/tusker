package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gsarma/tusker/internal/api"
	"github.com/gsarma/tusker/internal/crypto"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	rootKey := os.Getenv("ROOT_ENCRYPTION_KEY")
	if rootKey == "" {
		log.Fatal("ROOT_ENCRYPTION_KEY is required")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	enc, err := crypto.NewEncryptor(rootKey)
	if err != nil {
		log.Fatalf("failed to initialize encryptor: %v", err)
	}

	router := gin.Default()
	api.RegisterRoutes(router, pool, enc)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
