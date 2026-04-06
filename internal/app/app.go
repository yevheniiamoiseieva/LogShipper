package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"collector/internal/anomaly"
	"collector/internal/config"
	"collector/internal/event"
	"collector/internal/graph"
	"collector/internal/metrics"
	"collector/internal/pipeline"
	"collector/internal/resolve"
	"collector/internal/sinks"
	"collector/internal/sources"
	"collector/internal/transform"
	"collector/internal/tui"
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

func (a *App) RunWithTUI(ctx context.Context) error {
	g := graph.New(256)

	det := anomaly.NewZScoreDetector(100, 3.0, 256).
		WithMinSamples(20).
		WithCooldown(30 * time.Second)
	g.WithAnomalyDetector(det)

	ctx, cancel := context.WithCancel(ctx)
	g.Start(ctx)

	m := tui.New(g, det, cancel)
	prog := tea.NewProgram(m, tea.WithAltScreen())

	sink := &graphSink{graph: g, processed: func() { metrics.PipelineProcessed.Inc() }}

	p, err := a.buildPipelineWithSink(sink)
	if err != nil {
		cancel()
		return err
	}

	pipelineErr := make(chan error, 1)
	go func() {
		pipelineErr <- p.Run(ctx)
	}()

	if _, err := prog.Run(); err != nil {
		cancel()
		return err
	}

	cancel()
	return <-pipelineErr
}

func (a *App) RunMetrics(ctx context.Context, addr string) error {
	g := graph.New(1024)
	det := anomaly.NewZScoreDetector(100, 3.0, 1024).
		WithMinSamples(20).
		WithCooldown(30 * time.Second)
	g.WithAnomalyDetector(det)

	ctx, cancel := context.WithCancel(ctx)
	g.Start(ctx)

	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background()) //nolint:errcheck
	}()
	go func() {
		log.Printf("metrics server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server error: %v", err)
		}
	}()

	sink := &graphSink{graph: g, processed: func() { metrics.PipelineProcessed.Inc() }}
	p, err := a.buildPipelineWithSink(sink)
	if err != nil {
		cancel()
		return err
	}

	err = p.Run(ctx)
	cancel()
	return err
}

type graphSink struct {
	graph     *graph.CallGraph
	processed func()
}

func (s *graphSink) Run(ctx context.Context, in <-chan *event.NormalizedEvent) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-in:
			if !ok {
				return nil
			}
			if s.processed != nil {
				s.processed()
			}
			if ev.SrcService != "" && ev.DstService != "" {
				s.graph.Feed(&graph.NormalizedEvent{
					SrcService: ev.SrcService,
					DstService: ev.DstService,
					Operation:  ev.Operation,
					IsError:    ev.StatusCode >= 500,
					Latency:    ev.Latency,
					OccurredAt: ev.Timestamp,
				})
			}
		}
	}
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

	selectedSources, trans, transformName, hasTransform, err := a.buildSourcesAndTransform()
	if err != nil {
		return nil, err
	}

	var sinkName string
	var sinkCfg config.SinkConfig
	for name, sCfg := range a.cfg.Sinks {
		sinkName = name
		sinkCfg = sCfg
		break
	}

	log.Printf("initializing sink: %s (type: %s)", sinkName, sinkCfg.Type)

	if err := a.validateSinkInputs(sinkName, sinkCfg, transformName, hasTransform); err != nil {
		return nil, err
	}

	var sink pipeline.Sink
	switch sinkCfg.Type {
	case "stdout":
		sink = &sinks.StdoutSink{Pretty: sinkCfg.Pretty}
	default:
		return nil, fmt.Errorf("unknown sink type: %s", sinkCfg.Type)
	}

	resolver, err := resolve.FromConfig(a.cfg.Resolve)
	if err != nil {
		return nil, err
	}

	return &pipeline.Pipeline{
		Sources:   selectedSources,
		Transform: trans,
		Sink:      sink,
		Resolver:  resolver,
	}, nil
}

func (a *App) buildPipelineWithSink(normSink pipeline.NormalizedSink) (*pipeline.Pipeline, error) {
	if len(a.cfg.Sources) == 0 {
		return nil, fmt.Errorf("no sources defined in config")
	}
	if len(a.cfg.Transforms) > 1 {
		return nil, fmt.Errorf("only 1 transform is supported right now, got: %d", len(a.cfg.Transforms))
	}

	selectedSources, trans, _, _, err := a.buildSourcesAndTransform()
	if err != nil {
		return nil, err
	}

	resolver, err := resolve.FromConfig(a.cfg.Resolve)
	if err != nil {
		return nil, err
	}

	return &pipeline.Pipeline{
		Sources:        selectedSources,
		Transform:      trans,
		NormalizedSink: normSink,
		Resolver:       resolver,
	}, nil
}

func (a *App) buildSourcesAndTransform() (
	selectedSources []pipeline.Source,
	trans pipeline.Transformer,
	transformName string,
	hasTransform bool,
	err error,
) {
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
			err = fmt.Errorf("unknown source type: %s", sCfg.Type)
			return
		}
		sourcesByName[name] = src
	}

	var transformCfg config.TransformConfig
	for name, tCfg := range a.cfg.Transforms {
		transformName = name
		transformCfg = tCfg
		hasTransform = true
		break
	}

	if hasTransform {
		log.Printf("initializing transform: %s (type: %s)", transformName, transformCfg.Type)
		switch transformCfg.Type {
		case "remap-lite":
			trans = &transform.RemapTransform{
				AddFields: transformCfg.AddFields,
				Case:      transformCfg.Case,
			}
		default:
			err = fmt.Errorf("unknown transform type: %s", transformCfg.Type)
			return
		}
		for _, inputName := range transformCfg.Inputs {
			src, ok := sourcesByName[inputName]
			if !ok {
				err = fmt.Errorf("transform [%s]: unknown source '%s'", transformName, inputName)
				return
			}
			selectedSources = append(selectedSources, src)
		}
	} else {
		for _, src := range sourcesByName {
			selectedSources = append(selectedSources, src)
		}
	}

	if len(selectedSources) == 0 {
		err = fmt.Errorf("no sources selected for pipeline")
	}
	return
}

func (a *App) validateSinkInputs(sinkName string, sinkCfg config.SinkConfig, transformName string, hasTransform bool) error {
	if hasTransform {
		for _, in := range sinkCfg.Inputs {
			if in == transformName {
				return nil
			}
		}
		return fmt.Errorf("sink [%s] must include transform '%s' in inputs", sinkName, transformName)
	}
	if len(sinkCfg.Inputs) == 0 {
		return fmt.Errorf("sink [%s]: inputs list is empty", sinkName)
	}
	return nil
}
