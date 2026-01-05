package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"collector/internal/app"
	"collector/internal/config"
)

func main() {
	configPath := flag.String("c", "", "path to config file")
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
	if err := a.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
