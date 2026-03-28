package bench

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

type GeneratorConfig struct {
	Services  []string
	MeanMS    float64
	StdDevMS  float64
	ErrorRate float64
}

var defaultServices = []string{
	"api-gw", "auth", "payment-service", "user-service", "db",
}

func DefaultConfig() GeneratorConfig {
	return GeneratorConfig{
		Services:  defaultServices,
		MeanMS:    50,
		StdDevMS:  10,
		ErrorRate: 0.02,
	}
}

type LogGenerator struct {
	cfg  GeneratorConfig
	rng  *rand.Rand
	seqN int64
}

func NewLogGenerator(seed int64, cfg GeneratorConfig) *LogGenerator {
	return &LogGenerator{
		cfg: cfg,
		rng: rand.New(rand.NewSource(seed)),
	}
}

func (g *LogGenerator) sampleLatency() float64 {
	v := g.rng.NormFloat64()*g.cfg.StdDevMS + g.cfg.MeanMS
	if v < 1 {
		v = 1
	}
	if v > 5000 {
		v = 5000
	}
	return v
}

func (g *LogGenerator) NextJSON() []byte {
	g.seqN++
	src := g.cfg.Services[g.rng.Intn(len(g.cfg.Services))]
	dst := g.cfg.Services[g.rng.Intn(len(g.cfg.Services))]
	traceID := fmt.Sprintf("t%016x", g.rng.Uint64())
	latencyMS := g.sampleLatency()
	isErr := g.rng.Float64() < g.cfg.ErrorRate
	status := 200
	level := "info"
	if isErr {
		status = 500
		level = "error"
	}

	rec := map[string]any{
		"timestamp":   time.Now().UTC().Format(time.RFC3339Nano),
		"level":       level,
		"service":     src,
		"dst_service": dst,
		"trace_id":    traceID,
		"span_id":     fmt.Sprintf("s%08x", g.rng.Uint32()),
		"latency_ms":  latencyMS,
		"status_code": status,
		"message":     fmt.Sprintf("handled request #%d", g.seqN),
		"operation":   "HTTP",
	}

	b, _ := json.Marshal(rec)
	return b
}

func (g *LogGenerator) NextECS() []byte {
	g.seqN++
	src := g.cfg.Services[g.rng.Intn(len(g.cfg.Services))]
	traceID := fmt.Sprintf("t%016x", g.rng.Uint64())
	latencyMS := g.sampleLatency()
	isErr := g.rng.Float64() < g.cfg.ErrorRate
	status := 200
	level := "info"
	if isErr {
		status = 500
		level = "error"
	}

	rec := map[string]any{
		"@timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"log": map[string]any{
			"level": level,
		},
		"service": map[string]any{
			"name": src,
		},
		"trace": map[string]any{
			"id": traceID,
		},
		"http": map[string]any{
			"response": map[string]any{
				"status_code": status,
			},
		},
		"event": map[string]any{
			"duration": int64(latencyMS * 1e6),
		},
		"message": fmt.Sprintf("ecs request #%d", g.seqN),
	}

	b, _ := json.Marshal(rec)
	return b
}

func (g *LogGenerator) NextPlain() []byte {
	g.seqN++
	src := g.cfg.Services[g.rng.Intn(len(g.cfg.Services))]
	return []byte(fmt.Sprintf("[%s] INFO %s: handled request #%d latency=%.2fms",
		time.Now().UTC().Format(time.RFC3339), src, g.seqN, g.sampleLatency()))
}
