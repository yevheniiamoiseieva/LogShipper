package event

import (
	"errors"
	"net/http"
)

func (e *NormalizedEvent) Validate() error {
	if e.Timestamp.IsZero() {
		return errors.New("event: Timestamp is required")
	}
	if e.SrcService == "" {
		return errors.New("event: SrcService is required")
	}
	return nil
}

// IsMetric returns true if the event has Latency or an HTTP StatusCode.
func (e *NormalizedEvent) IsMetric() bool {
	return e.Latency > 0 || e.StatusCode >= http.StatusContinue
}

// HasCorrelationKey returns true if the event has a TraceID or SrcService+DstService pair.
func (e *NormalizedEvent) HasCorrelationKey() bool {
	return e.TraceID != "" || (e.SrcService != "" && e.DstService != "")
}