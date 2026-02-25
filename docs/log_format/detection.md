# Format Auto-Detection

`internal/parse/detect.go`

The parser identifies the format of every incoming line automatically.
No configuration is required. The algorithm runs per-line, in the order
described below.

---

## Detection Pipeline

```
┌──────────────────────────────────────────────────────┐
│  Input: raw []byte                                    │
└──────────────────────────┬───────────────────────────┘
                           │
              first byte == '{'  ?
                 yes │         no │
                     ▼           ▼
          ┌──────────────┐   ┌─────────────────────────────────┐
          │  json.Valid? │   │  starts with '#' or matches     │
          └──────┬───────┘   │  metric data-line pattern?      │
          no ◄───┘   yes     └────────────┬────────────────────┘
          │         │          yes │          no │
          │         ▼              ▼             ▼
          │  ┌──────────────┐  ┌──────────┐  ┌───────────────────────┐
          │  │ Docker?      │  │ Metric   │  │ Plain-text detector   │
          │  │ (log+stream  │  │ parser   │  │ (dialect chain)       │
          │  │  +time)      │  └──────────┘  └──────────┬────────────┘
          │  └──────┬───────┘                            │
          │  no ◄───┘   yes                    ┌─────────┴──────────┐
          │  │          │                      │                    │
          │  │          ▼                   syslog?              nginx?
          │  │   ┌─────────────┐              │                    │
          │  │   │ Docker      │           python?             fallback
          │  │   │ parser      │              │
          │  │   │ + inner     │           fallback
          │  │   │ re-parse    │
          │  │   └─────────────┘
          │  │
          │  ▼
          │ ┌──────────────────┐
          │ │ ECS?             │
          │ │ (@timestamp or   │
          │ │  ecs.version or  │
          │ │  trace.id or     │
          │ │  log.level +     │
          │ │  service.name)   │
          │ └──────┬───────────┘
          │  no ◄──┘   yes
          │  │         │
          │  │         ▼
          │  │   ┌────────────┐
          │  │   │ ECS parser │
          │  │   └────────────┘
          │  │
          │  ▼
          │ ┌──────────────────┐
          │ │ Generic JSON     │
          │ │ parser           │
          │ └──────────────────┘
          │
          ▼
      ┌──────────────────────────────┐
      │ Fallback                     │
      │ Message = raw string         │
      │ Timestamp = time.Now()       │
      └──────────────────────────────┘
```

---

## Detection Conditions — Quick Reference

### Step 1 — JSON gate

| Condition | Parser chain entered |
|-----------|---------------------|
| First byte is `{` **and** line is valid JSON | JSON branch |
| Otherwise | Non-JSON branch |

> **Performance note:** `json.Valid` is called only after the first-byte check.
> Non-JSON lines incur no JSON parsing overhead.

### Step 2a — Docker (inside JSON branch)

| Condition | Result |
|-----------|--------|
| Object has all three fields `log` (string), `stream` (`"stdout"` or `"stderr"`), `time` (string) | Docker parser |

### Step 2b — ECS (inside JSON branch, not Docker)

Any **one** of the following is sufficient:

| Condition |
|-----------|
| `ecs.version` key present |
| `@timestamp` key present |
| `trace.id` key present |
| Both `log.level` **and** `service.name` keys present |

### Step 2c — Generic JSON

Reached when JSON branch is entered but neither Docker nor ECS conditions match.

### Step 3 — Metric (inside non-JSON branch)

| Condition | Result |
|-----------|--------|
| Line starts with `#` followed by `HELP` or `TYPE` | Prometheus directive |
| Line matches `^[a-zA-Z_:][a-zA-Z0-9_:]*(\{[^}]*\})?\s+[-+]?[0-9]` | Prometheus data line |
| Line matches `^[a-zA-Z_][a-zA-Z0-9_.]*=[-+]?[0-9]` | Simple key=value metric |

### Step 4 — Plain-text dialect chain

Detectors run in this order; the first match wins:

| Priority | Dialect | Detection condition |
|----------|---------|---------------------|
| 1 | Syslog RFC 5424 | Starts with `<digits>1 ` (priority + version `1`) |
| 2 | Syslog RFC 3164 | Starts with `<digits>` followed by 3-letter month |
| 3 | Syslog (no priority) | Starts with 3-letter month and matches timestamp pattern |
| 4 | Nginx access log | Starts with IPv4/IPv6 address followed by ` - ` |
| 5 | Python `logging` | Starts with `YYYY-MM-DD HH:MM:SS` and contains a known level token |

### Step 5 — Fallback

Reached when all above conditions fail. The raw bytes become `Message`.

---

## Inner Re-Parse (Docker)

After the Docker envelope is extracted, the value of `log` is fed back through
the **full detection pipeline** (steps 1–5) as a new raw input.

The inner parse result is merged into the outer event:

- Fields set by the inner parser (`Level`, `Message`, `Service`, remaining `Attrs`)
  **override** values from the envelope.
- `Timestamp` from the inner content **overrides** the Docker `time` field;
  the Docker `time` is stored in `Attrs["container.ingestion_time"]`.
- `Attrs["container.*"]` fields set by the Docker source stage are **never** overridden
  by the inner parse.

---

## Performance Characteristics

The detector is designed to operate at >100,000 events/second on a single goroutine.

| Technique | Benefit |
|-----------|---------|
| First-byte check before `json.Valid` | JSON parsing skipped entirely for plain-text lines |
| Pre-compiled `*regexp.Regexp` at package init | Zero allocation per call for regex-based detectors |
| `bytes.HasPrefix` / `bytes.Index` for Prometheus `#` lines | No regex overhead for the common case |
| Depth-limited JSON flattening (max 5 levels) | Bounded allocation for adversarial inputs |
| Single-pass field extraction in JSON parser | One `json.Unmarshal` call per event |

---

## Adding a New Detector

1. Implement the detection condition as a function with signature:
   ```go
   func isMyFormat(raw []byte, obj map[string]any) bool
   ```
   (`obj` is non-nil only in the JSON branch, already unmarshalled.)

2. Implement the parser:
   ```go
   func parseMyFormat(raw []byte, obj map[string]any) (event.Event, error)
   ```

3. Insert the condition + parser call at the appropriate priority level in
   `internal/parse/detect.go`.

4. Add tests in `internal/parse/myformat_test.go` covering:
    - Positive detection with a representative sample.
    - Negative detection (should not match adjacent formats).
    - Partial / malformed input (fallback behaviour).