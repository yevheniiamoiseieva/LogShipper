# Elastic Common Schema (ECS)

ECS is an open schema published by Elastic that defines a standard set of field names
and types for log and observability data.
Parser: `parse.ECSParser`. Specification: https://www.elastic.co/guide/en/ecs/current/

---

## Detection

A JSON object is classified as ECS when **at least one** of the following is true:

| Condition | Rationale |
|-----------|-----------|
| Field `ecs.version` is present | Explicit self-declaration |
| Field `@timestamp` is present | The `@` prefix is unique to ECS / Logstash |
| Fields `log.level` **and** `service.name` are both present | Strong indicator of ECS dot-notation |
| Field `trace.id` is present | ECS-specific tracing namespace |

ECS detection runs **before** the generic JSON parser.
If any condition above is true, `parse.ECSParser` handles the event exclusively.

---

## Canonical ECS Event

```json
{
  "@timestamp":               "2024-03-15T12:34:56.789Z",
  "ecs.version":              "8.11.0",
  "message":                  "Payment processing failed",
  "log.level":                "error",
  "log.logger":               "payment.processor",
  "service.name":             "payment-service",
  "service.version":          "2.3.1",
  "service.environment":      "production",
  "trace.id":                 "abc123def456abc123def456abc123de",
  "transaction.id":           "xyz7890123456789",
  "span.id":                  "abcdef1234567890",
  "http.request.method":      "POST",
  "http.request.body.bytes":  256,
  "http.response.status_code": 500,
  "http.response.body.bytes": 128,
  "url.full":                 "https://api.example.com/v1/payments",
  "url.path":                 "/v1/payments",
  "url.domain":               "api.example.com",
  "client.ip":                "10.0.0.5",
  "client.port":              54321,
  "error.message":            "Insufficient funds",
  "error.type":               "PaymentException",
  "error.stack_trace":        "PaymentException: Insufficient funds\n    at ...",
  "host.name":                "worker-prod-3",
  "host.ip":                  ["172.16.0.10"],
  "event.dataset":            "payment.log",
  "event.module":             "payment",
  "event.kind":               "event",
  "event.category":           ["web"],
  "event.type":               ["error"]
}
```

---

## Field Mapping — ECS → Event

ECS field names are deterministic, so the mapping is a direct lookup
rather than an alias search.

### Named Event Fields

| ECS field | `Event` field | Notes |
|-----------|--------------|-------|
| `@timestamp` | `Timestamp` | RFC 3339; always UTC |
| `log.level` | `Level` | Normalised to lowercase |
| `message` | `Message` | — |
| `service.name` | `Service` | — |

### Attrs — Tracing

| ECS field | `Attrs` key |
|-----------|------------|
| `trace.id` | `"trace.id"` |
| `transaction.id` | `"transaction.id"` |
| `span.id` | `"span.id"` |

### Attrs — HTTP

| ECS field | `Attrs` key |
|-----------|------------|
| `http.request.method` | `"http.request.method"` |
| `http.request.body.bytes` | `"http.request.body.bytes"` |
| `http.response.status_code` | `"http.response.status_code"` |
| `http.response.body.bytes` | `"http.response.body.bytes"` |

### Attrs — URL

| ECS field | `Attrs` key |
|-----------|------------|
| `url.full` | `"url.full"` |
| `url.path` | `"url.path"` |
| `url.domain` | `"url.domain"` |
| `url.query` | `"url.query"` |

### Attrs — Network participants

| ECS field | `Attrs` key |
|-----------|------------|
| `client.ip` | `"client.ip"` |
| `client.port` | `"client.port"` |
| `server.ip` | `"server.ip"` |
| `server.port` | `"server.port"` |

### Attrs — Error

| ECS field | `Attrs` key |
|-----------|------------|
| `error.message` | `"error.message"` |
| `error.type` | `"error.type"` |
| `error.stack_trace` | `"error.stack_trace"` |
| `error.code` | `"error.code"` |

### Attrs — Host

| ECS field | `Attrs` key |
|-----------|------------|
| `host.name` | `"host.name"` |
| `host.ip` | `"host.ip"` |
| `host.os.name` | `"host.os.name"` |
| `host.os.version` | `"host.os.version"` |

### Attrs — Event classification

| ECS field | `Attrs` key |
|-----------|------------|
| `event.dataset` | `"event.dataset"` |
| `event.module` | `"event.module"` |
| `event.kind` | `"event.kind"` |
| `event.category` | `"event.category"` |
| `event.action` | `"event.action"` |

### Attrs — User

| ECS field | `Attrs` key |
|-----------|------------|
| `user.name` | `"user.name"` |
| `user.id` | `"user.id"` |
| `user.email` | `"user.email"` |

### Remaining fields

Any ECS field not listed above is stored in `Attrs` **with its original key**
(dot-notation preserved).
This ensures forward compatibility as new ECS field sets are introduced.

---

## Field Sets Reference

The following ECS field sets are relevant to LogShipper.
Full definitions are in the [ECS specification](https://www.elastic.co/guide/en/ecs/current/ecs-field-reference.html).

| Field set | Description |
|-----------|-------------|
| `log` | Attributes of the log record itself (level, logger, file/line) |
| `service` | Logical service identity and deployment environment |
| `trace` | Distributed tracing identifiers (OpenTelemetry-compatible) |
| `http` | HTTP request and response details |
| `url` | Parsed URL components |
| `client` / `server` | Network participants |
| `error` | Exception / error details including stack trace |
| `host` | Physical or virtual machine running the service |
| `event` | Event classification (kind, category, action) |
| `user` | Authenticated user |
| `network` | Network-level details (protocol, bytes transferred) |
| `process` | OS process (pid, name, executable) |
| `container` | Container identity (id, name, image) — overlaps with Docker metadata |

---

## Notes

- **`ecs.version`** should be included in every ECS event to signal the schema version.
  LogShipper does not require it but uses it as a detection signal.
- ECS natively supports **distributed tracing** via `trace.id`, `transaction.id`,
  and `span.id`, which map directly to OpenTelemetry concepts.
- The dot-notation used by ECS (e.g. `service.name`) is preserved as-is in `Attrs`.
  No additional flattening is applied because ECS keys are already flat strings.