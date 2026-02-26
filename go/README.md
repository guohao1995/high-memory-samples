# Go Memory Stress Test Application

## Purpose

This application simulates **gradual memory growth** in a Go container, making it easy to:

- **Reproduce memory pressure** scenarios (e.g., memory leaks) in a controlled way
- **Observe how containers behave** when approaching their memory limit
- **Trigger Out-of-Memory (OOM) kills** to test alerting, monitoring, and recovery
- **Validate container memory limits** set via Docker, Kubernetes, or Azure Container Apps

The app allocates large byte slices at a configurable rate (% per second) until a target usage percentage is reached — or the container is killed by the OOM killer.

---

## Prerequisites

- [Go 1.22+](https://go.dev/dl/) (for local runs)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (for container runs)

---

## How to Run

### Option 1: Run Locally (without Docker)

```bash
cd go
go run main.go
```

The app starts on **http://localhost:8080**.

### Option 2: Run as a Docker Container

**Build the image:**

```bash
cd memory-stress-samples
docker compose build go
```

**Run with a specific memory limit** (e.g., 1 GB):

```bash
docker run -d --name go-memorystress --memory=1g -p 8086:8080 memory-stress-samples-go
```

The app is accessible at **http://localhost:8086**.

**Stop and remove the container:**

```bash
docker rm -f go-memorystress
```

### Option 3: Run via Docker Compose

```bash
cd memory-stress-samples
docker compose up go --build -d
```

This uses the default 2 GB memory limit defined in `docker-compose.yml` and maps to port **8086**.

---

## API Endpoints

| Endpoint | Method | Description |
|-----------|--------|-------------|
| `/`       | GET    | Health check — returns `"Healthy"` |
| `/stress` | GET    | Start gradual memory allocation (streaming output) |
| `/stop`   | GET    | Stop a running stress test |
| `/reset`  | GET    | Free all allocated memory and force garbage collection |
| `/memory` | GET    | Get current memory usage snapshot |

---

## How to Test Each Feature

### 1. Health Check — `GET /`

```bash
curl http://localhost:8086/
```

**Expected output:** `Healthy`

---

### 2. Check Memory Usage — `GET /memory`

```bash
curl http://localhost:8086/memory
```

**Expected output:**

```json
{
  "timestamp": "2026-02-24T12:00:00.000Z",
  "memory_used_mb": 1.5,
  "memory_available_mb": 1024.0,
  "memory_usage_percent": 0.1,
  "total_app_memory_mb": 8.2
}
```

| Field | Meaning |
|-------|---------|
| `memory_used_mb` | Memory currently allocated (heap) |
| `memory_available_mb` | Total memory obtained from the OS |
| `memory_usage_percent` | Used as a percentage of available |
| `total_app_memory_mb` | Total system memory held by the Go runtime |

---

### 3. Gradual Memory Stress — `GET /stress`

Allocates memory **gradually (once per second)**, streaming real-time JSON lines.

**Parameters:**

| Param    | Default | Range   | Description |
|----------|---------|---------|-------------|
| `target` | 80      | 1–100   | Target memory usage percentage |
| `rate`   | 5       | 1–100   | % of available memory to add per second |

#### Example: Fill to 80% at 5%/sec

```bash
curl -N http://localhost:8086/stress?target=80&rate=5
```

**Streamed output (one JSON line per second):**

```
{"type":"start","target_percent":80,"rate_percent_per_sec":5,"available_mb":768,"target_mb":614,"chunk_mb":38}
{"type":"tick","tick":1,"elapsed_sec":0,"allocated_mb":38,"heap_mb":39,"sys_mb":618,"usage_percent":5.1,"target_percent":80}
{"type":"tick","tick":2,"elapsed_sec":1,"allocated_mb":76,"heap_mb":77,"sys_mb":657,"usage_percent":10,"target_percent":80}
...
{"type":"done","status":"Memory stress complete","allocated_mb":646,"iterations":18,"elapsed_seconds":17.4,"memory":{...}}
```

#### Example: Push to OOM (target 100%)

```bash
curl -N http://localhost:8086/stress?target=100&rate=10
```

---

### Example Results (1 GB container)

#### Result: Target 80% memory usage (`?target=80&rate=5`)

The test gradually fills memory to 80% and completes successfully after 16 iterations (~16 seconds):

```
Go App Memory Stress Test Sample
{"type":"start","target_percent":80,"rate_percent_per_sec":5,"available_mb":1073,"target_mb":858,"chunk_mb":53}
{"type":"tick","tick":1,"elapsed_sec":0,"allocated_mb":53,"rss_mb":64,"usage_percent":6,"target_percent":80}
{"type":"tick","tick":2,"elapsed_sec":1,"allocated_mb":106,"rss_mb":117,"usage_percent":10.9,"target_percent":80}
{"type":"tick","tick":3,"elapsed_sec":2.1,"allocated_mb":159,"rss_mb":170,"usage_percent":15.9,"target_percent":80}
{"type":"tick","tick":4,"elapsed_sec":3.1,"allocated_mb":212,"rss_mb":224,"usage_percent":20.9,"target_percent":80}
{"type":"tick","tick":5,"elapsed_sec":4.1,"allocated_mb":265,"rss_mb":277,"usage_percent":25.8,"target_percent":80}
{"type":"tick","tick":6,"elapsed_sec":5.1,"allocated_mb":318,"rss_mb":330,"usage_percent":30.8,"target_percent":80}
{"type":"tick","tick":7,"elapsed_sec":6.1,"allocated_mb":371,"rss_mb":383,"usage_percent":35.7,"target_percent":80}
{"type":"tick","tick":8,"elapsed_sec":7.1,"allocated_mb":424,"rss_mb":436,"usage_percent":40.7,"target_percent":80}
{"type":"tick","tick":9,"elapsed_sec":8.2,"allocated_mb":477,"rss_mb":490,"usage_percent":45.7,"target_percent":80}
{"type":"tick","tick":10,"elapsed_sec":9.2,"allocated_mb":530,"rss_mb":543,"usage_percent":50.6,"target_percent":80}
{"type":"tick","tick":11,"elapsed_sec":10.2,"allocated_mb":583,"rss_mb":596,"usage_percent":55.5,"target_percent":80}
{"type":"tick","tick":12,"elapsed_sec":11.2,"allocated_mb":636,"rss_mb":649,"usage_percent":60.5,"target_percent":80}
{"type":"tick","tick":13,"elapsed_sec":12.2,"allocated_mb":689,"rss_mb":702,"usage_percent":65.4,"target_percent":80}
{"type":"tick","tick":14,"elapsed_sec":13.3,"allocated_mb":742,"rss_mb":756,"usage_percent":70.4,"target_percent":80}
{"type":"tick","tick":15,"elapsed_sec":14.3,"allocated_mb":795,"rss_mb":809,"usage_percent":75.4,"target_percent":80}
{"type":"done","status":"Memory stress complete","target_percent":80,"rate_percent_per_sec":5,"allocated_mb":848,"iterations":16,"elapsed_seconds":15.3,"memory":{"timestamp":"2026-02-25T01:15:30.000Z","memory_used_mb":862.0,"memory_available_mb":1073.0,"memory_usage_percent":80.3,"total_app_memory_mb":870.5}}
```

**Outcome:** Memory grew from ~6% to ~80% over 16 seconds and stopped gracefully at the target.

#### Result: Target 100% — OOM (`?target=100&rate=5`)

The test fills memory until RSS reaches ~99% of the container limit:

```
Go App Memory Stress Test Sample
{"type":"start","target_percent":100,"rate_percent_per_sec":5,"available_mb":1073,"target_mb":1073,"chunk_mb":53}
{"type":"tick","tick":1,"elapsed_sec":0,"allocated_mb":53,"rss_mb":64,"usage_percent":6,"target_percent":100}
{"type":"tick","tick":2,"elapsed_sec":1,"allocated_mb":106,"rss_mb":117,"usage_percent":10.9,"target_percent":100}
{"type":"tick","tick":3,"elapsed_sec":2.1,"allocated_mb":159,"rss_mb":170,"usage_percent":15.9,"target_percent":100}
{"type":"tick","tick":4,"elapsed_sec":3.1,"allocated_mb":212,"rss_mb":224,"usage_percent":20.9,"target_percent":100}
{"type":"tick","tick":5,"elapsed_sec":4.1,"allocated_mb":265,"rss_mb":277,"usage_percent":25.8,"target_percent":100}
{"type":"tick","tick":6,"elapsed_sec":5.2,"allocated_mb":318,"rss_mb":330,"usage_percent":30.8,"target_percent":100}
{"type":"tick","tick":7,"elapsed_sec":6.2,"allocated_mb":371,"rss_mb":383,"usage_percent":35.7,"target_percent":100}
{"type":"tick","tick":8,"elapsed_sec":7.2,"allocated_mb":424,"rss_mb":436,"usage_percent":40.7,"target_percent":100}
{"type":"tick","tick":9,"elapsed_sec":8.2,"allocated_mb":477,"rss_mb":490,"usage_percent":45.7,"target_percent":100}
{"type":"tick","tick":10,"elapsed_sec":9.3,"allocated_mb":530,"rss_mb":543,"usage_percent":50.6,"target_percent":100}
{"type":"tick","tick":11,"elapsed_sec":10.3,"allocated_mb":583,"rss_mb":596,"usage_percent":55.5,"target_percent":100}
{"type":"tick","tick":12,"elapsed_sec":11.3,"allocated_mb":636,"rss_mb":649,"usage_percent":60.5,"target_percent":100}
{"type":"tick","tick":13,"elapsed_sec":12.3,"allocated_mb":689,"rss_mb":702,"usage_percent":65.4,"target_percent":100}
{"type":"tick","tick":14,"elapsed_sec":13.4,"allocated_mb":742,"rss_mb":756,"usage_percent":70.4,"target_percent":100}
{"type":"tick","tick":15,"elapsed_sec":14.4,"allocated_mb":795,"rss_mb":809,"usage_percent":75.4,"target_percent":100}
{"type":"tick","tick":16,"elapsed_sec":15.4,"allocated_mb":848,"rss_mb":862,"usage_percent":80.3,"target_percent":100}
{"type":"tick","tick":17,"elapsed_sec":16.4,"allocated_mb":901,"rss_mb":915,"usage_percent":85.3,"target_percent":100}
{"type":"tick","tick":18,"elapsed_sec":17.5,"allocated_mb":954,"rss_mb":968,"usage_percent":90.2,"target_percent":100}
{"type":"tick","tick":19,"elapsed_sec":18.5,"allocated_mb":1007,"rss_mb":1022,"usage_percent":95.2,"target_percent":100}
{"type":"oom","status":"Memory allocation failed - OOM","error":"RSS reached 99.3% of container memory limit (1073 MB)","allocated_mb_before_oom":1060,"iterations_before_oom":20,"elapsed_seconds":19.5,"rss_mb":1065,"usage_percent":99.3,"memory":{"timestamp":"2026-02-25T01:18:45.000Z","memory_used_mb":1065.0,"memory_available_mb":1073.0,"memory_usage_percent":99.3,"total_app_memory_mb":1070.2}}
```

**Outcome:** Memory grew to ~99.3% (1065 MB of 1073 MB available) before the app detected OOM threshold after 20 iterations (~19.5 seconds). The container remained running — the OOM was detected and reported before the kernel OOM killer acted.

**What to watch for:**
- The `usage_percent` field in each `tick` line climbs toward 100%
- An `oom` message appears when RSS reaches ≥99% of the container limit
- If the container is killed entirely, `docker ps` will show no container (or `Exited` with code 137)

#### Check if the container was OOM-killed:

```bash
docker inspect go-memorystress --format='{{.State.OOMKilled}}'
```

---

### 4. Stop a Running Test — `GET /stop`

```bash
curl http://localhost:8086/stop
```

**Expected output:** `{"status": "Stop signal sent"}`

---

### 5. Reset Memory — `GET /reset`

```bash
curl http://localhost:8086/reset
```

**Expected output:**

```json
{"status": "Memory cleared", "memory": {"memory_used_mb": 1.2, ...}}
```

---

## Full Test Walkthrough

```bash
# 1. Start container with 1 GB memory
docker run -d --name go-memorystress --memory=1g -p 8086:8080 memory-stress-samples-go

# 2. Verify it's running
curl http://localhost:8086/

# 3. Check baseline memory
curl http://localhost:8086/memory

# 4. Start gradual stress to 60%
curl -N http://localhost:8086/stress?target=60&rate=5

# 5. Reset memory
curl http://localhost:8086/reset

# 6. Run stress to 100% to trigger OOM
curl -N http://localhost:8086/stress?target=100&rate=10

# 7. Check if container was OOM-killed
docker inspect go-memorystress --format='{{.State.OOMKilled}}'

# 8. Clean up
docker rm -f go-memorystress
```

---

## Monitoring via Docker

```bash
docker stats go-memorystress
docker logs -f go-memorystress
```

---

## Project Structure

```
go/
├── main.go      # Application code (all endpoints)
├── go.mod       # Go module file
├── Dockerfile   # Multi-stage Docker build
└── README.md    # This file
```
