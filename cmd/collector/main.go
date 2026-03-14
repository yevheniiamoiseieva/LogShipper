package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"collector/internal/app"
	"collector/internal/config"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not found, using system environment variables")
	}

	configPath := flag.String("c", "", "path to config file")
	useTUI := flag.Bool("tui", false, "run with terminal UI")
	flag.Parse()

	if *configPath == "" {
		log.Fatal("config file is required (-c)")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	a := app.New(cfg)

	if *useTUI {
		err = a.RunWithTUI(ctx)
	} else {
		err = a.Run(ctx)
	}

	if err != nil {
		log.Fatal(err)
	}
}
