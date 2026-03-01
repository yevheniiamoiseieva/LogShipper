# Plain-Text Log Formats

Plain-text formats embed structure inside a human-readable string.
Unlike JSON there is no universal schema; the parser identifies the dialect
from the shape of the line and applies the matching regex.

---

## Dialects

| Dialect | Detection anchor | Covered by |
|---------|-----------------|------------|
| [Syslog RFC 3164](#syslog-rfc-3164) | `<priority>` header or 3-letter month prefix | `parse.SyslogParser` |
| [Syslog RFC 5424](#syslog-rfc-5424) | `<priority>version` prefix, e.g. `<34>1 ` | `parse.SyslogParser` |
| [Nginx access log](#nginx-access-log) | IPv4/IPv6 address followed by `[DD/Mon/YYYY` | `parse.NginxParser` |
| [Python `logging`](#python-logging) | `YYYY-MM-DD HH:MM:SS` followed by a known level | `parse.PythonLogParser` |
| [Fallback](#fallback) | Everything else | built-in |

---

## Syslog RFC 3164

### Wire Format

```
<PRIORITY>TIMESTAMP HOSTNAME APP[PID]: MESSAGE
```

The `PRIORITY` value encodes both **facility** (upper bits) and **severity** (lower 3 bits).

```
priority = facility * 8 + severity
```

### Example

```
<34>Mar 15 12:34:56 web-01 nginx[8192]: 10.0.0.1 - - [15/Mar/2024:12:34:56 +0000] "GET / HTTP/1.1" 200 612
```

### Field Mapping

| RFC 3164 field | Example value | `Event` field |
|----------------|--------------|---------------|
| `PRIORITY` | `34` | → severity bits → `Level` |
| `TIMESTAMP` | `Mar 15 12:34:56` | `Timestamp` (year = current) |
| `HOSTNAME` | `web-01` | `Attrs["host.name"]` |
| `APP` | `nginx` | `Service` |
| `PID` | `8192` | `Attrs["process.pid"]` |
| `MESSAGE` | everything after `: ` | `Message` |

### Severity → Level Mapping

| Code | Syslog name | `Event.Level` |
|------|------------|--------------|
| 0 | Emergency | `fatal` |
| 1 | Alert | `fatal` |
| 2 | Critical | `error` |
| 3 | Error | `error` |
| 4 | Warning | `warn` |
| 5 | Notice | `info` |
| 6 | Informational | `info` |
| 7 | Debug | `debug` |

---

## Syslog RFC 5424

### Wire Format

```
<PRIORITY>VERSION TIMESTAMP HOSTNAME APP PID MSGID [STRUCTURED-DATA] MESSAGE
```

Differences from RFC 3164:

- `VERSION` is always `1`.
- `TIMESTAMP` is ISO 8601 / RFC 3339 (timezone-aware).
- `STRUCTURED-DATA` is a `[...]` block of typed key-value pairs.
- Nil fields use `-`.

### Example

```
<165>1 2024-03-15T12:34:56.000Z web-01 payment 1234 req-99 [origin ip="10.0.0.5"] Transaction approved
```

### Field Mapping

| RFC 5424 field | Example value | `Event` field |
|----------------|--------------|---------------|
| `PRIORITY` | `165` | `Level` (severity bits) |
| `VERSION` | `1` | ignored |
| `TIMESTAMP` | `2024-03-15T12:34:56.000Z` | `Timestamp` |
| `HOSTNAME` | `web-01` | `Attrs["host.name"]` |
| `APP` | `payment` | `Service` |
| `PID` | `1234` | `Attrs["process.pid"]` |
| `MSGID` | `req-99` | `Attrs["syslog.msgid"]` |
| Structured-data params | `ip="10.0.0.5"` | `Attrs["syslog.sd.<key>"]` |
| `MESSAGE` | `Transaction approved` | `Message` |

---
