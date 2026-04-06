# Correlation Fields

Correlation fields are the fields that make it possible to link events across
services, hosts, and time. Without them, events are isolated log lines;
with them, they form a traceable call graph.

This document lists every field that matters for cross-service correlation,
and how each format expresses it.

---

## Field Index

| Semantic role | Canonical name used internally |
|---------------|-------------------------------|
| [Trace ID](#trace-id) | `trace.id` |
| [Span ID](#span-id) | `span.id` |
| [Timestamp](#timestamp) | `timestamp` |
| [Service name](#service-name) | `service` |
| [Host / Container](#host--container) | `host.name` / `container.id` |
| [HTTP method](#http-method) | `http.request.method` |
| [HTTP URL](#http-url) | `url.path` |
| [HTTP status code](#http-status-code) | `http.response.status_code` |
| [Latency / Duration](#latency--duration) | `duration_ms` |

---

## Trace ID

The single most important correlation field. Shared by every service that
participates in a single logical request.

| Format | Field name(s) | Type | Example |
|--------|--------------|------|---------|
| ECS | `trace.id` | string | `"abc123def456abc123def456abc123de"` |
| Generic JSON | `trace_id`, `traceId`, `trace.id`, `x-trace-id` | string | `"abc123def456"` |
| Syslog RFC 5424 | structured-data param, e.g. `[req traceId="abc123"]` | string | `"abc123"` |
| Nginx access log | not present by default; available if `$http_x_trace_id` added to log format | string | — |
| Python `logging` | not a standard field; added via `extra={"trace_id": ...}` or `LogRecord` filter | string | — |
| Docker | inherited from inner log content | — | — |
| Prometheus metrics | label, e.g. `{trace_id="abc123"}` | label | rarely used |

**Alias resolution order** (first match wins):

```
trace.id  →  traceId  →  trace_id  →  x-trace-id  →  X-Trace-Id
```

---

## Span ID

Identifies a single unit of work within a trace. Changes at every service hop.

| Format | Field name(s) | Example |
|--------|--------------|---------|
| ECS | `span.id` | `"abcdef1234567890"` |
| Generic JSON | `span_id`, `spanId`, `span.id` | `"abcdef12"` |
| Syslog RFC 5424 | structured-data param | `"abcdef12"` |
| Nginx, Python, Docker | not present by default | — |

**Alias resolution order:**

```
span.id  →  spanId  →  span_id
```

> `trace.id` links events across services; `span.id` links parent and child
> operations within one trace. Both are needed to reconstruct the call tree.

---

## Timestamp

Every correlation algorithm depends on time ordering. Inconsistent timestamp
formats or missing timezone information will silently misorder events.

| Format | Field name(s) | Value format | Example |
|--------|--------------|-------------|---------|
| ECS | `@timestamp` | RFC 3339, always UTC | `"2024-03-15T12:34:56.789Z"` |
| Generic JSON | `timestamp`, `time`, `ts`, `datetime`, `date` | RFC 3339 / Unix sec / Unix ms | `1710506096789` |
| Syslog RFC 5424 | positional field 2 | RFC 3339 | `"2024-03-15T12:34:56.000Z"` |
| Syslog RFC 3164 | positional field 1 | `MMM DD HH:MM:SS` (no year, no tz) | `"Mar 15 12:34:56"` |
| Nginx access log | `[$time_local]` | `DD/Mon/YYYY:HH:MM:SS +ZZZZ` | `"15/Mar/2024:12:34:56 +0300"` |
| Python `logging` | `%(asctime)s` | `YYYY-MM-DD HH:MM:SS,mmm` | `"2024-03-15 12:34:56,789"` |
| Docker (envelope) | `time` | RFC 3339 Nano | `"2024-03-15T12:34:56.789123456Z"` |
| Prometheus | optional trailing integer | Unix milliseconds | `1710506096789` |

**Known precision and timezone hazards:**

| Format | Hazard |
|--------|--------|
| Syslog RFC 3164 | No year, no timezone. Year assumed = current; timezone assumed = local. |
| Python `%(asctime)s` | Timezone not included unless `Formatter` is subclassed. |
| Nginx `$time_local` | Uses server local time; `$time_iso8601` is preferred but not the default. |
| Unix milliseconds vs seconds | Values > 1 × 10¹² are treated as milliseconds; otherwise seconds. |

All timestamps are normalised to **UTC** before being stored in `Event.Timestamp`.

---

## Service Name

Identifies which service produced the event. Essential for grouping and
for drawing edges in the dependency graph.

| Format | Field name(s) | Example |
|--------|--------------|---------|
| ECS | `service.name` | `"payment-service"` |
| Generic JSON | `service`, `service_name`, `serviceName`, `app`, `application`, `logger`, `name` | `"payment"` |
| Syslog RFC 3164 / 5424 | `APP` (positional) | `"nginx"`, `"sshd"` |
| Nginx access log | not present; inferred from `Source` or config | — |
| Python `logging` | `%(name)s` (logger name) | `"myapp.payment"` |
| Docker | container name (stripped of `/`) | `"payment-service-1"` |
| Prometheus | label `job` or `service` | `{job="payment"}` |

**Resolution priority** when multiple candidates are present:

```
1. ECS service.name
2. Generic JSON alias (service → app → application → logger → name)
3. Syslog APP field
4. Docker container name
5. Empty string
```

---

## Host / Container

Identifies the machine or container that produced the event.
Required for infrastructure-level correlation and for distinguishing
multiple instances of the same service.

### Host name

| Format | Field name(s) | Example |
|--------|--------------|---------|
| ECS | `host.name` | `"worker-prod-3"` |
| Generic JSON | `hostname`, `host`, `host_name` | `"worker-1"` |
| Syslog RFC 3164 / 5424 | `HOSTNAME` (positional) | `"web-01"` |
| Nginx access log | not present (server-side; add via `$hostname` variable) | — |
| Python `logging` | not standard; add via `socket.gethostname()` in filter | — |
| Docker | `host.name` from container metadata | `"docker-host-01"` |
| Prometheus | label `instance` | `{instance="worker-1:9090"}` |

### Container ID

| Format | Field name(s) | Example |
|--------|--------------|---------|
| ECS | `container.id` | `"a3f8b2c1d4e5"` |
| Generic JSON | `container_id`, `containerId` | `"a3f8b2c1d4e5"` |
| Docker | injected by source stage as `container.id` | `"a3f8b2c1d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9"` |
| Syslog, Nginx, Python | not present | — |

---

## HTTP Method

| Format | Field name(s) | Example |
|--------|--------------|---------|
| ECS | `http.request.method` | `"POST"` |
| Generic JSON | `method`, `http_method`, `request.method`, `http.method` | `"GET"` |
| Nginx access log | extracted from request line `"GET /path HTTP/1.1"` | `"GET"` |
| Syslog | not present | — |
| Python `logging` | not standard; app-specific | — |
| Prometheus | label `method` | `{method="GET"}` |

---

## HTTP URL

| Format | Field name(s) | Example |
|--------|--------------|---------|
| ECS | `url.full`, `url.path`, `url.domain` | `"/v1/payments"` |
| Generic JSON | `url`, `path`, `uri`, `request_uri`, `http.url`, `url.path` | `"/api/users"` |
| Nginx access log | extracted from request line; query string split into `url.query` | `"/api/users?page=2"` |
| Syslog | not present | — |
| Python `logging` | not standard; app-specific | — |
| Prometheus | label `handler` or `path` | `{handler="/api/users"}` |

---

## HTTP Status Code

| Format | Field name(s) | Type | Example |
|--------|--------------|------|---------|
| ECS | `http.response.status_code` | integer | `500` |
| Generic JSON | `status`, `status_code`, `statusCode`, `http_status`, `response_code` | integer or string | `"200"` |
| Nginx access log | `$status` (positional) | integer | `404` |
| Syslog | not present | — | — |
| Python `logging` | not standard; app-specific | — | — |
| Prometheus | label `status` or `status_code` | string label | `{status="500"}` |

String values (`"200"`) are coerced to integer when stored in `Attrs`.

---

## Latency / Duration

Measures how long an operation took. Critical for performance anomaly detection.

| Format | Field name(s) | Unit | Example |
|--------|--------------|------|---------|
| ECS | `event.duration` | nanoseconds | `1523000000` |
| Generic JSON | `duration`, `duration_ms`, `latency`, `latency_ms`, `elapsed`, `response_time`, `responseTime` | varies — see note | `1523` |
| Nginx access log | `$request_time` (if added to format) | seconds (float) | `1.523` |
| Python `logging` | not standard; app-specific | — | — |
| Prometheus | metric name suffix `_seconds`, `_milliseconds`, or label | see metric name | `http_request_duration_seconds` |

**Unit normalisation:** LogShipper normalises all duration values to **milliseconds**
before storing in `Attrs["duration_ms"]`. Detection heuristic:

| Condition | Assumed unit | Conversion |
|-----------|-------------|------------|
| Field name contains `_ms` or `Millis` | milliseconds | × 1 |
| Field name contains `_seconds` or `_sec` | seconds | × 1000 |
| Field name contains `_ns` or `Nanos` | nanoseconds | ÷ 1000000 |
| ECS `event.duration` | nanoseconds | ÷ 1000000 |
| Value > 100000 and field name is ambiguous | milliseconds assumed | × 1 |
| Otherwise | milliseconds assumed | × 1 |

---

## Cross-Format Alias Table

Summary of all variant names, grouped by semantic role.

| Canonical | All known aliases |
|-----------|------------------|
| `trace.id` | `trace_id` · `traceId` · `x-trace-id` · `X-B3-TraceId` · `uber-trace-id` |
| `span.id` | `span_id` · `spanId` · `X-B3-SpanId` |
| `timestamp` | `@timestamp` · `time` · `ts` · `datetime` · `date` · `created` |
| `service` | `service.name` · `service_name` · `serviceName` · `app` · `application` · `logger` |
| `host.name` | `hostname` · `host` · `host_name` · `HOSTNAME` · `instance` |
| `container.id` | `container_id` · `containerId` |
| `http.request.method` | `method` · `http_method` · `request.method` · `http.method` |
| `url.path` | `url` · `path` · `uri` · `request_uri` · `http.url` · `url.full` · `handler` |
| `http.response.status_code` | `status` · `status_code` · `statusCode` · `http_status` · `response_code` |
| `duration_ms` | `duration` · `latency` · `latency_ms` · `elapsed` · `response_time` · `responseTime` · `request_time` · `event.duration` |