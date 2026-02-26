# Azure High-Memory-Samples

Multiple languages that purposefully cause high memory — which is useful for testing memory dumps, repros, telemetry, or other scenarios.

---

## Overview

Each application **gradually increases memory usage** at a configurable rate (% per second) until a target usage percentage is reached — or the container is killed by the OOM killer. Real-time memory metrics are streamed as newline-delimited JSON (NDJSON) to the HTTP response.

Use these to:

- **Reproduce memory pressure** scenarios (e.g., memory leaks) in a controlled way
- **Observe how containers behave** when approaching their memory limit
- **Trigger Out-of-Memory (OOM) kills** to test alerting, monitoring, and recovery
- **Validate container memory limits** set via Docker, Kubernetes, or Azure Container Apps
- **Test memory dumps and diagnostics tooling** against realistic memory growth

---

## Applications

| Language | Framework | Port | Internal Port | README |
|----------|-----------|------|---------------|--------|
| Python   | Flask + Gunicorn | 8081 | 8080 | [python/README.md](python/README.md) |
| Node.js  | Express | 8082 | 8080 | [nodejs/README.md](nodejs/README.md) |
| .NET     | ASP.NET Core (Minimal API) | 8083 | 8080 | [dotnet/README.md](dotnet/README.md) |
| Java     | Spring Boot | 8084 | 8080 | [java/README.md](java/README.md) |
| PHP      | Apache + PHP 8.3 | 8085 | 8080 | [php/README.md](php/README.md) |
| Go       | net/http (stdlib) | 8086 | 8080 | [go/README.md](go/README.md) |

---

## Project Structure

```
high-memory-samples/
├── docker-compose.yml        # Run all apps with 2 GB memory limits
├── README.md                 # This file
├── python/                   # Flask app — allocates large byte strings
│   ├── app.py
│   ├── Dockerfile
│   ├── requirements.txt
│   └── README.md
├── nodejs/                   # Express app — allocates Buffers outside V8 heap
│   ├── app.js
│   ├── Dockerfile
│   ├── package.json
│   └── README.md
├── dotnet/                   # ASP.NET Core app — allocates on Large Object Heap
│   ├── Program.cs
│   ├── MemoryStress.csproj
│   ├── Dockerfile
│   └── README.md
├── java/                     # Spring Boot app — allocates byte arrays on JVM heap
│   ├── src/main/java/com/memorystress/
│   ├── pom.xml
│   ├── Dockerfile
│   └── README.md
├── php/                      # PHP/Apache app — allocates strings in global array
│   ├── index.php
│   ├── Dockerfile
│   └── README.md
└── go/                       # Go stdlib app — allocates byte slices
    ├── main.go
    ├── Dockerfile
    ├── go.mod
    └── README.md
```

---

## API Endpoints

All six applications expose the same REST API:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Health check — returns `Healthy` |
| GET | `/stress?target=80&rate=5` | Start gradual memory fill (streaming NDJSON response) |
| GET | `/stop` | Signal the running stress test to stop |
| GET | `/reset` | Free all allocated memory and trigger garbage collection |
| GET | `/memory` | Return a current memory usage snapshot |

### Query Parameters for `/stress`

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `target` | 80 | 1–100 | Target memory usage percentage to reach |
| `rate` | 5 | 1–100 | Percentage of available memory to add per second |

### Streamed Response Types

The `/stress` endpoint streams one JSON object per line, per second:

| Type | When | Meaning |
|------|------|---------|
| `start` | First line | Configuration summary (available MB, target, chunk size) |
| `tick` | Every second | Current allocation progress and memory metrics |
| `done` | End | Stress completed successfully — target reached |
| `oom` | End | Out-of-Memory detected before container kill |
| `stopped` | End | Test was stopped via `/stop` |

---

## Quick Start

### Run All Services with Docker Compose

```bash
docker compose up --build -d
```

This starts all six apps with a **1 GB memory limit** each. Each is accessible on its assigned port:

```
http://localhost:8081   # Python
http://localhost:8082   # Node.js
http://localhost:8083   # .NET
http://localhost:8084   # Java
http://localhost:8085   # PHP
http://localhost:8086   # Go
```

### Run a Single Service

```bash
# Example: run only the Go app
docker compose up go --build -d
```

### Run Standalone with a Custom Memory Limit

```bash
# Build
docker compose build python

# Run with 1 GB memory limit
docker run -d --name python-memorystress --memory=1g -p 8081:8080 memory-stress-samples-python
```

---

## Testing

### 1. Health Check

```bash
curl http://localhost:8081/
# Python App Memory Stress Test Sample
# Healthy
```

### 2. Check Memory Usage

```bash
curl http://localhost:8081/memory
```

Returns a JSON snapshot:

```json
{
  "timestamp": "2026-02-25T01:00:00.000Z",
  "memory_used_mb": 25.4,
  "memory_available_mb": 1073.0,
  "memory_usage_percent": 2.4,
  "total_app_memory_mb": 25.4
}
```

### 3. Gradual Stress Test (Streaming)

```bash
# Fill to 80% at 5%/sec (use -N for real-time streaming)
curl -N http://localhost:8081/stress?target=80&rate=5
```

Streamed output (one JSON line per second):

```
{"type":"start","target_percent":80,"rate_percent_per_sec":5,"available_mb":1073,"target_mb":858,"chunk_mb":53}
{"type":"tick","tick":1,"elapsed_sec":0.1,"allocated_mb":53,"rss_mb":78,"usage_percent":7.3,"target_percent":80}
{"type":"tick","tick":2,"elapsed_sec":1.1,"allocated_mb":106,"rss_mb":131,"usage_percent":12.2,"target_percent":80}
...
{"type":"done","status":"Memory stress complete",...}
```

### 4. Trigger OOM

```bash
# Push to 100% — will hit OOM threshold or get killed
curl -N http://localhost:8081/stress?target=100&rate=10
```

### 5. Stop a Running Test

```bash
curl http://localhost:8081/stop
```

### 6. Reset Memory

```bash
curl http://localhost:8081/reset
```

### 7. Check OOM Kill Status

```bash
docker inspect python-memorystress --format='{{.State.OOMKilled}}'
```

---

## Monitoring

While a stress test is running, monitor container-level metrics from the host:

```bash
# Real-time container stats for all services
docker stats

# Container logs for a specific service
docker compose logs -f python
```

---

## Language-Specific Details

Each app has its own README with full details, example results, and language-specific notes:

| Language | Key Details | README |
|----------|-------------|--------|
| **Python** | Uses Flask + Gunicorn; allocates `bytes` objects; reads RSS from `/proc/self/status` and cgroup limits | [python/README.md](python/README.md) |
| **Node.js** | Uses Express; allocates `Buffer` objects (outside V8 heap); tracks both `heap_mb` and `rss_mb` | [nodejs/README.md](nodejs/README.md) |
| **.NET** | ASP.NET Core Minimal API; allocates on the Large Object Heap (LOH); uses `GC.GetGCMemoryInfo()` for container-aware metrics | [dotnet/README.md](dotnet/README.md) |
| **Java** | Spring Boot with `StreamingResponseBody`; uses `Runtime.maxMemory()` for heap ceiling; JVM auto-configures via `-XX:MaxRAMPercentage=80.0` | [java/README.md](java/README.md) |
| **PHP** | PHP 8.3 on Apache; uses cgroup-aware memory tracking; communicates stop signal via temp file between requests | [php/README.md](php/README.md) |
| **Go** | Go stdlib `net/http`; allocates byte slices; reads RSS from `/proc/self/status` and cgroup limits | [go/README.md](go/README.md) |

---

## Warning

> **These applications are designed to consume excessive memory and may crash your container or system. Use only in controlled testing environments.**
