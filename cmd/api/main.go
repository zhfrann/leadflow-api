package main

import (
	"log"

	"github.com/zhfrann/leadflow-api/internal/platform/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	log.Printf("LeadFlow API Starting on %s in %s mode", cfg.HTTPAddress, cfg.Environment)
}
