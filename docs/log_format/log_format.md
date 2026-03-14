# Log Format Reference

This directory documents every input format that the LogShipper pipeline can receive and parse.
The parser lives in `internal/parse/` and runs automatically after each source stage —
no user configuration is required.

## Contents

| File | Covers |
|------|--------|
| [plain-text.md](plain-text.md) | Syslog (RFC 3164 / 5424), Nginx access log, Python `logging` |
| [json.md](json.md) | Arbitrary JSON, nested objects, field-alias resolution |
| [ecs.md](ecs.md) | Elastic Common Schema — field sets, detection, mapping |
| [docker.md](docker.md) | Docker JSON log driver, container metadata, multi-line |
| [metrics.md](metrics.md) | `key=value` and Prometheus exposition format |
| [detection.md](detection.md) | Auto-detection algorithm, priority order, performance |
| [event-model.md](event-model.md) | The internal `Event` struct — the common data unit |

## Design Goals

1. **Zero config** — the pipeline detects the format of every line automatically.
2. **Best-effort** — if no known format matches, the raw string becomes `Message`; nothing is dropped.
3. **Lossless** — every field from the source lands somewhere in `Event`, either in a named field
   or in `Attrs map[string]any`.
4. **Composable** — Docker logs trigger a second parse pass on their inner `log` string,
   so all other detectors apply transparently inside a container stream.

## Quick Reference — Detection Priority

```
1. JSON object?  ──yes──▶  Docker?  ──yes──▶  docker parser  ──▶  inner log re-parsed
                │                    └──no──▶  ECS?  ──yes──▶  ecs parser
                │                              └──no──▶  generic json parser
                └──no──▶  Metric?  ──yes──▶  metric parser
                          └──no──▶  Plain-text dialect detector
                                    └──fallback──▶  raw string → Message
```

See [detection.md](detection.md) for the full algorithm with tie-breaking rules.