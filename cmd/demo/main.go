package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type edge struct {
	src     string
	dst     string
	op      string
	baseMS  float64
	stdMS   float64
	errRate float64
	weight  int
}

var topology = []edge{
	{"api-gw", "auth", "POST /auth/verify", 5, 2, 0.01, 10},
	{"api-gw", "user-service", "GET /users", 15, 5, 0.02, 8},
	{"api-gw", "payment", "POST /pay", 50, 15, 0.02, 6},
	{"api-gw", "inventory", "GET /products", 20, 8, 0.01, 9},
	{"api-gw", "search", "GET /search", 30, 10, 0.01, 7},
	{"payment", "db", "INSERT transactions", 30, 10, 0.01, 10},
	{"payment", "redis", "GET cache", 2, 1, 0.005, 10},
	{"payment", "notification", "POST /notify", 10, 3, 0.02, 8},
	{"payment", "fraud-check", "POST /verify", 25, 8, 0.03, 7},
	{"user-service", "db", "SELECT users", 25, 8, 0.01, 10},
	{"user-service", "cache", "GET session", 3, 1, 0.005, 10},
	{"user-service", "notification", "POST /welcome", 12, 4, 0.02, 4},
	{"inventory", "db", "SELECT products", 20, 6, 0.01, 10},
	{"inventory", "cache", "GET products", 4, 1, 0.005, 10},
	{"inventory", "search", "POST /index", 15, 5, 0.01, 6},
	{"notification", "user-service", "GET /user/email", 12, 4, 0.02, 8},
	{"notification", "mailer", "POST /send", 80, 30, 0.05, 6},
	{"billing", "payment", "POST /billing", 45, 12, 0.015, 5},
	{"billing", "db", "INSERT invoices", 28, 8, 0.01, 5},
	{"billing", "notification", "POST /invoice", 10, 3, 0.02, 4},
	{"search", "db", "SELECT search_idx", 35, 12, 0.01, 8},
	{"search", "cache", "GET results", 5, 2, 0.005, 9},
	{"fraud-check", "db", "SELECT risk_rules", 20, 6, 0.01, 7},
	{"fraud-check", "redis", "GET blacklist", 3, 1, 0.005, 8},
	{"auth", "db", "SELECT credentials", 15, 5, 0.01, 10},
	{"auth", "redis", "GET token", 2, 1, 0.003, 10},
	{"mailer", "notification", "POST /delivery", 50, 20, 0.08, 4},
}

var weightedTopology []edge

func buildWeightedTopology() {
	for _, e := range topology {
		for i := 0; i < e.weight; i++ {
			weightedTopology = append(weightedTopology, e)
		}
	}
}

var (
	paymentDBSpike  atomic.Int32
	authErrSpike    atomic.Int32
	inventorySpike  atomic.Int32
	cycleActive     atomic.Int32
	mailerSpike     atomic.Int32
)

func incidentLoop() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	_ = rng

	for {
		time.Sleep(90 * time.Second)
		paymentDBSpike.Store(1)
		time.Sleep(30 * time.Second)
		paymentDBSpike.Store(0)
	}
}

func authIncidentLoop() {
	for {
		time.Sleep(120 * time.Second)
		authErrSpike.Store(1)
		time.Sleep(20 * time.Second)
		authErrSpike.Store(0)
	}
}

func inventoryIncidentLoop() {
	for {
		time.Sleep(150 * time.Second)
		inventorySpike.Store(1)
		time.Sleep(40 * time.Second)
		inventorySpike.Store(0)
	}
}

func cycleIncidentLoop() {
	for {
		time.Sleep(180 * time.Second)
		cycleActive.Store(1)
		time.Sleep(15 * time.Second)
		cycleActive.Store(0)
	}
}

func mailerIncidentLoop() {
	for {
		time.Sleep(200 * time.Second)
		mailerSpike.Store(1)
		time.Sleep(25 * time.Second)
		mailerSpike.Store(0)
	}
}

type logRecord struct {
	Timestamp  string  `json:"timestamp"`
	Level      string  `json:"level"`
	Service    string  `json:"service"`
	DstService string  `json:"dst_service"`
	TraceID    string  `json:"trace_id"`
	SpanID     string  `json:"span_id"`
	Operation  string  `json:"operation"`
	LatencyMs  float64 `json:"latency_ms"`
	StatusCode int     `json:"status_code"`
	Message    string  `json:"message"`
}

var mu sync.Mutex
var writer *bufio.Writer

func worker(id int) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)*1000))
	n := len(weightedTopology)

	buf := make([]logRecord, 0, 200)

	for {
		for i := 0; i < 200; i++ {
			e := weightedTopology[rng.Intn(n)]

			latency := rng.NormFloat64()*e.stdMS + e.baseMS
			errRate := e.errRate

			if paymentDBSpike.Load() == 1 && e.src == "payment" && e.dst == "db" {
				latency = rng.NormFloat64()*50 + 500
				errRate = 0.25
			}
			if authErrSpike.Load() == 1 && e.src == "auth" {
				errRate = 0.55
				latency *= 3
			}
			if inventorySpike.Load() == 1 && e.src == "inventory" {
				latency = rng.NormFloat64()*30 + 400
			}
			if mailerSpike.Load() == 1 && e.src == "mailer" {
				latency = rng.NormFloat64()*100 + 2000
				errRate = 0.4
			}
			if latency < 0.5 {
				latency = 0.5
			}

			isErr := rng.Float64() < errRate
			status := 200
			level := "info"
			if isErr {
				status = 500
				level = "error"
			} else if rng.Float64() < 0.05 {
				status = 400
				level = "warn"
			}

			traceID := fmt.Sprintf("%016x%016x", rng.Uint64(), rng.Uint64())
			spanID := fmt.Sprintf("%016x", rng.Uint64())

			buf = append(buf, logRecord{
				Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
				Level:      level,
				Service:    e.src,
				DstService: e.dst,
				TraceID:    traceID,
				SpanID:     spanID,
				Operation:  e.op,
				LatencyMs:  latency,
				StatusCode: status,
				Message:    fmt.Sprintf("%s called %s via %s", e.src, e.dst, e.op),
			})
		}

		if cycleActive.Load() == 1 {
			for k := 0; k < 5; k++ {
				traceID := fmt.Sprintf("%016x%016x", rng.Uint64(), rng.Uint64())
				buf = append(buf, logRecord{
					Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
					Level:      "warn",
					Service:    "api-gw",
					DstService: "notification",
					TraceID:    traceID,
					SpanID:     fmt.Sprintf("%016x", rng.Uint64()),
					Operation:  "GET /notify/callback",
					LatencyMs:  rng.NormFloat64()*5 + 20,
					StatusCode: 200,
					Message:    "cycle: api-gw -> notification -> api-gw",
				})
				buf = append(buf, logRecord{
					Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
					Level:      "warn",
					Service:    "notification",
					DstService: "api-gw",
					TraceID:    traceID,
					SpanID:     fmt.Sprintf("%016x", rng.Uint64()),
					Operation:  "POST /callback",
					LatencyMs:  rng.NormFloat64()*5 + 15,
					StatusCode: 200,
					Message:    "cycle: notification -> api-gw",
				})
			}
		}

		mu.Lock()
		for _, rec := range buf {
			b, _ := json.Marshal(rec)
			writer.Write(b)
			writer.WriteByte('\n')
		}
		mu.Unlock()

		buf = buf[:0]

		time.Sleep(5 * time.Millisecond)
	}
}

func flushLoop() {
	for {
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		writer.Flush()
		mu.Unlock()
	}
}

func main() {
	buildWeightedTopology()

	writer = bufio.NewWriterSize(os.Stdout, 4*1024*1024)

	go incidentLoop()
	go authIncidentLoop()
	go inventoryIncidentLoop()
	go cycleIncidentLoop()
	go mailerIncidentLoop()
	go flushLoop()

	workers := 16
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			worker(id)
		}(i)
	}
	wg.Wait()
}
