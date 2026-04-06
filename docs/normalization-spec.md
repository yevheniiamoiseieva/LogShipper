# Normalization Specification

This document defines the rules by which every incoming log line — regardless
of format — is transformed into a canonical `Event`.

It is the authoritative reference for:
- which source fields map to which `Event` fields
- which fields are required vs optional
- how values are coerced between types
- what happens when a field is missing or malformed

---

## Table of Contents

1. [Field Mapping Tables](#1-field-mapping-tables)
2. [Required and Optional Fields](#2-required-and-optional-fields)
3. [Type Coercion Rules](#3-type-coercion-rules)
4. [Missing Field Strategy](#4-missing-field-strategy)
5. [Edge Cases](#5-edge-cases)

---

## 1. Field Mapping Tables

### 1.1 Syslog RFC 3164

| Source field | Position / pattern | Normalised `Event` field | Notes |
|-------------|-------------------|--------------------------|-------|
| Priority (severity bits) | `<PRI> & 0x07` | `Level` | See level map in [plain-text.md](plain-text.md) |
| Timestamp | positions 1–15 | `Timestamp` | Format `MMM DD HH:MM:SS`; no year, no tz — see §5.1 |
| Hostname | word after timestamp | `Attrs["host.name"]` | |
| App-name | word before `[` or `:` | `Service` | |
| PID | digits inside `[…]` | `Attrs["process.pid"]` | integer |
| Message | everything after `: ` | `Message` | |

### 1.2 Syslog RFC 5424

| Source field | Position | Normalised `Event` field | Notes |
|-------------|----------|--------------------------|-------|
| Priority (severity bits) | `<PRI> & 0x07` | `Level` | |
| Version | field 2 | *(ignored)* | Always `1` |
| Timestamp | field 3 | `Timestamp` | RFC 3339; converted to UTC |
| Hostname | field 4 | `Attrs["host.name"]` | `-` → omitted |
| App-name | field 5 | `Service` | `-` → omitted |
| ProcID | field 6 | `Attrs["process.pid"]` | `-` → omitted |
| MsgID | field 7 | `Attrs["syslog.msgid"]` | `-` → omitted |
| Structured-data params | `[…]` block | `Attrs["syslog.sd.<key>"]` | one entry per param |
| Message | remainder | `Message` | |

### 1.3 Nginx Access Log

| Source field | Normalised `Event` field | Notes |
|-------------|--------------------------|-------|
| `$time_local` | `Timestamp` | Converted to UTC |
| HTTP method (from request line) | `Attrs["http.request.method"]` | Uppercased |
| URI path (from request line) | `Attrs["url.path"]` | Query string split off |
| Query string (from request line) | `Attrs["url.query"]` | Raw string |
| HTTP version (from request line) | `Attrs["http.version"]` | e.g. `"1.1"` |
| Full request line | `Message` | e.g. `"GET /path HTTP/1.1"` |
| `$status` | `Attrs["http.response.status_code"]` | Coerced to `int` |
| `$body_bytes_sent` | `Attrs["http.response.bytes"]` | Coerced to `int` |
| `$remote_addr` | `Attrs["client.ip"]` | |
| `$remote_user` | `Attrs["user.name"]` | `-` → omitted |
| `$http_referer` | `Attrs["http.request.referrer"]` | `-` → omitted |
| `$http_user_agent` | `Attrs["user_agent.original"]` | `-` → omitted |
| *(derived from `$status`)* | `Level` | `2xx/3xx→info`, `4xx→warn`, `5xx→error` |

### 1.4 Python `logging`

| Source field | Format token | Normalised `Event` field | Notes |
|-------------|-------------|--------------------------|-------|
| `asctime` | `%(asctime)s` | `Timestamp` | Format `YYYY-MM-DD HH:MM:SS,mmm` |
| `levelname` | `%(levelname)s` | `Level` | Normalised; `WARNING→warn`, `CRITICAL→fatal` |
| `name` | `%(name)s` | `Service` | Logger hierarchy name |
| `message` | `%(message)s` | `Message` | |
| `filename` | `%(filename)s` | `Attrs["code.filepath"]` | If present in format |
| `lineno` | `%(lineno)d` | `Attrs["code.lineno"]` | `int` |
| `funcName` | `%(funcName)s` | `Attrs["code.function"]` | If present in format |
| `process` | `%(process)d` | `Attrs["process.pid"]` | `int` |
| `thread` | `%(thread)d` | `Attrs["thread.id"]` | `int` |

### 1.5 Generic JSON

| Resolved alias group | Normalised `Event` field | Notes |
|---------------------|--------------------------|-------|
| `@timestamp` · `timestamp` · `time` · `ts` · `datetime` | `Timestamp` | First match wins |
| `level` · `severity` · `loglevel` · `lvl` | `Level` | |
| `message` · `msg` · `text` · `body` | `Message` | |
| `service` · `service_name` · `app` · `application` · `logger` | `Service` | |
| All remaining top-level keys | `Attrs` | Nested objects flattened to dot-notation, max depth 5 |

### 1.6 ECS

| ECS field | Normalised `Event` field | Notes |
|-----------|--------------------------|-------|
| `@timestamp` | `Timestamp` | |
| `log.level` | `Level` | |
| `message` | `Message` | |
| `service.name` | `Service` | |
| `trace.id` | `Attrs["trace.id"]` | |
| `transaction.id` | `Attrs["transaction.id"]` | |
| `span.id` | `Attrs["span.id"]` | |
| `http.request.method` | `Attrs["http.request.method"]` | |
| `http.response.status_code` | `Attrs["http.response.status_code"]` | `int` |
| `url.path` | `Attrs["url.path"]` | |
| `url.full` | `Attrs["url.full"]` | |
| `client.ip` | `Attrs["client.ip"]` | |
| `error.message` | `Attrs["error.message"]` | |
| `error.type` | `Attrs["error.type"]` | |
| `error.stack_trace` | `Attrs["error.stack_trace"]` | |
| `host.name` | `Attrs["host.name"]` | |
| All other ECS fields | `Attrs` | Original dot-notation key preserved |

### 1.7 Docker Envelope

| Docker field | Normalised `Event` field | Notes |
|-------------|--------------------------|-------|
| `time` | `Timestamp` | Overridden by inner content timestamp if present; then stored in `Attrs["container.ingestion_time"]` |
| `stream` | `Attrs["container.stream"]` | `"stdout"` or `"stderr"` |
| `log` | *(re-parsed)* | Inner content parsed by full detection pipeline |
| *(source metadata)* container ID | `Attrs["container.id"]` | Injected by source stage |
| *(source metadata)* container name | `Service` | Stripped of leading `/`; overridable by inner content |
| *(source metadata)* image | `Attrs["container.image"]` | |

### 1.8 Prometheus Metrics

| Prometheus element | Normalised `Event` field | Notes |
|-------------------|--------------------------|-------|
| Metric name | `MetricName` | Full name including `_bucket` / `_sum` / `_count` |
| Value | `MetricValue` | `float64`; `NaN`, `+Inf`, `-Inf` are valid |
| Labels `{k="v"}` | `MetricTags` | All labels as `map[string]string` |
| Timestamp (ms) | `Timestamp` | Unix ms → UTC; `time.Now()` if absent |
| `# TYPE` value | `Attrs["metric.type"]` | `counter`, `gauge`, `histogram`, `summary` |
| `# HELP` text | `Attrs["metric.help"]` | Associated with subsequent data lines |
| `"metric"` | `Type` | Always |

---

## 2. Required and Optional Fields

### 2.1 Required Fields

These fields are **always** set after normalisation. A normalised `Event` with
any of these missing is a bug in the parser.

| Field | Guaranteed value | How |
|-------|-----------------|-----|
| `Timestamp` | Non-zero UTC time | Parsed from source, or `time.Now()` as fallback |
| `Source` | Non-empty string | Set by source stage before parsing |
| `Type` | `"log"` or `"metric"` | Set by parser |
| `Attrs` | Non-nil map (may be empty) | Initialised by parser |
| `MetricTags` | Non-nil map (may be empty) | Initialised for metric events |

### 2.2 Optional Fields

These fields are populated when the source provides enough information.
They may be empty strings or zero values.

| Field | Empty when |
|-------|-----------|
| `Service` | Source does not include a service/app name and no container metadata is available |
| `Level` | Metric events; plain-text lines with no recognised level token |
| `Message` | Metric events |
| `MetricName` | Log events |
| `MetricValue` | Log events (`0.0` is the zero value — not meaningful for logs) |

---

## 3. Type Coercion Rules

### 3.1 Timestamp (`string` → `time.Time`)

Attempted in this order:

| Format | Example | Notes |
|--------|---------|-------|
| RFC 3339 / ISO 8601 | `"2024-03-15T12:34:56.789Z"` | Preferred |
| RFC 3339 without sub-seconds | `"2024-03-15T12:34:56Z"` | |
| RFC 3339 with offset | `"2024-03-15T12:34:56+03:00"` | Converted to UTC |
| Unix milliseconds (`int` or `float`, value > 1×10¹²) | `1710506096789` | Divided by 1000 |
| Unix seconds (`int` or `float`) | `1710506096.789` | |
| `YYYY-MM-DD HH:MM:SS,mmm` | `"2024-03-15 12:34:56,789"` | Python logging |
| `YYYY-MM-DD HH:MM:SS` | `"2024-03-15 12:34:56"` | Assumed UTC |
| `DD/Mon/YYYY:HH:MM:SS ±ZZZZ` | `"15/Mar/2024:12:34:56 +0300"` | Nginx |
| `MMM DD HH:MM:SS` | `"Mar 15 12:34:56"` | RFC 3164; year = current, tz = UTC |

If all attempts fail: `Timestamp = time.Now()` and `Attrs["_parse_warn"] = "timestamp parse failed: <raw value>"`.

### 3.2 Level (`string` or `int` → canonical string)

| Input | Output |
|-------|--------|
| `"DEBUG"`, `"debug"`, `10` (pino), `7` (syslog) | `"debug"` |
| `"INFO"`, `"info"`, `"INFORMATION"`, `"notice"`, `20`, `30` (pino), `5`, `6` (syslog) | `"info"` |
| `"WARN"`, `"WARNING"`, `"warn"`, `40` (pino), `4` (syslog) | `"warn"` |
| `"ERROR"`, `"ERR"`, `"error"`, `50` (pino), `3` (syslog) | `"error"` |
| `"FATAL"`, `"CRITICAL"`, `"EMERGENCY"`, `"ALERT"`, `60` (pino), `0`, `1`, `2` (syslog) | `"fatal"` |
| Any other string | stored verbatim in lowercase |

### 3.3 Integer fields (`string` → `int`)

Applied to: `http.response.status_code`, `http.response.bytes`,
`http.request.body.bytes`, `process.pid`, `code.lineno`, `thread.id`.

- `strconv.Atoi` is used.
- On failure: value stored as original `string` in `Attrs` and
  `Attrs["_parse_warn"]` is appended with the field name and raw value.

### 3.4 Float fields (`string` → `float64`)

Applied to: `MetricValue`, `event.duration`.

- `strconv.ParseFloat(s, 64)` is used.
- Special strings `"NaN"`, `"+Inf"`, `"-Inf"` are accepted.
- On failure: `MetricValue = 0` and parse warning recorded.

### 3.5 Duration → milliseconds (`string` or `float64` → `float64`)

All duration-like fields are normalised to milliseconds in `Attrs["duration_ms"]`.

| Condition | Assumed unit | Operation |
|-----------|-------------|-----------|
| Field name contains `_ms`, `Millis`, `Milliseconds` | ms | × 1 |
| Field name contains `_s`, `_sec`, `Seconds` | seconds | × 1000 |
| Field name contains `_ns`, `Nanos`, `Nanoseconds` | nanoseconds | ÷ 1 000 000 |
| ECS `event.duration` | nanoseconds | ÷ 1 000 000 |
| Nginx `$request_time` | seconds (float) | × 1000 |
| Ambiguous name, value > 100 000 | ms assumed | × 1 |
| Ambiguous name, value ≤ 100 000 | ms assumed | × 1 |

Original field is also preserved in `Attrs` under its source key.

### 3.6 Boolean fields (`string` → `bool`)

Applied when a JSON field value is the string `"true"` or `"false"`.

- `"true"`, `"1"`, `"yes"` → `true`
- `"false"`, `"0"`, `"no"` → `false`
- Anything else → stored as string

### 3.7 Nested JSON string → object

When a string field value is itself valid JSON (starts with `{` or `[`),
it is unmarshalled and its fields are merged into `Attrs` under the parent key.
See §5.4 for the full edge case description.

---

## 4. Missing Field Strategy

### 4.1 Overview

| Outcome | When used |
|---------|-----------|
| **Default value** | Field has a well-defined fallback (e.g. `Timestamp`) |
| **Partial event** | Field is optional; event is emitted without it |
| **Parse warning** | Field was present but could not be coerced; stored in `Attrs["_parse_warn"]` |
| **Drop event** | Never. Every input line produces exactly one `Event`. |

Events are **never dropped** due to missing or unparseable fields.

### 4.2 Field-level Strategy Table

| Field | If missing | If malformed |
|-------|-----------|-------------|
| `Timestamp` | `time.Now()` (UTC) | `time.Now()` + parse warning in `Attrs` |
| `Level` | `""` (empty string) | stored verbatim in lowercase |
| `Message` | `""` (empty string) | n/a (always a string) |
| `Service` | `""` → resolution chain (see §5.2) | n/a |
| `MetricName` | `""` | n/a |
| `MetricValue` | `0.0` + parse warning | `0.0` + parse warning |
| `http.response.status_code` | field omitted from `Attrs` | stored as string + parse warning |
| `duration_ms` | field omitted from `Attrs` | stored as string + parse warning |

### 4.3 Parse Warnings

When coercion fails, the parser appends a description to `Attrs["_parse_warn"]`
(a `[]string`). This field is always an array to accommodate multiple warnings
from the same event.

```json
{
  "_parse_warn": [
    "timestamp parse failed: 'yesterday'",
    "int coercion failed: http.response.status_code = 'OK'"
  ]
}
```

Downstream transforms and sinks can filter or alert on the presence of `_parse_warn`.

---

## 5. Edge Cases

### 5.1 Timestamp Missing

**Trigger:** the input line contains no recognisable timestamp field.

**Behaviour:**
```
Timestamp = time.Now()  // UTC, at parse time — not ingestion time of the source
```

**Affected formats:**
- Plain-text lines that do not match any known dialect (fallback path).
- Simple `key=value` metric lines.
- JSON objects where none of the timestamp alias keys are present.

**Implication for correlation:** events without a source timestamp cannot be
reliably ordered relative to events from other services. The ingestion time is
a proxy, not a ground truth. Such events should be treated with lower confidence
in the dependency graph.

---

### 5.2 Service Name Missing

**Trigger:** no service name can be extracted from the payload.

**Resolution chain** (first non-empty result wins):

```
1. Payload field   — service / service.name / app / logger / APP (syslog)
2. Container name  — injected by Docker source stage (stripped of leading "/")
3. Hostname        — Attrs["host.name"] if present
4. Source path     — last path component of file source
                     e.g. "file:/var/log/payment/app.log" → "payment"
5. Empty string    — event emitted without Service
```

Step 4 applies only to file-tail sources where the directory name implies
the service (a common convention in `/var/log/<service>/` layouts).

---

### 5.3 Syslog RFC 3164 — Missing Year and Timezone

**Trigger:** RFC 3164 timestamp `Mar 15 12:34:56` contains no year and no timezone.

**Year resolution:**
- Use current calendar year.
- Exception: if the parsed month/day is in the **future** by more than 7 days,
  subtract one year. This handles log lines from late December read in early January.

**Timezone resolution:**
- Assume UTC.
- If the source config specifies a `tz` override, apply it.

**Parse warning:** none — this is expected behaviour for RFC 3164, not a failure.

---

### 5.4 Nested JSON Inside a String Field

**Trigger:** a string-valued field contains a JSON object or array as its value.

**Example:**
```json
{
  "message": "{\"trace_id\": \"abc123\", \"user_id\": 42}",
  "level": "info"
}
```

**Behaviour:**

1. Detect that the string value starts with `{` or `[` and is valid JSON.
2. Unmarshal the inner JSON.
3. Merge the inner fields into `Attrs` under the parent key using dot-notation:
   ```
   Attrs["message.trace_id"] = "abc123"
   Attrs["message.user_id"]  = 42
   ```
4. Replace `Message` with `""` (the original string was not a human-readable message).

**Scope:** applied to the following fields only — `message`, `msg`, `log`, `body`.
Not applied to arbitrary `Attrs` fields to avoid unbounded recursion.

**Depth limit:** the inner JSON is flattened to a maximum of **3 levels** (not 5,
because it is already nested inside an outer document).

---

### 5.5 Mixed Line — Plain Text with Embedded JSON

**Trigger:** a plain-text line contains a JSON object as a suffix or infix.

**Example (common in some Java loggers):**
```
2024-03-15 12:34:56 INFO  Request handled {"trace_id":"abc123","duration_ms":312}
```

**Detection heuristic:**
1. The line matches a plain-text dialect pattern (timestamp + level at the start).
2. The line also contains a `{…}` substring.

**Behaviour:**

1. Parse the plain-text portion normally: extract `Timestamp`, `Level`, `Service`.
2. Extract `Message` as the text **before** the first `{`.
3. Attempt to parse the `{…}` substring as JSON.
4. On success: merge the JSON fields into `Attrs` (with depth limit 3).
5. On failure: keep the entire remaining text as `Message`.

**Result for the example above:**
```
Timestamp  = 2024-03-15T12:34:56Z
Level      = "info"
Message    = "Request handled"
Attrs = {
  "trace_id":   "abc123",
  "duration_ms": 312
}
```

---

### 5.6 Duplicate Fields

**Trigger:** a JSON object contains the same key more than once (technically
invalid per RFC 8259 but produced by some libraries).

**Behaviour:** last value wins — standard `encoding/json` behaviour. No warning
is emitted because the input is ambiguous by definition.

---

### 5.7 Extremely Large Events

**Trigger:** a single input line exceeds **1 MB**.

**Behaviour:**
- The line is truncated to 1 MB before parsing.
- `Attrs["_parse_warn"]` includes `"line truncated: original size N bytes"`.
- Truncation happens at a UTF-8 character boundary to avoid invalid sequences.

---

### 5.8 Non-UTF-8 Input

**Trigger:** the input bytes contain invalid UTF-8 sequences.

**Behaviour:**
- Invalid bytes are replaced with the Unicode replacement character `U+FFFD`.
- `Attrs["_parse_warn"]` includes `"non-utf8 input: N bytes replaced"`.
- Parsing continues on the sanitised string.