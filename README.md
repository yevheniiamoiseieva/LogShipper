# Logshipper 

A simple, fast, and extensible log shipper written in Go. This utility reads log streams from standard input (**stdin**), supports transformation of **structured JSON logs**, and outputs them to the console or forwards them via **HTTP POST** requests to a log aggregation endpoint (e.g., Loki, Elasticsearch, or a Webhook).

##  Features

* **Flexible Input:** Reads logs from `stdin`, making it easy to pipe output from `docker logs -f`, Kubernetes (`kubectl logs`), or files (`tail -f`).
* **JSON Transformation:** Automatically extracts metadata (**Timestamp**, **Log Level**) from structured JSON logs for standardization.
* **Standardized Output:** Converts all incoming data into a uniform **`LogRecord`** structure before processing.
* **Forwarding:** Sends standardized log records as JSON via HTTP POST to a configurable endpoint.
* **Graceful Shutdown:** Handles termination signals (`SIGINT`, `SIGTERM`) cleanly.

---

##  Usage

The program is configured entirely via command-line flags.

### Available Flags

| Flag | Default Value | Description |
| :--- | :--- | :--- |
| `-source` | `stdin` | Origin of the log (e.g., `docker`, `k8s`, `stdin`). Used for the `Source` field in the log record. |
| `-service` | `""` | Name of the service generating the log (e.g., `my-api`, `database`). Used for the `Service` field. |
| `-endpoint` | `""` | Target URL for HTTP POST forwarding (e.g., `http://localhost:3100/loki/api/v1/push`). If empty, output is written to console. |
| `-format` | `text` | Log format: `text` (raw line) or **`json`** (enables extraction of `ts`/`time` and `level` fields). |

---

##  Examples

### 1. Console Output (Testing)

Read a simple text log from standard input and print the standardized output to the console.

```bash
echo "Application started successfully" | ./logshipper -source=local -service=init
# Output: [CURRENT_TIME] [init] () Application started successfully
