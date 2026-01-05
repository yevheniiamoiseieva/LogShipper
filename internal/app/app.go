package app

import (
	"context"
	"fmt"
	"log"
	"sync"

	"collector/internal/config"
	"collector/internal/pipeline"
	"collector/internal/sinks"
	"collector/internal/sources"
)

type App struct {
	cfg *config.Config
}

func New(cfg *config.Config) *App {
	return &App{cfg: cfg}
}

func (a *App) Run(ctx context.Context) error {
	log.Println("collector service starting")

	p, err := a.buildPipeline()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		if err := p.Run(ctx); err != nil {
			log.Printf("pipeline error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutdown signal received")

	wg.Wait()
	log.Println("collector service stopped")

	return nil
}

func (a *App) buildPipeline() (*pipeline.Pipeline, error) {

	var src pipeline.Source

	switch a.cfg.Source.Type {
	case "stdin":
		src = &sources.StdinSource{
			Service: a.cfg.Source.Service,
		}
	default:
		return nil, fmt.Errorf("unknown source type: %s", a.cfg.Source.Type)
	}

	var sink pipeline.Sink

	switch a.cfg.Sink.Type {
	case "stdout":
		sink = &sinks.StdoutSink{
			Pretty: a.cfg.Sink.Pretty,
		}
	default:
		return nil, fmt.Errorf("unknown sink type: %s", a.cfg.Sink.Type)
	}

	return &pipeline.Pipeline{
		Source: src,
		Sink:   sink,
	}, nil
}
