package app

import (
	"context"
	"log"

	"collector/internal/config"
)

type App struct {
	cfg *config.Config
}

func New(cfg *config.Config) *App {
	return &App{cfg: cfg}
}

func (a *App) Run(ctx context.Context) error {
	log.Println("collector service starting")

	<-ctx.Done()

	log.Println("collector service stopped")
	return nil
}
