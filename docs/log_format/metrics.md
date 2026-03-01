# Metric Formats

Lines that match a metric format are parsed into an `Event` with `Type = "metric"`.
The log-specific fields (`Level`, `Message`) are left empty.
Parser: `parse.MetricParser`.

---

## Supported Formats

| Format | Example |
|--------|---------|
| [Simple key=value](#simple-keyvalue) | `cpu_usage=73.2 host=worker-1` |
| [Prometheus exposition](#prometheus-exposition-format) | `http_requests_total{method="GET"} 1027 1710506096789` |

---

## Simple key=value

### Grammar

```
metric_name=<number> [<tag_key>=<tag_value> ...]
```

- The **first** token must be `name=number`. This anchors detection.
- Additional tokens are treated as **tags** (string key-value pairs).
- Values can be integers or floating-point numbers (including scientific notation).
- Tag values are unquoted strings; whitespace is the delimiter.

### Examples

```
cpu_usage=73.2
memory_bytes=2147483648 host=worker-1
http_requests_total=1523 service=api method=GET status=200
queue_depth=42 queue=payments region=eu-west-1
latency_p99_ms=312.5 service=checkout env=prod
```

### Field Mapping

| Token | `Event` field |
|-------|--------------|
| Key part of first token | `MetricName` |
| Value part of first token | `MetricValue` (parsed as `float64`) |
| All remaining `k=v` tokens | `MetricTags` |
| `time.Now()` | `Timestamp` (no timestamp in this format) |
| `"metric"` | `Type` |

---

## Prometheus Exposition Format

The Prometheus text-based exposition format is the de facto standard for
metrics in cloud-native systems.
Full specification: https://prometheus.io/docs/instrumenting/exposition_formats/

### Line Types

```
# HELP <metric_name> <description>
# TYPE <metric_name> <type>
<metric_name>[{<labels>}] <value> [<timestamp_ms>]
```

### Example Block

```
# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter

http_requests_total{method="GET",handler="/api/users",status="200"}    1027 1710506096789
http_requests_total{method="POST",handler="/api/payments",status="500"}  14 1710506096789

# HELP process_cpu_seconds_total Total user and system CPU time spent in seconds
# TYPE process_cpu_seconds_total counter
process_cpu_seconds_total 4.609768

# HELP node_memory_MemAvailable_bytes Available memory in bytes
# TYPE node_memory_MemAvailable_bytes gauge
node_memory_MemAvailable_bytes 1.3476864e+09
```

### Metric Types

| Type | Description | Typical use |
|------|-------------|------------|
| `counter` | Monotonically increasing value; resets to 0 on process restart | Request counts, error counts |
| `gauge` | Arbitrary value that can go up or down | Memory usage, queue depth, temperature |
| `histogram` | Distribution of observations in configurable buckets | Request latency, payload sizes |
| `summary` | Sliding-window quantiles (`_sum`, `_count`, quantile labels) | Request latency p50/p95/p99 |

### Histogram and Summary Conventions

Histograms emit three synthetic series:

```
http_request_duration_seconds_bucket{le="0.1"} 24054
http_request_duration_seconds_bucket{le="0.25"} 33444
http_request_duration_seconds_bucket{le="+Inf"} 144320
http_request_duration_seconds_sum   53423
http_request_duration_seconds_count 144320
```

Each `_bucket`, `_sum`, and `_count` line is parsed as a **separate `Event`**
with `MetricName` set to the full series name including suffix.

### Field Mapping

| Prometheus element | `Event` field | Notes |
|-------------------|--------------|-------|
| `<metric_name>` | `MetricName` | Full name including `_bucket` / `_sum` / `_count` suffixes |
| `<value>` | `MetricValue` | Parsed as `float64`; `NaN` and `+Inf` / `-Inf` are valid |
| `<labels>` | `MetricTags` | Keys and values from `{k="v", …}` block |
| `<timestamp_ms>` | `Timestamp` | Unix milliseconds; falls back to `time.Now()` if absent |
| `# HELP` text | `Attrs["metric.help"]` | Associated with subsequent data lines |
| `# TYPE` value | `Attrs["metric.type"]` | `counter`, `gauge`, `histogram`, or `summary` |
| `"metric"` | `Type` | Always |

### Detection Condition

A line is classified as Prometheus if it matches any of:

```
^#\s+(HELP|TYPE)\s+
^[a-zA-Z_:][a-zA-Z0-9_:]*(\{[^}]*\})?\s+[-+]?[0-9]
```

The first regex catches `# HELP` / `# TYPE` directives.
The second catches data lines with or without a label set.

### HELP / TYPE Context

`# HELP` and `# TYPE` directives are parsed but not emitted as events.
They are stored in a per-parser context map and attached to subsequent
data-line events for the same metric name.

The context is reset when the parser is re-created (i.e. per input stream, not globally).

---

## Detection — key=value vs Prometheus

Detection order within the metric parser:

1. If the line starts with `#` → Prometheus directive.
2. If the line matches the Prometheus data-line regex → Prometheus data line.
3. If the line matches `<identifier>=<number>` → simple key=value.
4. Otherwise → not a metric; fall through to plain-text detector.

---

## Notes

- Both formats produce `Event.Type = "metric"`.
  Log-oriented fields (`Level`, `Message`) are empty for metric events.
- The `Source` field is set by the originating source stage, not the metric parser.
- `MetricTags` is always non-nil after parsing (initialised to `{}`).
- Prometheus label names are used verbatim as `MetricTags` keys.
  Label values have surrounding `"` stripped.