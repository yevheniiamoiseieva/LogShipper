# Docker Container Logs

Docker's default `json-file` log driver wraps every line of container output
in a thin JSON envelope. Parser: `parse.DockerParser`.

---

## Log Driver Overview

When a container writes to `stdout` or `stderr`, Docker captures the bytes and
appends them to a JSON file on the host:

```
/var/lib/docker/containers/<container-id>/<container-id>-json.log
```

Each line of container output becomes **one JSON object** in that file.

---

## Envelope Format

```json
{
  "log":    "2024-03-15T12:34:56.789Z ERROR Payment failed: insufficient funds\n",
  "stream": "stderr",
  "time":   "2024-03-15T12:34:56.789123456Z"
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `log` | `string` | Raw output of the container, including the trailing `\n` |
| `stream` | `string` | `"stdout"` or `"stderr"` |
| `time` | `string` | Docker ingestion timestamp in RFC 3339 Nano |

### Detection Condition

A JSON object is classified as Docker format when **all three fields** `log`,
`stream`, and `time` are present and `stream` is either `"stdout"` or `"stderr"`.

---

## Two-Pass Parsing

Docker only wraps the output — it does not interpret its contents.
The parser therefore performs two passes:

```
Pass 1 — envelope extraction
  Timestamp  ← time  (RFC 3339 Nano, from Docker daemon)
  Attrs["container.stream"] ← stream
  raw ← log  (trailing \n stripped)

Pass 2 — inner content parsing
  Feed raw through the standard detection pipeline:
    ├─ JSON?  → DockerJSON / ECS / generic JSON parser
    ├─ Metric? → metric parser
    └─ Plain text? → syslog / nginx / python / fallback
```

If the inner `log` string contains its **own** timestamp (e.g. the application
emits structured JSON with a `time` field), the **inner timestamp takes precedence**
over the Docker envelope `time` field. The Docker `time` is then stored in
`Attrs["container.ingestion_time"]` for reference.

---

## Container Metadata

When the `docker` source is active, it attaches container metadata to every
event **before** the parser runs:

```go
// Injected by sources/docker.go before parse
event.Source  = "docker:" + containerID
event.Service = containerName  // leading "/" stripped
event.Attrs["container.id"]    = containerID
event.Attrs["container.name"]  = containerName
event.Attrs["container.image"] = imageName       // e.g. "mycompany/payment:2.3.1"
```

**Label propagation** — Docker Compose labels are mapped to `Attrs`:

| Docker label | `Attrs` key |
|-------------|------------|
| `com.docker.compose.service` | `"container.labels.compose_service"` |
| `com.docker.compose.project` | `"container.labels.compose_project"` |
| Any other label `k` | `"container.labels.<k>"` (unsafe chars replaced with `_`) |

If the inner content also provides a `Service` value (e.g. via `service.name`
in ECS JSON), that value **overrides** the container name.

---

## Multi-Line Log Records

Docker splits output at newline boundaries. A single application log record
that spans multiple lines (e.g. a Java stack trace) arrives as several
separate Docker envelope objects.

### Problem

```jsonc
// Line 1
{"log": "2024-03-15T12:34:56Z ERROR NullPointerException\n", "stream": "stderr", "time": "..."}
// Line 2
{"log": "    at com.example.Service.process(Service.java:42)\n", "stream": "stderr", "time": "..."}
// Line 3
{"log": "    at com.example.Main.main(Main.java:10)\n", "stream": "stderr", "time": "..."}
```

Without assembly these become three separate `Event` objects.

### Assembly Heuristic

The Docker source buffers lines per `(container_id, stream)` pair and applies
the following rule:

- **Start of a new record:** the inner `log` string matches a "start-of-record"
  pattern — a leading timestamp, or a known log-level token at a known position.
- **Continuation:** the inner `log` string begins with whitespace (`\t` or ` `)
  **or** does not match any start-of-record pattern.

Continuation lines are appended to `Event.Message` (joined with `\n`).
The assembled event is emitted when the next start-of-record line is seen,
or when the container stream closes.

**Flush timeout:** if no new line arrives within **5 seconds**, a buffered
partial record is flushed as-is to prevent indefinite blocking.

---

## Field Mapping Summary

| Source | `Event` field | Notes |
|--------|--------------|-------|
| `time` (envelope) | `Timestamp` | Overridden by inner timestamp if present |
| `stream` | `Attrs["container.stream"]` | `"stdout"` or `"stderr"` |
| inner `log` content | `Message` (and other fields) | Parsed by inner pass |
| container ID | `Attrs["container.id"]` | From source metadata |
| container name | `Service` | Overridable by inner content |
| image name | `Attrs["container.image"]` | — |
| Docker labels | `Attrs["container.labels.*"]` | See label table above |

---

## Notes

- The `json-file` driver is Docker's default. Other drivers (`journald`, `syslog`,
  `fluentd`, etc.) produce different formats and are not currently supported.
- Container IDs are the full 64-character hex string. The `Source` field uses
  only the first 12 characters for readability: `docker:a3f8b2c1d4e5`.
- The multi-line assembly buffer holds **at most 1000 lines** per stream to bound
  memory usage. If this limit is reached the buffer is flushed immediately.