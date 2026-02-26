# .NET Memory Stress Test Application

## Purpose

This application is designed to **simulate gradual memory growth** in a .NET container, making it easy to:

- **Reproduce memory pressure** scenarios (e.g., memory leaks) in a controlled way
- **Observe how containers behave** when approaching their memory limit
- **Trigger Out-of-Memory (OOM) kills** to test alerting, monitoring, and recovery
- **Validate container memory limits** set via Docker, Kubernetes, or Azure Container Apps

The app allocates large byte arrays on the .NET Large Object Heap (LOH) at a configurable rate (% per second) until a target usage percentage is reached — or the container is killed by the OOM killer.

---

## Prerequisites

- [.NET 8 SDK](https://dotnet.microsoft.com/download/dotnet/8.0) (for local runs)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (for container runs)

---

## How to Run

### Option 1: Run Locally (without Docker)

```bash
cd dotnet
dotnet run
```

The app starts on **http://localhost:8080**.

### Option 2: Run as a Docker Container

**Build the image:**

```bash
cd memory-stress-samples
docker compose build dotnet
```

**Run with a specific memory limit** (e.g., 1 GB):

```bash
docker run -d --name dotnet-memorystress --memory=1g -p 8083:8080 memory-stress-samples-dotnet
```

The app is accessible at **http://localhost:8083**.

**Stop and remove the container:**

```bash
docker rm -f dotnet-memorystress
```

### Option 3: Run via Docker Compose

```bash
cd memory-stress-samples
docker compose up dotnet --build -d
```

This uses the default 2 GB memory limit defined in `docker-compose.yml` and maps to port **8083**.

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

Verifies the app is running and responding.

```bash
curl http://localhost:8083/
```

**Expected output:**

```
Healthy
```

---

### 2. Check Memory Usage — `GET /memory`

Returns the current memory state of the application.

```bash
curl http://localhost:8083/memory
```

**Expected output:**

```json
{
  "timestamp": "2026-02-24T12:00:00.0000000Z",
  "memory_used_mb": 5.23,
  "memory_available_mb": 1024.0,
  "memory_usage_percent": 0.5,
  "total_app_memory_mb": 30.15
}
```

**Fields explained:**

| Field | Meaning |
|-------|---------|
| `memory_used_mb` | Memory currently used by the app (GC heap) |
| `memory_available_mb` | Total memory available to the app (reflects container limit) |
| `memory_usage_percent` | Used memory as a percentage of available |
| `total_app_memory_mb` | Total physical memory footprint of the process (working set) |

---

### 3. Gradual Memory Stress — `GET /stress`

Starts allocating memory **gradually (once per second)**, streaming real-time usage to the HTTP response as NDJSON (newline-delimited JSON).

**Parameters:**

| Param    | Default | Range   | Description |
|----------|---------|---------|-------------|
| `target` | 80      | 1–100   | Target memory usage percentage to reach |
| `rate`   | 5       | 1–100   | Percentage of available memory to add per second |

#### Example: Fill to 50% at 5%/sec

```bash
curl -N http://localhost:8083/stress?target=50&rate=5
```

> **Note:** Use `curl -N` (or `--no-buffer`) to see streaming output in real-time.

**Streamed output (one JSON line per second):**

```jsonl
{"type":"start","target_percent":50,"rate_percent_per_sec":5,"available_mb":1024,"target_mb":512,"chunk_mb":51}
{"type":"tick","tick":1,"elapsed_sec":0.0,"allocated_mb":51,"heap_mb":53,"working_set_mb":85,"usage_percent":5.2,"target_percent":50}
{"type":"tick","tick":2,"elapsed_sec":1.0,"allocated_mb":102,"heap_mb":104,"working_set_mb":136,"usage_percent":10.2,"target_percent":50}
...
{"type":"done","status":"Memory stress complete","target_percent":50,"allocated_mb":510,"iterations":10,"elapsed_seconds":10.0,"memory":{...}}
```

**Stream message types:**

| Type | When | Meaning |
|------|------|---------|
| `start` | First line | Configuration summary (available MB, target, chunk size) |
| `tick` | Every second | Current allocation progress and memory metrics |
| `done` | End | Stress completed successfully — target reached |
| `oom` | End | Out-of-Memory exception caught before container kill |
| `stopped` | End | Test was stopped via `/stop` |

#### Example: Push to OOM (target 100%)

```bash
curl -N http://localhost:8083/stress?target=100&rate=10
```

With a 1 GB container limit, this will allocate ~100 MB/sec until the container is killed by the OOM killer or the .NET runtime catches `OutOfMemoryException`.

---

### Example Results (1 GB container)

#### Result: Target 80% memory usage (`?target=80&rate=5`)

The test gradually fills memory to 80% and completes successfully after 18 iterations (~17 seconds):

```
{"type":"start","target_percent":80,"rate_percent_per_sec":5,"available_mb":768,"target_mb":614,"chunk_mb":38}
{"type":"tick","tick":1,"elapsed_sec":0,"allocated_mb":38,"heap_mb":39,"working_set_mb":618,"usage_percent":5.1,"target_percent":80}
{"type":"tick","tick":2,"elapsed_sec":1,"allocated_mb":76,"heap_mb":77,"working_set_mb":657,"usage_percent":10,"target_percent":80}
{"type":"tick","tick":3,"elapsed_sec":2.1,"allocated_mb":114,"heap_mb":115,"working_set_mb":695,"usage_percent":15,"target_percent":80}
{"type":"tick","tick":4,"elapsed_sec":3.1,"allocated_mb":152,"heap_mb":153,"working_set_mb":695,"usage_percent":19.9,"target_percent":80}
{"type":"tick","tick":5,"elapsed_sec":4.1,"allocated_mb":190,"heap_mb":191,"working_set_mb":695,"usage_percent":24.9,"target_percent":80}
{"type":"tick","tick":6,"elapsed_sec":5.1,"allocated_mb":228,"heap_mb":229,"working_set_mb":695,"usage_percent":29.8,"target_percent":80}
{"type":"tick","tick":7,"elapsed_sec":6.2,"allocated_mb":266,"heap_mb":267,"working_set_mb":696,"usage_percent":34.8,"target_percent":80}
{"type":"tick","tick":8,"elapsed_sec":7.2,"allocated_mb":304,"heap_mb":305,"working_set_mb":734,"usage_percent":39.8,"target_percent":80}
{"type":"tick","tick":9,"elapsed_sec":8.2,"allocated_mb":342,"heap_mb":343,"working_set_mb":734,"usage_percent":44.7,"target_percent":80}
{"type":"tick","tick":10,"elapsed_sec":9.2,"allocated_mb":380,"heap_mb":381,"working_set_mb":772,"usage_percent":49.7,"target_percent":80}
{"type":"tick","tick":11,"elapsed_sec":10.3,"allocated_mb":418,"heap_mb":419,"working_set_mb":772,"usage_percent":54.6,"target_percent":80}
{"type":"tick","tick":12,"elapsed_sec":11.3,"allocated_mb":456,"heap_mb":457,"working_set_mb":772,"usage_percent":59.6,"target_percent":80}
{"type":"tick","tick":13,"elapsed_sec":12.3,"allocated_mb":494,"heap_mb":495,"working_set_mb":810,"usage_percent":64.5,"target_percent":80}
{"type":"tick","tick":14,"elapsed_sec":13.3,"allocated_mb":532,"heap_mb":533,"working_set_mb":810,"usage_percent":69.5,"target_percent":80}
{"type":"tick","tick":15,"elapsed_sec":14.3,"allocated_mb":570,"heap_mb":571,"working_set_mb":810,"usage_percent":74.4,"target_percent":80}
{"type":"tick","tick":16,"elapsed_sec":15.4,"allocated_mb":608,"heap_mb":608,"working_set_mb":697,"usage_percent":79.3,"target_percent":80}
{"type":"tick","tick":17,"elapsed_sec":16.4,"allocated_mb":646,"heap_mb":646,"working_set_mb":735,"usage_percent":84.2,"target_percent":80}
{"type":"done","status":"Memory stress complete","target_percent":80,"rate_percent_per_sec":5,"allocated_mb":646,"iterations":18,"elapsed_seconds":17.4,"memory":{"timestamp":"2026-02-25T01:02:14.7903953Z","memory_used_mb":647.01,"memory_available_mb":768,"memory_usage_percent":84.2,"total_app_memory_mb":735.14}}
```

**Outcome:** Memory grew from ~5% to ~84% over 17 seconds and stopped gracefully at the target.

#### Result: Target 100% — OOM (`?target=100&rate=5`)

The test fills memory until the .NET runtime throws `OutOfMemoryException` at ~94% usage:

```
{"type":"start","target_percent":100,"rate_percent_per_sec":5,"available_mb":768,"target_mb":768,"chunk_mb":38}
{"type":"tick","tick":1,"elapsed_sec":0,"allocated_mb":38,"heap_mb":38,"working_set_mb":628,"usage_percent":5.1,"target_percent":100}
{"type":"tick","tick":2,"elapsed_sec":1.1,"allocated_mb":76,"heap_mb":76,"working_set_mb":628,"usage_percent":10,"target_percent":100}
{"type":"tick","tick":3,"elapsed_sec":2.1,"allocated_mb":114,"heap_mb":115,"working_set_mb":665,"usage_percent":15,"target_percent":100}
{"type":"tick","tick":4,"elapsed_sec":3.1,"allocated_mb":152,"heap_mb":153,"working_set_mb":665,"usage_percent":19.9,"target_percent":100}
{"type":"tick","tick":5,"elapsed_sec":4.1,"allocated_mb":190,"heap_mb":191,"working_set_mb":665,"usage_percent":24.9,"target_percent":100}
{"type":"tick","tick":6,"elapsed_sec":5.2,"allocated_mb":228,"heap_mb":229,"working_set_mb":665,"usage_percent":29.8,"target_percent":100}
{"type":"tick","tick":7,"elapsed_sec":6.3,"allocated_mb":266,"heap_mb":267,"working_set_mb":703,"usage_percent":34.8,"target_percent":100}
{"type":"tick","tick":8,"elapsed_sec":7.3,"allocated_mb":304,"heap_mb":305,"working_set_mb":741,"usage_percent":39.7,"target_percent":100}
{"type":"tick","tick":9,"elapsed_sec":8.3,"allocated_mb":342,"heap_mb":343,"working_set_mb":779,"usage_percent":44.7,"target_percent":100}
{"type":"tick","tick":10,"elapsed_sec":9.4,"allocated_mb":380,"heap_mb":381,"working_set_mb":817,"usage_percent":49.7,"target_percent":100}
{"type":"tick","tick":11,"elapsed_sec":10.4,"allocated_mb":418,"heap_mb":419,"working_set_mb":817,"usage_percent":54.6,"target_percent":100}
{"type":"tick","tick":12,"elapsed_sec":11.4,"allocated_mb":456,"heap_mb":457,"working_set_mb":817,"usage_percent":59.6,"target_percent":100}
{"type":"tick","tick":13,"elapsed_sec":12.4,"allocated_mb":494,"heap_mb":495,"working_set_mb":817,"usage_percent":64.5,"target_percent":100}
{"type":"tick","tick":14,"elapsed_sec":13.4,"allocated_mb":532,"heap_mb":533,"working_set_mb":822,"usage_percent":69.5,"target_percent":100}
{"type":"tick","tick":15,"elapsed_sec":14.4,"allocated_mb":570,"heap_mb":571,"working_set_mb":822,"usage_percent":74.4,"target_percent":100}
{"type":"tick","tick":16,"elapsed_sec":15.4,"allocated_mb":608,"heap_mb":609,"working_set_mb":822,"usage_percent":79.4,"target_percent":100}
{"type":"tick","tick":17,"elapsed_sec":16.5,"allocated_mb":646,"heap_mb":646,"working_set_mb":746,"usage_percent":84.2,"target_percent":100}
{"type":"tick","tick":18,"elapsed_sec":17.5,"allocated_mb":684,"heap_mb":684,"working_set_mb":784,"usage_percent":89.2,"target_percent":100}
{"type":"tick","tick":19,"elapsed_sec":18.5,"allocated_mb":722,"heap_mb":723,"working_set_mb":822,"usage_percent":94.1,"target_percent":100}
{"type":"oom","status":"Memory allocation failed - OOM","error":"Exception of type \u0027System.OutOfMemoryException\u0027 was thrown.","allocated_mb_before_oom":722,"iterations_before_oom":20,"elapsed_seconds":19.5,"memory":{"timestamp":"2026-02-25T01:26:01403232Z","memory_used_mb":722.98,"memory_available_mb":768,"memory_usage_percent":94.1,"total_app_memory_mb":822.89}}
```

**Outcome:** Memory grew to ~94.1% (722 MB of 768 MB available) before .NET threw `OutOfMemoryException` after 20 iterations (~19.5 seconds). The container remained running — the OOM was caught gracefully.

**What to watch for:**
- The `usage_percent` field in each `tick` line climbs toward 100%
- An `oom` message appears if .NET catches the exception before the kernel kills the process
- If the container is killed entirely, `docker ps` will show no container (or `Exited` with code 137)

#### Check if the container was OOM-killed:

```bash
docker inspect dotnet-memorystress --format='{{.State.OOMKilled}}'
```

---

### 4. Stop a Running Test — `GET /stop`

Gracefully cancels a running stress test. The `/stress` response stream will emit a `stopped` message.

```bash
# In a separate terminal while /stress is running:
curl http://localhost:8083/stop
```

**Expected output:**

```json
{"status": "Stop signal sent"}
```

---

### 5. Reset Memory — `GET /reset`

Frees all allocated memory and triggers a full garbage collection (including the Large Object Heap).

```bash
curl http://localhost:8083/reset
```

**Expected output:**

```json
{
  "status": "Memory cleared",
  "memory": {
    "timestamp": "2026-02-24T12:05:00.0000000Z",
    "memory_used_mb": 3.12,
    "memory_available_mb": 1024.0,
    "memory_usage_percent": 0.3,
    "total_app_memory_mb": 28.5
  }
}
```

**Validate:** Call `/memory` after `/reset` — `memory_used_mb` should drop back to baseline (~3–5 MB).

---

## Full Test Walkthrough

Here is a step-by-step scenario to test everything end-to-end:

```bash
# 1. Start the container with 1 GB memory
docker run -d --name dotnet-memorystress --memory=1g -p 8083:8080 memory-stress-samples-dotnet

# 2. Verify it's running
curl http://localhost:8083/

# 3. Check baseline memory
curl http://localhost:8083/memory

# 4. Start gradual stress to 60% (watch output stream)
curl -N http://localhost:8083/stress?target=60&rate=5

# 5. (In another terminal) Monitor memory while stress is running
curl http://localhost:8083/memory

# 6. Reset memory back to baseline
curl http://localhost:8083/reset

# 7. Confirm memory is freed
curl http://localhost:8083/memory

# 8. Run stress to 100% to trigger OOM
curl -N http://localhost:8083/stress?target=100&rate=10

# 9. Check if container was OOM-killed
docker inspect dotnet-memorystress --format='{{.State.OOMKilled}}'

# 10. Clean up
docker rm -f dotnet-memorystress
```

---

## Monitoring via Docker

While a stress test is running, you can also watch container-level memory from the host:

```bash
# Real-time container stats
docker stats dotnet-memorystress

# Container logs (shows the formatted console table)
docker logs -f dotnet-memorystress
```

---

## Project Structure

```
dotnet/
├── Program.cs            # Application code (all endpoints)
├── MemoryStress.csproj   # .NET 8 project file
├── Dockerfile            # Multi-stage Docker build
└── README.md             # This file
```
