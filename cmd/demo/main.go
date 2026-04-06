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
	{"api-gw", "billing", "GET /billing/history", 40, 12, 0.02, 5},
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
	{"billing", "fraud-check", "POST /risk-check", 22, 7, 0.02, 4},
	{"api-gw", "fraud-check", "POST /pre-check", 18, 6, 0.01, 3},
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
	paymentDBSpike atomic.Int32
	billingCascade atomic.Int32
	authErrSpike   atomic.Int32
	inventorySpike atomic.Int32
	cycleActive    atomic.Int32
	mailerSpike    atomic.Int32
	searchOverload atomic.Int32
)

func cascadeIncidentLoop() {
	for {
		time.Sleep(30 * time.Second)
		paymentDBSpike.Store(1)
		time.Sleep(12 * time.Second)
		billingCascade.Store(1)
		time.Sleep(18 * time.Second)
		paymentDBSpike.Store(0)
		time.Sleep(8 * time.Second)
		billingCascade.Store(0)
	}
}

func authIncidentLoop() {
	time.Sleep(15 * time.Second)
	for {
		time.Sleep(55 * time.Second)
		authErrSpike.Store(1)
		time.Sleep(20 * time.Second)
		authErrSpike.Store(0)
	}
}

func inventoryIncidentLoop() {
	time.Sleep(20 * time.Second)
	for {
		time.Sleep(65 * time.Second)
		inventorySpike.Store(1)
		time.Sleep(25 * time.Second)
		inventorySpike.Store(0)
	}
}

func cycleIncidentLoop() {
	time.Sleep(35 * time.Second)
	for {
		time.Sleep(75 * time.Second)
		cycleActive.Store(1)
		time.Sleep(15 * time.Second)
		cycleActive.Store(0)
	}
}

func mailerIncidentLoop() {
	time.Sleep(25 * time.Second)
	for {
		time.Sleep(60 * time.Second)
		mailerSpike.Store(1)
		time.Sleep(20 * time.Second)
		mailerSpike.Store(0)
	}
}

func searchIncidentLoop() {
	time.Sleep(40 * time.Second)
	for {
		time.Sleep(80 * time.Second)
		searchOverload.Store(1)
		time.Sleep(30 * time.Second)
		searchOverload.Store(0)
	}
}

type jsonRecord struct {
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

type ecsRecord struct {
	Timestamp string         `json:"@timestamp"`
	Log       ecsLog         `json:"log"`
	Service   ecsService     `json:"service"`
	Event     ecsEvent       `json:"event"`
	HTTP      ecsHTTP        `json:"http"`
	Trace     ecsTrace       `json:"trace"`
	Labels    map[string]any `json:"labels,omitempty"`
}

type ecsLog struct{ Level string `json:"level"` }
type ecsService struct{ Name string `json:"name"` }
type ecsEvent struct {
	Action   string `json:"action"`
	Duration int64  `json:"duration"`
	Outcome  string `json:"outcome"`
}
type ecsHTTP struct {
	Response ecsHTTPResp `json:"response"`
}
type ecsHTTPResp struct{ StatusCode int `json:"status_code"` }
type ecsTrace struct {
	ID   string  `json:"id"`
	Span ecsSpan `json:"span"`
}
type ecsSpan struct{ ID string `json:"id"` }

type metricRecord struct {
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Service   string  `json:"service"`
	Timestamp string  `json:"timestamp"`
}

var mu sync.Mutex
var writer *bufio.Writer

func worker(id int) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)*1337))
	n := len(weightedTopology)

	type record struct {
		b   []byte
		err error
	}
	buf := make([]record, 0, 200)

	for {
		buf = buf[:0]

		for i := 0; i < 200; i++ {
			e := weightedTopology[rng.Intn(n)]

			latency := rng.NormFloat64()*e.stdMS + e.baseMS
			errRate := e.errRate

			if paymentDBSpike.Load() == 1 && e.src == "payment" && e.dst == "db" {
				latency = rng.NormFloat64()*80 + 800
				errRate = 0.35
			}
			if billingCascade.Load() == 1 && e.src == "billing" && e.dst == "payment" {
				latency = rng.NormFloat64()*60 + 600
				errRate = 0.28
			}
			if authErrSpike.Load() == 1 && e.src == "auth" {
				errRate = 0.60
				latency *= 3.5
			}
			if inventorySpike.Load() == 1 && e.src == "inventory" {
				latency = rng.NormFloat64()*40 + 500
			}
			if mailerSpike.Load() == 1 && e.src == "mailer" {
				latency = rng.NormFloat64()*150 + 3000
				errRate = 0.45
			}
			if searchOverload.Load() == 1 && e.src == "search" && e.dst == "db" {
				latency = rng.NormFloat64()*60 + 700
				errRate = 0.30
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
			ts := time.Now().UTC()

			fmtRoll := rng.Intn(100)
			var b []byte
			var err error

			switch {
			case fmtRoll < 55:
				b, err = json.Marshal(jsonRecord{
					Timestamp:  ts.Format(time.RFC3339Nano),
					Level:      level,
					Service:    e.src,
					DstService: e.dst,
					TraceID:    traceID,
					SpanID:     spanID,
					Operation:  e.op,
					LatencyMs:  latency,
					StatusCode: status,
					Message:    fmt.Sprintf("%s → %s: %s", e.src, e.dst, e.op),
				})

			case fmtRoll < 80:
				outcome := "success"
				if isErr {
					outcome = "failure"
				}
				b, err = json.Marshal(ecsRecord{
					Timestamp: ts.Format(time.RFC3339Nano),
					Log:       ecsLog{Level: level},
					Service:   ecsService{Name: e.src},
					Event: ecsEvent{
						Action:   e.op,
						Duration: int64(latency * 1e6),
						Outcome:  outcome,
					},
					HTTP:  ecsHTTP{Response: ecsHTTPResp{StatusCode: status}},
					Trace: ecsTrace{ID: traceID, Span: ecsSpan{ID: spanID}},
					Labels: map[string]any{
						"dst_service": e.dst,
					},
				})

			case fmtRoll < 90:
				msg := fmt.Sprintf("[%s] %s %s → %s latency=%.1fms status=%d",
					level, ts.Format("15:04:05.000"), e.src, e.dst, latency, status)
				b = []byte(msg)

			default:
				b, err = json.Marshal(metricRecord{
					Metric:    "rpc_duration_ms",
					Value:     latency,
					Service:   e.src,
					Timestamp: ts.Format(time.RFC3339Nano),
				})
			}

			if err == nil {
				buf = append(buf, record{b: b})
			}
		}

		if cycleActive.Load() == 1 {
			traceID := fmt.Sprintf("%016x%016x", rng.Uint64(), rng.Uint64())
			ts := time.Now().UTC()
			for _, pair := range [][2]string{
				{"api-gw", "notification"},
				{"notification", "api-gw"},
			} {
				b, _ := json.Marshal(jsonRecord{
					Timestamp:  ts.Format(time.RFC3339Nano),
					Level:      "warn",
					Service:    pair[0],
					DstService: pair[1],
					TraceID:    traceID,
					SpanID:     fmt.Sprintf("%016x", rng.Uint64()),
					Operation:  "GET /notify/callback",
					LatencyMs:  rng.NormFloat64()*5 + 20,
					StatusCode: 200,
					Message:    fmt.Sprintf("cycle: %s ↔ %s", pair[0], pair[1]),
				})
				buf = append(buf, record{b: b})
			}
		}

		mu.Lock()
		for _, rec := range buf {
			writer.Write(rec.b)
			writer.WriteByte('\n')
		}
		mu.Unlock()

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

	go cascadeIncidentLoop()
	go authIncidentLoop()
	go inventoryIncidentLoop()
	go cycleIncidentLoop()
	go mailerIncidentLoop()
	go searchIncidentLoop()
	go flushLoop()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			worker(id)
		}(i)
	}
	wg.Wait()
}
