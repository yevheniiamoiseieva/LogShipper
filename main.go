package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type LogRecord struct {
	Timestamp time.Time `json:"ts"`
	Source    string    `json:"source"`
	Service   string    `json:"service"`
	Level     string    `json:"level,omitempty"`
	Line      string    `json:"line"`
}

func main() {
	source := flag.String("source", "stdin", "docker, k8s, stdin")
	service := flag.String("service", "", "my-api")
	endpoint := flag.String("endpoint", "", "HTTP call")
	logFormat := flag.String("format", "text", "text, json")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	scanner := bufio.NewScanner(os.Stdin)
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			log.Println("context canceled, exiting")
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var logLine string = line
		var timestamp time.Time = time.Now().UTC()
		var logLevel string = ""

		if *logFormat == "json" {
			var raw map[string]interface{}
			if err := json.Unmarshal([]byte(line), &raw); err == nil {

				if tsVal, ok := raw["time"].(string); ok {
					if parsedTime, err := time.Parse(time.RFC3339, tsVal); err == nil {
						timestamp = parsedTime.UTC()
						delete(raw, "time")
					}
				} else if tsVal, ok := raw["ts"].(string); ok {
					if parsedTime, err := time.Parse(time.RFC3339, tsVal); err == nil {
						timestamp = parsedTime.UTC()
						delete(raw, "ts")
					}
				}

				if levelVal, ok := raw["level"].(string); ok {
					logLevel = levelVal
					delete(raw, "level")
				}

				remainingBody, err := json.Marshal(raw)
				if err == nil {
					logLine = string(remainingBody)
				} else {
					logLine = line
				}

			} else {
				log.Printf("JSON parse error: %v, treating as raw text: %s", err, line)
				logLine = line
			}
		}

		rec := LogRecord{
			Timestamp: timestamp,
			Source:    *source,
			Service:   *service,
			Level:     logLevel,
			Line:      logLine,
		}

		if *endpoint == "" {
			fmt.Printf("%s [%s] (%s) %s\n",
				rec.Timestamp.Format(time.RFC3339),
				rec.Service,
				rec.Level,
				rec.Line,
			)
			continue
		}

		body, err := json.Marshal(rec)
		if err != nil {
			log.Printf("marshal error: %v", err)
			continue
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, *endpoint, bytes.NewReader(body))
		if err != nil {
			log.Printf("new request error: %v", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("send error: %v", err)
			continue
		}
		_ = resp.Body.Close()
	}

	if err := scanner.Err(); err != nil {
		log.Printf("scanner error: %v", err)
	}
}
