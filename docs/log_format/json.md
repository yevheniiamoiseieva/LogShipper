# JSON Log Format

A line is treated as JSON when it begins with `{` and is valid JSON.
Parsing is handled by `parse.JSONParser`.

---

## Overview

JSON logging has no universal schema. Different frameworks use different key names
for the same semantic fields. The parser resolves this through an **alias table**:
a prioritised list of known field names for each semantic role.

---

## Supported Structures

### Flat (Go `slog`, Logrus, Zap)

```json
{
  "time":    "2024-03-15T12:34:56.789Z",
  "level":   "error",
  "msg":     "Database connection failed",
  "service": "user-api",
  "db_host": "postgres:5432",
  "retry":   3
}
```

### Nested (Node.js `pino`)

```json
{
  "level":    50,
  "time":     1710506096789,
  "pid":      1234,
  "hostname": "worker-1",
  "msg":      "Request failed",
  "req": {
    "method": "POST",
    "url":    "/api/checkout",
    "headers": { "x-request-id": "abc-123" }
  },
  "res": {
    "statusCode":   500,
    "responseTime": 1523
  },
  "err": {
    "type":    "Error",
    "message": "timeout",
    "stack":   "Error: timeout\n    at ..."
  }
}
```

### Deeply structured (Winston)

```json
{
  "timestamp": "2024-03-15T12:34:56.789Z",
  "level":     "warn",
  "message":   "Slow query",
  "metadata": {
    "query":       "SELECT * FROM orders WHERE …",
    "duration_ms": 2340,
    "db": {
      "host": "pg-primary",
      "port": 5432
    }
  }
}
```

---

## Field Alias Resolution

For each semantic role the parser tries each alias in order and takes the **first match**.

### Timestamp

| Priority | Key |
|----------|-----|
| 1 | `@timestamp` |
| 2 | `timestamp` |
| 3 | `time` |
| 4 | `ts` |
| 5 | `datetime` |
| 6 | `date` |

Accepted value formats (auto-detected):

| Format | Example |
|--------|---------|
| RFC 3339 / ISO 8601 | `"2024-03-15T12:34:56.789Z"` |
| Unix seconds (float) | `1710506096.789` |
| Unix milliseconds (int > 1e12) | `1710506096789` |
| Custom date-time string | `"2024-03-15 12:34:56,789"` |

### Level

| Priority | Key |
|----------|-----|
| 1 | `level` |
| 2 | `severity` |
| 3 | `loglevel` |
| 4 | `log_level` |
| 5 | `lvl` |

Numeric pino levels are mapped as follows:

| pino value | `Event.Level` |
|-----------|--------------|
| `10` | `debug` |
| `20` | `debug` |
| `30` | `info` |
| `40` | `warn` |
| `50` | `error` |
| `60` | `fatal` |

### Message

| Priority | Key |
|----------|-----|
| 1 | `message` |
| 2 | `msg` |
| 3 | `text` |
| 4 | `body` |
| 5 | `log` |

### Service name

| Priority | Key |
|----------|-----|
| 1 | `service` |
| 2 | `service_name` |
| 3 | `serviceName` |
| 4 | `app` |
| 5 | `application` |
| 6 | `logger` |
| 7 | `name` |

---

## Nested Object Flattening

Nested objects are flattened into dot-notation keys in `Attrs`.

```jsonc
// Input
{
  "req": {
    "method": "GET",
    "url":    "/api/users",
    "headers": { "x-request-id": "abc-123" }
  }
}

// Result in Attrs
{
  "req.method":              "GET",
  "req.url":                 "/api/users",
  "req.headers.x-request-id": "abc-123"
}
```

**Depth limit:** recursion stops at **5 levels**. Objects nested deeper are stored
as raw `map[string]any` under their parent key.

**Array handling:** arrays are stored as-is (`[]any`) under their key — elements
are not individually flattened.

---

## Field Mapping Summary

```
Resolved timestamp key   → Event.Timestamp
Resolved level key       → Event.Level      (normalised, see event-model.md)
Resolved message key     → Event.Message
Resolved service key     → Event.Service
All remaining keys       → Event.Attrs      (nested objects flattened)
```

---

## Well-Known Framework Examples

The table below shows the exact JSON keys emitted by common logging libraries
and how they resolve through the alias table.

| Framework | Timestamp key | Level key | Message key | Notes |
|-----------|--------------|-----------|-------------|-------|
| Go `slog` (default) | `time` | `level` | `msg` | — |
| Go `logrus` | `time` | `level` | `msg` | — |
| Go `zap` | `ts` | `level` | `msg` | `ts` is Unix float |
| Node.js `pino` | `time` | `level` | `msg` | `time` is Unix ms, `level` is numeric |
| Node.js `winston` | `timestamp` | `level` | `message` | — |
| Python `python-json-logger` | `asctime` / `created` | `levelname` | `message` | `created` is Unix float |
| Java `logback` + `logstash-encoder` | `@timestamp` | `level` | `message` | Resembles ECS — check for `@timestamp` first |

> **Note:** Lines from `logback` + `logstash-encoder` often resemble ECS
> because they use `@timestamp`. The ECS detector runs **before** the generic
> JSON detector and will claim them if other ECS markers are present.
> See [ecs.md](ecs.md#detection) for the exact detection conditions.

---

## Edge Cases

| Scenario | Behaviour |
|----------|-----------|
| Valid JSON but not an object (e.g. `[1,2,3]`, `"hello"`) | Treated as plain text |
| JSON object with no recognised timestamp | `Timestamp = time.Now()` |
| JSON object with no recognised message | `Message = ""` |
| Field value is `null` | Stored in `Attrs` as `nil` |
| Duplicate keys | Last value wins (standard Go `encoding/json` behaviour) |
| Key is empty string `""` | Stored in `Attrs[""]` |