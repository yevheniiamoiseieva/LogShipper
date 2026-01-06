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
	var allSources []pipeline.Source
	var sink pipeline.Sink
	var trans pipeline.Transformer
	for name, sCfg := range a.cfg.Sources {
		log.Printf("initializing source: %s (type: %s)", name, sCfg.Type)
		switch sCfg.Type {
		case "stdin":
			allSources = append(allSources, &sources.StdinSource{
				Service: sCfg.Service,
			})
		case "file":
			allSources = append(allSources, &sources.FileSource{
				Service: sCfg.Service,
				Path:    sCfg.Path,
			})
		case "docker":
			allSources = append(allSources, &sources.DockerSource{
				Service:     sCfg.Service,
				ContainerID: sCfg.ContainerID,
			})
		default:
			return nil, fmt.Errorf("unknown source type: %s", sCfg.Type)
		}
	}

	for name, tCfg := range a.cfg.Transforms {
		log.Printf("initializing transform: %s", name)
		switch tCfg.Type {
		case "remap-lite":
			trans = &pipeline.RemapTransform{
				AddFields: tCfg.AddFields,
			}
		default:
			return nil, fmt.Errorf("unknown transform type: %s", tCfg.Type)
		}
		break
	}

	for name, snkCfg := range a.cfg.Sinks {
		log.Printf("initializing sink: %s", name)
		switch snkCfg.Type {
		case "stdout":
			sink = &sinks.StdoutSink{
				Pretty: snkCfg.Pretty,
			}
		default:
			return nil, fmt.Errorf("unknown sink type: %s", snkCfg.Type)
		}
		break
	}

	if len(allSources) == 0 {
		return nil, fmt.Errorf("no sources defined in config")
	}
	if sink == nil {
		return nil, fmt.Errorf("no sinks defined in config")
	}

	return &pipeline.Pipeline{
		Sources:   allSources,
		Transform: trans,
		Sink:      sink,
	}, nil
}