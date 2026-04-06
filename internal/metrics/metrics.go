package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	EventsReceived = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "logshipper_events_received_total",
		Help: "Total events received by source type",
	}, []string{"source"})

	ParseTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "logshipper_parse_total",
		Help: "Total parse operations by format",
	}, []string{"format"})

	ParseErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "logshipper_parse_errors_total",
		Help: "Total parse failures",
	})

	GraphNodes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "logshipper_graph_nodes",
		Help: "Current number of nodes in the service graph",
	})

	GraphEdges = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "logshipper_graph_edges",
		Help: "Current number of edges in the service graph",
	})

	GraphNewEdges = promauto.NewCounter(prometheus.CounterOpts{
		Name: "logshipper_graph_new_edges_total",
		Help: "Total new edges discovered",
	})

	GraphCycles = promauto.NewCounter(prometheus.CounterOpts{
		Name: "logshipper_graph_cycles_total",
		Help: "Total cyclic dependencies detected",
	})

	AnomaliesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "logshipper_anomalies_total",
		Help: "Total anomalies detected by metric type",
	}, []string{"metric"})

	PipelineProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "logshipper_pipeline_processed_total",
		Help: "Total events processed through the full pipeline",
	})
)

func Handler() http.Handler {
	return promhttp.Handler()
}
