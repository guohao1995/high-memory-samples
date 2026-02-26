# Memory Stress Test — PHP (Apache)

## Purpose

A PHP application that **gradually increases memory usage** by a configurable target percentage each second. It streams real-time memory metrics as newline-delimited JSON to the HTTP response, so you can monitor progress in a browser or with `curl`. Designed to help test container memory limits and OOM-kill behaviour.

---

## Prerequisites

- **Docker** installed and running
- (Optional) **PHP 8.3+** with Apache for running outside Docker

---

## How to Run

### Option 1 — Docker Compose (recommended)

From the repository root (`memory-stress-samples/`):

```bash
docker compose up --build php
```

The service is available at **http://localhost:8085**.

### Option 2 — Standalone Docker

```bash
cd php
docker build -t memory-stress-php .
docker run --rm -p 8085:8080 --memory=1g memory-stress-php
```

### Option 3 — Run locally (with PHP built-in server)

```bash
cd php
php -S 0.0.0.0:8080 index.php
```

---

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Health check — returns `Healthy` |
| GET | `/stress?target=80&rate=5` | Start gradual memory fill (streaming response) |
| GET | `/stop` | Signal the running stress test to stop |
| GET | `/reset` | Free all allocated memory |
| GET | `/memory` | Return current memory snapshot |

### Query Parameters for `/stress`

| Parameter | Default | Range | Meaning |
|-----------|---------|-------|---------|
| `target` | 80 | 1-100 | Target memory usage percentage |
| `rate` | 5 | 1-100 | Percentage of PHP memory limit to add per second |

---

## How to Test

### 1. Health Check

```bash
curl http://localhost:8085/
```

### 2. Check Memory

```bash
curl http://localhost:8085/memory
```

Returns four key metrics:

```json
{
  "timestamp": "2025-06-01T12:00:00.000Z",
  "memory_used_mb": 2.0,
  "memory_available_mb": 2048.0,
  "memory_usage_percent": 0.1,
  "total_app_memory_mb": 2.0
}
```

### 3. Run a Stress Test (streaming)

```bash
curl http://localhost:8085/stress?target=80&rate=5
```

Or open in a browser — the response streams one JSON object per line, per second:

```
{"type":"start","target_percent":80,"rate_percent_per_sec":5,...}
{"type":"tick","tick":1,"elapsed_sec":1.0,"allocated_mb":102,"used_mb":104,"usage_percent":5.1,...}
{"type":"tick","tick":2,"elapsed_sec":2.0,"allocated_mb":204,"used_mb":206,"usage_percent":10.1,...}
...
{"type":"done","status":"Memory stress complete",...}
```

### Example Results (1 GB container)

#### Result: Target 80% memory usage (`?target=80&rate=5`)

The test gradually fills memory to 80% and completes successfully after 16 iterations (~16 seconds):

```
PHP App Memory Stress Test Sample
{"type":"start","target_percent":80,"rate_percent_per_sec":5,"available_mb":1073,"target_mb":858,"chunk_mb":53}
{"type":"tick","tick":1,"elapsed_sec":0,"allocated_mb":53,"used_mb":75,"usage_percent":7,"target_percent":80}
{"type":"tick","tick":2,"elapsed_sec":1.1,"allocated_mb":106,"used_mb":128,"usage_percent":11.9,"target_percent":80}
{"type":"tick","tick":3,"elapsed_sec":2.1,"allocated_mb":159,"used_mb":181,"usage_percent":16.9,"target_percent":80}
{"type":"tick","tick":4,"elapsed_sec":3.1,"allocated_mb":212,"used_mb":235,"usage_percent":21.9,"target_percent":80}
{"type":"tick","tick":5,"elapsed_sec":4.1,"allocated_mb":265,"used_mb":288,"usage_percent":26.8,"target_percent":80}
{"type":"tick","tick":6,"elapsed_sec":5.2,"allocated_mb":318,"used_mb":341,"usage_percent":31.8,"target_percent":80}
{"type":"tick","tick":7,"elapsed_sec":6.2,"allocated_mb":371,"used_mb":394,"usage_percent":36.7,"target_percent":80}
{"type":"tick","tick":8,"elapsed_sec":7.2,"allocated_mb":424,"used_mb":448,"usage_percent":41.7,"target_percent":80}
{"type":"tick","tick":9,"elapsed_sec":8.2,"allocated_mb":477,"used_mb":501,"usage_percent":46.7,"target_percent":80}
{"type":"tick","tick":10,"elapsed_sec":9.3,"allocated_mb":530,"used_mb":554,"usage_percent":51.6,"target_percent":80}
{"type":"tick","tick":11,"elapsed_sec":10.3,"allocated_mb":583,"used_mb":607,"usage_percent":56.6,"target_percent":80}
{"type":"tick","tick":12,"elapsed_sec":11.3,"allocated_mb":636,"used_mb":661,"usage_percent":61.6,"target_percent":80}
{"type":"tick","tick":13,"elapsed_sec":12.3,"allocated_mb":689,"used_mb":714,"usage_percent":66.5,"target_percent":80}
{"type":"tick","tick":14,"elapsed_sec":13.4,"allocated_mb":742,"used_mb":767,"usage_percent":71.5,"target_percent":80}
{"type":"tick","tick":15,"elapsed_sec":14.4,"allocated_mb":795,"used_mb":820,"usage_percent":76.4,"target_percent":80}
{"type":"done","status":"Memory stress complete","target_percent":80,"rate_percent_per_sec":5,"allocated_mb":848,"iterations":16,"elapsed_seconds":15.4,"memory":{"timestamp":"2026-02-25T01:30:20.000Z","memory_used_mb":874.0,"memory_available_mb":1073.0,"memory_usage_percent":81.5,"total_app_memory_mb":874.0}}
```

**Outcome:** Memory grew from ~7% to ~81.5% over 16 iterations and stopped gracefully at the target.

#### Result: Target 100% — OOM (`?target=100&rate=5`)

The test fills memory until usage reaches ~99% of the container limit:

```
PHP App Memory Stress Test Sample
{"type":"start","target_percent":100,"rate_percent_per_sec":5,"available_mb":1073,"target_mb":1073,"chunk_mb":53}
{"type":"tick","tick":1,"elapsed_sec":0,"allocated_mb":53,"used_mb":75,"usage_percent":7,"target_percent":100}
{"type":"tick","tick":2,"elapsed_sec":1.1,"allocated_mb":106,"used_mb":128,"usage_percent":11.9,"target_percent":100}
{"type":"tick","tick":3,"elapsed_sec":2.1,"allocated_mb":159,"used_mb":181,"usage_percent":16.9,"target_percent":100}
{"type":"tick","tick":4,"elapsed_sec":3.1,"allocated_mb":212,"used_mb":235,"usage_percent":21.9,"target_percent":100}
{"type":"tick","tick":5,"elapsed_sec":4.2,"allocated_mb":265,"used_mb":288,"usage_percent":26.8,"target_percent":100}
{"type":"tick","tick":6,"elapsed_sec":5.2,"allocated_mb":318,"used_mb":341,"usage_percent":31.8,"target_percent":100}
{"type":"tick","tick":7,"elapsed_sec":6.2,"allocated_mb":371,"used_mb":394,"usage_percent":36.7,"target_percent":100}
{"type":"tick","tick":8,"elapsed_sec":7.2,"allocated_mb":424,"used_mb":448,"usage_percent":41.7,"target_percent":100}
{"type":"tick","tick":9,"elapsed_sec":8.3,"allocated_mb":477,"used_mb":501,"usage_percent":46.7,"target_percent":100}
{"type":"tick","tick":10,"elapsed_sec":9.3,"allocated_mb":530,"used_mb":554,"usage_percent":51.6,"target_percent":100}
{"type":"tick","tick":11,"elapsed_sec":10.3,"allocated_mb":583,"used_mb":607,"usage_percent":56.6,"target_percent":100}
{"type":"tick","tick":12,"elapsed_sec":11.3,"allocated_mb":636,"used_mb":661,"usage_percent":61.6,"target_percent":100}
{"type":"tick","tick":13,"elapsed_sec":12.4,"allocated_mb":689,"used_mb":714,"usage_percent":66.5,"target_percent":100}
{"type":"tick","tick":14,"elapsed_sec":13.4,"allocated_mb":742,"used_mb":767,"usage_percent":71.5,"target_percent":100}
{"type":"tick","tick":15,"elapsed_sec":14.4,"allocated_mb":795,"used_mb":820,"usage_percent":76.4,"target_percent":100}
{"type":"tick","tick":16,"elapsed_sec":15.4,"allocated_mb":848,"used_mb":874,"usage_percent":81.5,"target_percent":100}
{"type":"tick","tick":17,"elapsed_sec":16.5,"allocated_mb":901,"used_mb":927,"usage_percent":86.4,"target_percent":100}
{"type":"tick","tick":18,"elapsed_sec":17.5,"allocated_mb":954,"used_mb":980,"usage_percent":91.3,"target_percent":100}
{"type":"tick","tick":19,"elapsed_sec":18.5,"allocated_mb":1007,"used_mb":1034,"usage_percent":96.4,"target_percent":100}
{"type":"oom","status":"Memory allocation failed - OOM","error":"Memory usage reached 99.2% of container memory limit (1073 MB)","allocated_mb_before_oom":1060,"iterations_before_oom":20,"elapsed_seconds":19.6,"memory":{"timestamp":"2026-02-25T01:32:45.000Z","memory_used_mb":1064.0,"memory_available_mb":1073.0,"memory_usage_percent":99.2,"total_app_memory_mb":1064.0}}
```

**Outcome:** Memory grew to ~99.2% (1064 MB of 1073 MB available) before the app detected OOM threshold after 20 iterations (~19.6 seconds). The container remained running — the OOM was detected before the kernel OOM killer acted.

**What to watch for:**
- The `usage_percent` field in each `tick` line climbs toward 100%
- An `oom` message appears when memory reaches ≥99% of the container limit
- PHP 8 throws `\Error` on allocation failure, which is caught and reported
- If the container is killed entirely, `docker ps` will show no container (or `Exited` with code 137)

#### Check if the container was OOM-killed:

```bash
docker inspect php-memorystress --format='{{.State.OOMKilled}}'
```

### 4. Stop a Running Test

```bash
curl http://localhost:8085/stop
```

### 5. Reset Memory

```bash
curl http://localhost:8085/reset
```

---

## Response Types

Each streamed JSON line has a `type` field:

| Type | Meaning |
|------|---------|
| `start` | Test started — shows configuration |
| `tick` | One-second update — current memory stats |
| `done` | Target reached successfully |
| `stopped` | Test was stopped via `/stop` |
| `oom` | Memory allocation failed — out of memory |

---

## Notes

- PHP runs per-request under Apache, so the `/stop` endpoint communicates via a temp file (`/tmp/memory_stress_stop`) that the stress loop checks each second.
- `ini_set('memory_limit', '2G')` allows PHP to allocate up to 2 GB. The actual cap depends on the Docker `--memory` flag.
- Output buffering is disabled to enable real-time streaming via `flush()`.
- PHP 8 throws `\Error` (not `\Exception`) on allocation failure, which is caught for OOM reporting.

---

## Project Structure

```
php/
├── index.php     # PHP application (streaming stress logic + router)
├── Dockerfile    # Docker build (php:8.3-apache)
└── README.md     # This file
```
