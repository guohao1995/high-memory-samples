# Memory Stress Test — Node.js (Express)

## Purpose

A Node.js application that **gradually increases memory usage** by a configurable target percentage each second. It streams real-time memory metrics as newline-delimited JSON to the HTTP response, so you can monitor progress in a browser or with `curl`. Designed to help test container memory limits and OOM-kill behaviour.

---

## Prerequisites

- **Docker** installed and running
- (Optional) **Node.js 20+** for running outside Docker

---

## How to Run

### Option 1 — Docker Compose (recommended)

From the repository root (`memory-stress-samples/`):

```bash
docker compose up --build nodejs
```

The service is available at **http://localhost:8082**.

### Option 2 — Standalone Docker

```bash
cd nodejs
docker build -t memory-stress-nodejs .
docker run --rm -p 8082:8080 --memory=1g memory-stress-nodejs
```

### Option 3 — Run locally

```bash
cd nodejs
npm install
node app.js
```

Runs on **http://localhost:8080** by default.

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
| `rate` | 5 | 1-100 | Percentage of estimated memory to add per second |

---

## How to Test

### 1. Health Check

```bash
curl http://localhost:8082/
```

### 2. Check Memory

```bash
curl http://localhost:8082/memory
```

Returns four key metrics:

```json
{
  "timestamp": "2025-06-01T12:00:00.000Z",
  "memory_used_mb": 5.23,
  "memory_available_mb": 8.0,
  "memory_usage_percent": 65.4,
  "total_app_memory_mb": 42.1
}
```

### 3. Run a Stress Test (streaming)

```bash
curl http://localhost:8082/stress?target=80&rate=5
```

Or open in a browser — the response streams one JSON object per line, per second:

```
{"type":"start","target_percent":80,"rate_percent_per_sec":5,...}
{"type":"tick","tick":1,"elapsed_sec":1.0,"allocated_mb":50,"heap_mb":12,"rss_mb":82,"usage_percent":7.6,...}
{"type":"tick","tick":2,"elapsed_sec":2.0,"allocated_mb":100,"heap_mb":12,"rss_mb":132,"usage_percent":12.3,...}
...
{"type":"done","status":"Memory stress complete",...}
```

### Example Results (1 GB container)

#### Result: Target 80% memory usage (`?target=80&rate=5`)

The test gradually fills memory to 80% and completes successfully after 16 iterations (~16 seconds):

```
Node.js App Memory Stress Test Sample
{"type":"start","target_percent":80,"rate_percent_per_sec":5,"available_mb":1073,"target_mb":858,"chunk_mb":53}
{"type":"tick","tick":1,"elapsed_sec":0,"allocated_mb":53,"heap_mb":4,"rss_mb":85,"usage_percent":7.9,"target_percent":80}
{"type":"tick","tick":2,"elapsed_sec":1,"allocated_mb":106,"heap_mb":4,"rss_mb":138,"usage_percent":12.9,"target_percent":80}
{"type":"tick","tick":3,"elapsed_sec":2.1,"allocated_mb":159,"heap_mb":4,"rss_mb":192,"usage_percent":17.9,"target_percent":80}
{"type":"tick","tick":4,"elapsed_sec":3.1,"allocated_mb":212,"heap_mb":4,"rss_mb":245,"usage_percent":22.8,"target_percent":80}
{"type":"tick","tick":5,"elapsed_sec":4.1,"allocated_mb":265,"heap_mb":4,"rss_mb":298,"usage_percent":27.8,"target_percent":80}
{"type":"tick","tick":6,"elapsed_sec":5.1,"allocated_mb":318,"heap_mb":4,"rss_mb":351,"usage_percent":32.7,"target_percent":80}
{"type":"tick","tick":7,"elapsed_sec":6.1,"allocated_mb":371,"heap_mb":4,"rss_mb":404,"usage_percent":37.7,"target_percent":80}
{"type":"tick","tick":8,"elapsed_sec":7.2,"allocated_mb":424,"heap_mb":4,"rss_mb":458,"usage_percent":42.7,"target_percent":80}
{"type":"tick","tick":9,"elapsed_sec":8.2,"allocated_mb":477,"heap_mb":4,"rss_mb":511,"usage_percent":47.6,"target_percent":80}
{"type":"tick","tick":10,"elapsed_sec":9.2,"allocated_mb":530,"heap_mb":4,"rss_mb":564,"usage_percent":52.6,"target_percent":80}
{"type":"tick","tick":11,"elapsed_sec":10.2,"allocated_mb":583,"heap_mb":4,"rss_mb":617,"usage_percent":57.5,"target_percent":80}
{"type":"tick","tick":12,"elapsed_sec":11.2,"allocated_mb":636,"heap_mb":4,"rss_mb":670,"usage_percent":62.5,"target_percent":80}
{"type":"tick","tick":13,"elapsed_sec":12.3,"allocated_mb":689,"heap_mb":4,"rss_mb":724,"usage_percent":67.5,"target_percent":80}
{"type":"tick","tick":14,"elapsed_sec":13.3,"allocated_mb":742,"heap_mb":4,"rss_mb":777,"usage_percent":72.4,"target_percent":80}
{"type":"tick","tick":15,"elapsed_sec":14.3,"allocated_mb":795,"heap_mb":4,"rss_mb":830,"usage_percent":77.4,"target_percent":80}
{"type":"done","status":"Memory stress complete","target_percent":80,"rate_percent_per_sec":5,"allocated_mb":848,"iterations":16,"elapsed_seconds":15.3,"memory":{"timestamp":"2026-02-25T01:20:15.000Z","memory_used_mb":883.0,"memory_available_mb":1073.0,"memory_usage_percent":82.3,"total_app_memory_mb":883.0}}
```

**Outcome:** Memory (RSS) grew from ~8% to ~82% over 16 iterations and stopped gracefully at the target. Note that Node.js `heap_mb` stays low because `Buffer.alloc()` allocates memory outside the V8 heap.

#### Result: Target 100% — OOM (`?target=100&rate=5`)

The test fills memory until RSS reaches ~99% of the container limit:

```
Node.js App Memory Stress Test Sample
{"type":"start","target_percent":100,"rate_percent_per_sec":5,"available_mb":1073,"target_mb":1073,"chunk_mb":53}
{"type":"tick","tick":1,"elapsed_sec":0,"allocated_mb":53,"heap_mb":4,"rss_mb":85,"usage_percent":7.9,"target_percent":100}
{"type":"tick","tick":2,"elapsed_sec":1.1,"allocated_mb":106,"heap_mb":4,"rss_mb":138,"usage_percent":12.9,"target_percent":100}
{"type":"tick","tick":3,"elapsed_sec":2.1,"allocated_mb":159,"heap_mb":4,"rss_mb":192,"usage_percent":17.9,"target_percent":100}
{"type":"tick","tick":4,"elapsed_sec":3.1,"allocated_mb":212,"heap_mb":4,"rss_mb":245,"usage_percent":22.8,"target_percent":100}
{"type":"tick","tick":5,"elapsed_sec":4.1,"allocated_mb":265,"heap_mb":4,"rss_mb":298,"usage_percent":27.8,"target_percent":100}
{"type":"tick","tick":6,"elapsed_sec":5.2,"allocated_mb":318,"heap_mb":4,"rss_mb":351,"usage_percent":32.7,"target_percent":100}
{"type":"tick","tick":7,"elapsed_sec":6.2,"allocated_mb":371,"heap_mb":4,"rss_mb":404,"usage_percent":37.7,"target_percent":100}
{"type":"tick","tick":8,"elapsed_sec":7.2,"allocated_mb":424,"heap_mb":4,"rss_mb":458,"usage_percent":42.7,"target_percent":100}
{"type":"tick","tick":9,"elapsed_sec":8.2,"allocated_mb":477,"heap_mb":4,"rss_mb":511,"usage_percent":47.6,"target_percent":100}
{"type":"tick","tick":10,"elapsed_sec":9.3,"allocated_mb":530,"heap_mb":4,"rss_mb":564,"usage_percent":52.6,"target_percent":100}
{"type":"tick","tick":11,"elapsed_sec":10.3,"allocated_mb":583,"heap_mb":4,"rss_mb":617,"usage_percent":57.5,"target_percent":100}
{"type":"tick","tick":12,"elapsed_sec":11.3,"allocated_mb":636,"heap_mb":4,"rss_mb":670,"usage_percent":62.5,"target_percent":100}
{"type":"tick","tick":13,"elapsed_sec":12.3,"allocated_mb":689,"heap_mb":4,"rss_mb":724,"usage_percent":67.5,"target_percent":100}
{"type":"tick","tick":14,"elapsed_sec":13.4,"allocated_mb":742,"heap_mb":4,"rss_mb":777,"usage_percent":72.4,"target_percent":100}
{"type":"tick","tick":15,"elapsed_sec":14.4,"allocated_mb":795,"heap_mb":4,"rss_mb":830,"usage_percent":77.4,"target_percent":100}
{"type":"tick","tick":16,"elapsed_sec":15.4,"allocated_mb":848,"heap_mb":4,"rss_mb":883,"usage_percent":82.3,"target_percent":100}
{"type":"tick","tick":17,"elapsed_sec":16.4,"allocated_mb":901,"heap_mb":4,"rss_mb":937,"usage_percent":87.3,"target_percent":100}
{"type":"tick","tick":18,"elapsed_sec":17.5,"allocated_mb":954,"heap_mb":4,"rss_mb":990,"usage_percent":92.3,"target_percent":100}
{"type":"tick","tick":19,"elapsed_sec":18.5,"allocated_mb":1007,"heap_mb":4,"rss_mb":1043,"usage_percent":97.2,"target_percent":100}
{"type":"oom","status":"Memory allocation failed - OOM","error":"RSS reached 99.5% of container memory limit (1073 MB)","allocated_mb_before_oom":1060,"iterations_before_oom":20,"elapsed_seconds":19.5,"rss_mb":1067,"usage_percent":99.5,"memory":{"timestamp":"2026-02-25T01:22:30.000Z","memory_used_mb":1067.0,"memory_available_mb":1073.0,"memory_usage_percent":99.5,"total_app_memory_mb":1067.0}}
```

**Outcome:** Memory grew to ~99.5% (1067 MB of 1073 MB available) before the app detected OOM threshold after 20 iterations (~19.5 seconds). The container remained running — the OOM was detected before the kernel OOM killer acted.

**What to watch for:**
- The `usage_percent` field in each `tick` line climbs toward 100%
- An `oom` message appears when RSS reaches ≥99% of the container limit
- If the container is killed entirely, `docker ps` will show no container (or `Exited` with code 137)

#### Check if the container was OOM-killed:

```bash
docker inspect nodejs-memorystress --format='{{.State.OOMKilled}}'
```

### 4. Stop a Running Test

```bash
curl http://localhost:8082/stop
```

### 5. Reset Memory

```bash
curl http://localhost:8082/reset
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

## Project Structure

```
nodejs/
├── app.js           # Express application (streaming stress logic)
├── Dockerfile       # Multi-stage Docker build (node:20-slim)
├── package.json     # Dependencies (express)
└── README.md        # This file
```
