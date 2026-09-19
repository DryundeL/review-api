package main

import (
	"context"
	"flag"
	"log"
	"os"

	"review-api/internal/platform/config"
	"review-api/internal/platform/database"
)

func main() {
	command := flag.String("command", "up", "up | down")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.Database.DSN())
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	switch *command {
	case "up":
		if err := database.MigrateUp(ctx, pool); err != nil {
			log.Fatal(err)
		}
		log.Println("migrations applied")
	case "down":
		if err := database.MigrateDown(ctx, pool); err != nil {
			log.Fatal(err)
		}
		log.Println("last migration rolled back")
	default:
		log.Fatalf("unknown command %q", *command)
		os.Exit(2)
	}
}
