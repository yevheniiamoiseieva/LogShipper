package app

import (
	"context"
	"fmt"
	"log"

	"collector/internal/config"
	"collector/internal/pipeline"
	"collector/internal/sinks"
	"collector/internal/sources"
	"collector/internal/transform"
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
	
	go func() {
		<-ctx.Done()
		log.Println("shutdown signal received")
	}()

	err = p.Run(ctx)

	log.Println("collector service stopped")
	return err
}

func (a *App) buildPipeline() (*pipeline.Pipeline, error) {
	if len(a.cfg.Sources) == 0 {
		return nil, fmt.Errorf("no sources defined in config")
	}
	if len(a.cfg.Sinks) == 0 {
		return nil, fmt.Errorf("no sinks defined in config")
	}
	if len(a.cfg.Transforms) > 1 {
		return nil, fmt.Errorf("only 1 transform is supported right now, got: %d", len(a.cfg.Transforms))
	}
	if len(a.cfg.Sinks) > 1 {
		return nil, fmt.Errorf("only 1 sink is supported right now, got: %d", len(a.cfg.Sinks))
	}

	sourcesByName := make(map[string]pipeline.Source, len(a.cfg.Sources))

	for name, sCfg := range a.cfg.Sources {
		log.Printf("initializing source: %s (type: %s)", name, sCfg.Type)

		var src pipeline.Source
		switch sCfg.Type {
		case "stdin":
			src = &sources.StdinSource{Service: sCfg.Service}
		case "file":
			src = &sources.FileSource{Service: sCfg.Service, Path: sCfg.Path}
		case "docker":
			src = &sources.DockerSource{Service: sCfg.Service, ContainerID: sCfg.ContainerID}
		default:
			return nil, fmt.Errorf("unknown source type: %s", sCfg.Type)
		}

		sourcesByName[name] = src
	}

	var (
		trans         pipeline.Transformer
		transformName string
		transformCfg  config.TransformConfig
		hasTransform  bool
	)

	for name, tCfg := range a.cfg.Transforms {
		transformName = name
		transformCfg = tCfg
		hasTransform = true
		break
	}

	var selectedSources []pipeline.Source
	if hasTransform {
		log.Printf("initializing transform: %s (type: %s)", transformName, transformCfg.Type)

		switch transformCfg.Type {
		case "remap-lite":
			trans = &transform.RemapTransform{
				AddFields: transformCfg.AddFields,
				Case:      transformCfg.Case,
			}
		default:
			return nil, fmt.Errorf("unknown transform type: %s", transformCfg.Type)
		}

		for _, inputName := range transformCfg.Inputs {
			src, ok := sourcesByName[inputName]
			if !ok {
				return nil, fmt.Errorf("transform [%s]: unknown source '%s'", transformName, inputName)
			}
			selectedSources = append(selectedSources, src)
		}
	} else {
		for _, src := range sourcesByName {
			selectedSources = append(selectedSources, src)
		}
	}

	if len(selectedSources) == 0 {
		return nil, fmt.Errorf("no sources selected for pipeline")
	}

	var (
		sink     pipeline.Sink
		sinkName string
		sinkCfg  config.SinkConfig
	)

	for name, sCfg := range a.cfg.Sinks {
		sinkName = name
		sinkCfg = sCfg
		break
	}

	log.Printf("initializing sink: %s (type: %s)", sinkName, sinkCfg.Type)

	if hasTransform {
		ok := false
		for _, in := range sinkCfg.Inputs {
			if in == transformName {
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("sink [%s] must include transform '%s' in inputs", sinkName, transformName)
		}
	} else {
		if len(sinkCfg.Inputs) == 0 {
			return nil, fmt.Errorf("sink [%s]: inputs list is empty", sinkName)
		}
	}

	switch sinkCfg.Type {
	case "stdout":
		sink = &sinks.StdoutSink{Pretty: sinkCfg.Pretty}
	default:
		return nil, fmt.Errorf("unknown sink type: %s", sinkCfg.Type)
	}

	return &pipeline.Pipeline{
		Sources:   selectedSources,
		Transform: trans,
		Sink:      sink,
	}, nil
}
