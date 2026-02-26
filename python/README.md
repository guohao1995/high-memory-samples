# Memory Stress Test — Python (Flask + Gunicorn)

## Purpose

A Python application that **gradually increases memory usage** by a configurable target percentage each second. It streams real-time memory metrics as newline-delimited JSON to the HTTP response, so you can monitor progress in a browser or with `curl`. Designed to help test container memory limits and OOM-kill behaviour.

---

## Prerequisites

- **Docker** installed and running
- (Optional) **Python 3.12+** for running outside Docker

---

## How to Run

### Option 1 — Docker Compose (recommended)

From the repository root (`memory-stress-samples/`):

```bash
docker compose up --build python
```

The service is available at **http://localhost:8081**.

### Option 2 — Standalone Docker

```bash
cd python
docker build -t memory-stress-python .
docker run --rm -p 8081:8080 --memory=1g memory-stress-python
```

### Option 3 — Run locally

```bash
cd python
pip install -r requirements.txt
python app.py
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
| `rate` | 5 | 1-100 | Percentage of available memory to add per second |

---

## How to Test

### 1. Health Check

```bash
curl http://localhost:8081/
```

### 2. Check Memory

```bash
curl http://localhost:8081/memory
```

Returns four key metrics:

```json
{
  "timestamp": "2025-06-01T12:00:00.000Z",
  "memory_used_mb": 25.4,
  "memory_available_mb": 120.0,
  "memory_usage_percent": 21.2,
  "total_app_memory_mb": 25.4
}
```

### 3. Run a Stress Test (streaming)

```bash
curl http://localhost:8081/stress?target=80&rate=5
```

Or open in a browser — the response streams one JSON object per line, per second:

```
{"type":"start","target_percent":80,"rate_percent_per_sec":5,...}
{"type":"tick","tick":1,"elapsed_sec":1.0,"allocated_mb":50,"rss_mb":82,"usage_percent":7.6,...}
{"type":"tick","tick":2,"elapsed_sec":2.0,"allocated_mb":100,"rss_mb":132,"usage_percent":12.3,...}
...
{"type":"done","status":"Memory stress complete",...}
```

### Example Results (1 GB container)

#### Result: Target 80% memory usage (`?target=80&rate=5`)

The test gradually fills memory to 80% and completes successfully after 16 iterations (~16 seconds):

```
Python App Memory Stress Test Sample
{"type":"start","target_percent":80,"rate_percent_per_sec":5,"available_mb":1073,"target_mb":858,"chunk_mb":53}
{"type":"tick","tick":1,"elapsed_sec":0.1,"allocated_mb":53,"rss_mb":78,"usage_percent":7.3,"target_percent":80}
{"type":"tick","tick":2,"elapsed_sec":1.1,"allocated_mb":106,"rss_mb":131,"usage_percent":12.2,"target_percent":80}
{"type":"tick","tick":3,"elapsed_sec":2.1,"allocated_mb":159,"rss_mb":184,"usage_percent":17.2,"target_percent":80}
{"type":"tick","tick":4,"elapsed_sec":3.1,"allocated_mb":212,"rss_mb":238,"usage_percent":22.2,"target_percent":80}
{"type":"tick","tick":5,"elapsed_sec":4.2,"allocated_mb":265,"rss_mb":291,"usage_percent":27.1,"target_percent":80}
{"type":"tick","tick":6,"elapsed_sec":5.2,"allocated_mb":318,"rss_mb":344,"usage_percent":32.1,"target_percent":80}
{"type":"tick","tick":7,"elapsed_sec":6.2,"allocated_mb":371,"rss_mb":397,"usage_percent":37,"target_percent":80}
{"type":"tick","tick":8,"elapsed_sec":7.2,"allocated_mb":424,"rss_mb":451,"usage_percent":42,"target_percent":80}
{"type":"tick","tick":9,"elapsed_sec":8.3,"allocated_mb":477,"rss_mb":504,"usage_percent":47,"target_percent":80}
{"type":"tick","tick":10,"elapsed_sec":9.3,"allocated_mb":530,"rss_mb":557,"usage_percent":51.9,"target_percent":80}
{"type":"tick","tick":11,"elapsed_sec":10.3,"allocated_mb":583,"rss_mb":610,"usage_percent":56.9,"target_percent":80}
{"type":"tick","tick":12,"elapsed_sec":11.3,"allocated_mb":636,"rss_mb":664,"usage_percent":61.9,"target_percent":80}
{"type":"tick","tick":13,"elapsed_sec":12.4,"allocated_mb":689,"rss_mb":717,"usage_percent":66.8,"target_percent":80}
{"type":"tick","tick":14,"elapsed_sec":13.4,"allocated_mb":742,"rss_mb":770,"usage_percent":71.8,"target_percent":80}
{"type":"tick","tick":15,"elapsed_sec":14.4,"allocated_mb":795,"rss_mb":823,"usage_percent":76.7,"target_percent":80}
{"type":"done","status":"Memory stress complete","target_percent":80,"rate_percent_per_sec":5,"allocated_mb":848,"iterations":16,"elapsed_seconds":15.4,"memory":{"timestamp":"2026-02-25T01:25:10.000Z","memory_used_mb":877.0,"memory_available_mb":1073.0,"memory_usage_percent":81.7,"total_app_memory_mb":877.0}}
```

**Outcome:** Memory (RSS) grew from ~7% to ~82% over 16 iterations and stopped gracefully at the target.

#### Result: Target 100% — OOM (`?target=100&rate=5`)

The test fills memory until RSS reaches ~99% of the container limit:

```
Python App Memory Stress Test Sample
{"type":"start","target_percent":100,"rate_percent_per_sec":5,"available_mb":1073,"target_mb":1073,"chunk_mb":53}
{"type":"tick","tick":1,"elapsed_sec":0.1,"allocated_mb":53,"rss_mb":78,"usage_percent":7.3,"target_percent":100}
{"type":"tick","tick":2,"elapsed_sec":1.1,"allocated_mb":106,"rss_mb":131,"usage_percent":12.2,"target_percent":100}
{"type":"tick","tick":3,"elapsed_sec":2.1,"allocated_mb":159,"rss_mb":184,"usage_percent":17.2,"target_percent":100}
{"type":"tick","tick":4,"elapsed_sec":3.1,"allocated_mb":212,"rss_mb":238,"usage_percent":22.2,"target_percent":100}
{"type":"tick","tick":5,"elapsed_sec":4.2,"allocated_mb":265,"rss_mb":291,"usage_percent":27.1,"target_percent":100}
{"type":"tick","tick":6,"elapsed_sec":5.2,"allocated_mb":318,"rss_mb":344,"usage_percent":32.1,"target_percent":100}
{"type":"tick","tick":7,"elapsed_sec":6.2,"allocated_mb":371,"rss_mb":397,"usage_percent":37,"target_percent":100}
{"type":"tick","tick":8,"elapsed_sec":7.3,"allocated_mb":424,"rss_mb":451,"usage_percent":42,"target_percent":100}
{"type":"tick","tick":9,"elapsed_sec":8.3,"allocated_mb":477,"rss_mb":504,"usage_percent":47,"target_percent":100}
{"type":"tick","tick":10,"elapsed_sec":9.3,"allocated_mb":530,"rss_mb":557,"usage_percent":51.9,"target_percent":100}
{"type":"tick","tick":11,"elapsed_sec":10.3,"allocated_mb":583,"rss_mb":610,"usage_percent":56.9,"target_percent":100}
{"type":"tick","tick":12,"elapsed_sec":11.4,"allocated_mb":636,"rss_mb":664,"usage_percent":61.9,"target_percent":100}
{"type":"tick","tick":13,"elapsed_sec":12.4,"allocated_mb":689,"rss_mb":717,"usage_percent":66.8,"target_percent":100}
{"type":"tick","tick":14,"elapsed_sec":13.4,"allocated_mb":742,"rss_mb":770,"usage_percent":71.8,"target_percent":100}
{"type":"tick","tick":15,"elapsed_sec":14.4,"allocated_mb":795,"rss_mb":823,"usage_percent":76.7,"target_percent":100}
{"type":"tick","tick":16,"elapsed_sec":15.5,"allocated_mb":848,"rss_mb":877,"usage_percent":81.7,"target_percent":100}
{"type":"tick","tick":17,"elapsed_sec":16.5,"allocated_mb":901,"rss_mb":930,"usage_percent":86.7,"target_percent":100}
{"type":"tick","tick":18,"elapsed_sec":17.5,"allocated_mb":954,"rss_mb":983,"usage_percent":91.6,"target_percent":100}
{"type":"tick","tick":19,"elapsed_sec":18.5,"allocated_mb":1007,"rss_mb":1037,"usage_percent":96.6,"target_percent":100}
{"type":"oom","status":"Memory allocation failed - OOM","error":"RSS reached 99.4% of container memory limit (1073 MB)","allocated_mb_before_oom":1060,"iterations_before_oom":20,"elapsed_seconds":19.6,"rss_mb":1066,"usage_percent":99.4,"memory":{"timestamp":"2026-02-25T01:27:30.000Z","memory_used_mb":1066.0,"memory_available_mb":1073.0,"memory_usage_percent":99.4,"total_app_memory_mb":1066.0}}
```

**Outcome:** Memory grew to ~99.4% (1066 MB of 1073 MB available) before the app detected OOM threshold after 20 iterations (~19.6 seconds). The container remained running — the OOM was detected before the kernel OOM killer acted.

**What to watch for:**
- The `usage_percent` field in each `tick` line climbs toward 100%
- An `oom` message appears when RSS reaches ≥99% of the container limit
- Python raises `MemoryError` on allocation failure, which is caught and reported
- If the container is killed entirely, `docker ps` will show no container (or `Exited` with code 137)

#### Check if the container was OOM-killed:

```bash
docker inspect python-memorystress --format='{{.State.OOMKilled}}'
```

### 4. Stop a Running Test

```bash
curl http://localhost:8081/stop
```

### 5. Reset Memory

```bash
curl http://localhost:8081/reset
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

- When running behind **Gunicorn** (as in the Docker image), the default worker type is synchronous. The streaming response blocks the worker for the duration of the test.
- `/proc/self/status` is used for accurate RSS readings inside containers. Falls back to `psutil` on non-Linux systems.
- Memory reads cgroup limits (`/sys/fs/cgroup/memory/memory.limit_in_bytes` or `memory.max`) to determine the container memory cap.

---

## Project Structure

```
python/
├── app.py             # Flask application (streaming stress logic)
├── Dockerfile         # Docker build (python:3.12-slim + gunicorn)
├── requirements.txt   # Dependencies (Flask, gunicorn)
└── README.md          # This file
```
