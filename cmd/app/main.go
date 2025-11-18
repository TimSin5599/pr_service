package main

import (
	"log"

	"github.com/evrone/go-clean-template/config"
	"github.com/evrone/go-clean-template/internal/app"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: No .env file found: %v", err)
	}

	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}

	app.Run(cfg)
}
