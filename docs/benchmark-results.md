# Результати бенчмарків

Середовище: Apple M3, darwin/arm64, Go 1.24

---

## Завдання 1: Throughput пайплайну

| Метрика | 1K под/с | 10K под/с | 100K под/с |
|---------|----------|-----------|------------|
| p50 затримка | 2µs | 2µs | 2µs |
| p95 затримка | 3µs | 3µs | 3µs |
| p99 затримка | 4–7µs | 4–7µs | 4–7µs |
| Throughput (actual ev/s) | ~1 001 | ~10 001 | ~97 778 |
| Heap після 1M подій | ~1 783 MB | — | — |

**Продуктивність парсера за форматом (медіана з 3 запусків):**

| Формат | ns/op | B/op | allocs/op |
|--------|-------|------|-----------|
| JSON | 2 867 ns | 2 436 B | 52 |
| ECS | 2 879 ns | 3 669 B | 57 |
| Plain | 429 ns | 972 B | 14 |

**Детектор аномалій:** 479 ns/op, 24 B/op, 1 alloc/op

Plain-текст у 6.7× швидший за JSON/ECS — немає витрат на `json.Unmarshal`. JSON та ECS приблизно однакові за швидкістю (~2.9µs), ECS важчий по пам'яті через вкладену структуру. При 100K под/с досягається ~98K actual — rate limiter на основі ticker впирається в гранулярність планувальника ОС. Heap після 1M подій — ~1.7 GB: кожен `NormalizedEvent` зберігає `Raw map[string]any` з усіма полями. При продакшн-навантаженні потрібен `sync.Pool` або обнуління `Raw` після обробки.

---

## Завдання 2: Точність детектора аномалій

Датасет: 10 000 нормальних значень N(50, 10²) + 50 аномалій у випадкових позиціях (k·σ, k ∈ {4, 5, 6}).

| threshold | window | Precision | Recall | F1 | TP | FP | FN |
|-----------|--------|-----------|--------|-----|----|----|-----|
| 2.0 | 50 | 0.116 | 0.980 | 0.208 | 49 | 373 | 1 |
| 2.5 | 50 | 0.348 | 0.960 | 0.511 | 48 | 90 | 2 |
| 3.0 | 50 | 0.776 | 0.900 | 0.833 | 45 | 13 | 5 |
| 3.0 | 100 | 0.714 | 0.900 | 0.796 | 45 | 18 | 5 |
| 3.0 | 200 | 0.787 | 0.960 | 0.865 | 48 | 13 | 2 |
| **3.5** | **100** | **1.000** | **0.860** | **0.925** | 43 | 0 | 7 |

Найкраща конфігурація: threshold=3.5, window=100, F1=0.925.

`threshold=3.5, window=100` дає Precision=1.0 — нуль хибних алертів — при Recall=0.86. Пропускаємо 7 з 50 аномалій на межі k=4σ, що прийнятно для продакшну де хибний алерт гірший за пропуск. Альтернатива `threshold=3.0, window=200` (F1=0.865) ловить більше, але дає 13 FP і потребує більшого вікна історії.

---

## Завдання 3: Time-to-Diagnose

Сценарій: 5 хв нормального трафіку → latency ×10 на payment-service→db + error rate 30% + цикл api-gw→auth→api-gw. Подано 10 800 подій, отримано 4 алерти.

| Метрика | Результат | Ціль |
|---------|-----------|------|
| Detection latency | 19.875µs | < 2s |
| Кроків у TUI drill-down | 3 | ≤ 3 |

| # | Дія | Екран |
|---|-----|-------|
| 1 | Запуск | Service List — payment-service підсвічений |
| 2 | Enter | Edge List — ребро payment-service→db показує spike |
| 3 | Enter | Event Detail — ZScore, latency ~500ms, timestamp |

Детектор реагує на latency spike ×10 за перші кілька тіків інцидентної фази. Detection latency 19µs зумовлений синхронним дрейном каналу після кожного Feed. Drill-down вкладається у 3 переходи: Service List → Edge List → Event Detail.

---

## Оптимальна конфігурація детектора

```yaml
anomaly:
  window_size: 100
  threshold:   3.5
  min_samples: 50
  cooldown:    30s
```

`threshold=3.5` дає Precision=1.0 при F1=0.925. `window=100` — достатня історія для стабільного σ і швидка реакція на нові сервіси. `min_samples=50` запобігає спрацюванням при прогріві. `cooldown=30s` пригнічує дублюючі алерти в межах одного інциденту.

Основні вузькі місця пайплайну: `json.Unmarshal` (головний споживач CPU), алокації `map` в `ParseJSON` (52 allocs/op), `Raw map[string]any` в `NormalizedEvent` (~1.7 GB heap на 1M подій). Оптимізація — `sync.Pool` та обнуління `Raw` після передачі в граф.

---

## Відтворення

```bash
go test -v -run 'TestDatasetSanity|TestAnomalyEval' ./bench/...
go test -v -run 'TestTimeToDiagnose' ./bench/...
go test -bench=. -benchmem -count=3 ./bench/...

go test -bench=. -count=1 -cpuprofile=bench/profiles/cpu.prof ./bench/...
go test -bench=. -count=1 -memprofile=bench/profiles/mem.prof ./bench/...
```