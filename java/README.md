# Memory Stress Test — Java (Spring Boot)

## Purpose

A Java application that **gradually increases memory usage** by a configurable target percentage each second. It streams real-time memory metrics as newline-delimited JSON to the HTTP response, so you can monitor progress in a browser or with `curl`. Designed to help test container memory limits and OOM-kill behaviour.

---

## Prerequisites

- **Docker** installed and running
- (Optional) **Java 21+** and **Maven** for running outside Docker

---

## How to Run

### Option 1 — Docker Compose (recommended)

From the repository root (`memory-stress-samples/`):

```bash
docker compose up --build java
```

The service is available at **http://localhost:8084**.

### Option 2 — Standalone Docker

```bash
cd java
docker build -t memory-stress-java .
docker run --rm -p 8084:8080 --memory=1g memory-stress-java
```

### Option 3 — Run locally

```bash
cd java
mvn clean package -DskipTests
java -jar target/memory-stress-0.0.1-SNAPSHOT.jar
```

Runs on **http://localhost:8080** by default.

---

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Health check — returns `Healthy` |
| GET | `/stress?target=80&rate=5` | Start gradual memory fill (streaming response) |
| GET | `/stop` | Signal the running stress test to stop |
| GET | `/reset` | Free all allocated memory and request GC |
| GET | `/memory` | Return current memory snapshot |

### Query Parameters for `/stress`

| Parameter | Default | Range | Meaning |
|-----------|---------|-------|---------|
| `target` | 80 | 1-100 | Target memory usage percentage |
| `rate` | 5 | 1-100 | Percentage of max heap to add per second |

---

## How to Test

### 1. Health Check

```bash
curl http://localhost:8084/
```

### 2. Check Memory

```bash
curl http://localhost:8084/memory
```

Returns four key metrics:

```json
{
  "timestamp": "2025-06-01T12:00:00.000Z",
  "memory_used_mb": 42.5,
  "memory_available_mb": 512.0,
  "memory_usage_percent": 8.3,
  "total_app_memory_mb": 128.0
}
```

### 3. Run a Stress Test (streaming)

```bash
curl http://localhost:8084/stress?target=80&rate=5
```

Or open in a browser — the response streams one JSON object per line, per second:

```
{"type":"start","target_percent":80,"rate_percent_per_sec":5,...}
{"type":"tick","tick":1,"elapsed_sec":1.0,"allocated_mb":25,"heap_mb":67,"usage_percent":13.1,...}
{"type":"tick","tick":2,"elapsed_sec":2.0,"allocated_mb":50,"heap_mb":92,"usage_percent":18.0,...}
...
{"type":"done","status":"Memory stress complete",...}
```

### Example Results (1 GB container)

#### Result: Target 80% memory usage (`?target=80&rate=5`)

The test gradually fills heap memory to 80% and completes successfully after 16 iterations (~16 seconds):

```
Java App Memory Stress Test Sample
{"type":"start","target_percent":80,"rate_percent_per_sec":5,"available_mb":512,"target_mb":409,"chunk_mb":25}
{"type":"tick","tick":1,"elapsed_sec":0.1,"allocated_mb":25,"heap_mb":67,"usage_percent":13.1,"target_percent":80}
{"type":"tick","tick":2,"elapsed_sec":1.1,"allocated_mb":50,"heap_mb":92,"usage_percent":18,"target_percent":80}
{"type":"tick","tick":3,"elapsed_sec":2.1,"allocated_mb":75,"heap_mb":117,"usage_percent":22.9,"target_percent":80}
{"type":"tick","tick":4,"elapsed_sec":3.1,"allocated_mb":100,"heap_mb":142,"usage_percent":27.8,"target_percent":80}
{"type":"tick","tick":5,"elapsed_sec":4.1,"allocated_mb":125,"heap_mb":167,"usage_percent":32.7,"target_percent":80}
{"type":"tick","tick":6,"elapsed_sec":5.2,"allocated_mb":150,"heap_mb":192,"usage_percent":37.6,"target_percent":80}
{"type":"tick","tick":7,"elapsed_sec":6.2,"allocated_mb":175,"heap_mb":217,"usage_percent":42.5,"target_percent":80}
{"type":"tick","tick":8,"elapsed_sec":7.2,"allocated_mb":200,"heap_mb":242,"usage_percent":47.3,"target_percent":80}
{"type":"tick","tick":9,"elapsed_sec":8.2,"allocated_mb":225,"heap_mb":267,"usage_percent":52.2,"target_percent":80}
{"type":"tick","tick":10,"elapsed_sec":9.3,"allocated_mb":250,"heap_mb":292,"usage_percent":57.1,"target_percent":80}
{"type":"tick","tick":11,"elapsed_sec":10.3,"allocated_mb":275,"heap_mb":317,"usage_percent":62,"target_percent":80}
{"type":"tick","tick":12,"elapsed_sec":11.3,"allocated_mb":300,"heap_mb":342,"usage_percent":66.9,"target_percent":80}
{"type":"tick","tick":13,"elapsed_sec":12.3,"allocated_mb":325,"heap_mb":367,"usage_percent":71.8,"target_percent":80}
{"type":"tick","tick":14,"elapsed_sec":13.4,"allocated_mb":350,"heap_mb":392,"usage_percent":76.6,"target_percent":80}
{"type":"tick","tick":15,"elapsed_sec":14.4,"allocated_mb":375,"heap_mb":417,"usage_percent":81.5,"target_percent":80}
{"type":"done","status":"Memory stress complete","target_percent":80,"rate_percent_per_sec":5,"allocated_mb":375,"iterations":15,"elapsed_seconds":14.4,"memory":{"timestamp":"2026-02-25T01:10:20.000Z","memory_used_mb":417.0,"memory_available_mb":512.0,"memory_usage_percent":81.5,"total_app_memory_mb":512.0}}
```

**Outcome:** Heap usage grew from ~13% to ~81.5% over 15 iterations and stopped gracefully at the target.

#### Result: Target 100% — OOM (`?target=100&rate=5`)

The test fills heap memory until it reaches ~99% of max heap:

```
Java App Memory Stress Test Sample
{"type":"start","target_percent":100,"rate_percent_per_sec":5,"available_mb":512,"target_mb":512,"chunk_mb":25}
{"type":"tick","tick":1,"elapsed_sec":0.1,"allocated_mb":25,"heap_mb":67,"usage_percent":13.1,"target_percent":100}
{"type":"tick","tick":2,"elapsed_sec":1.1,"allocated_mb":50,"heap_mb":92,"usage_percent":18,"target_percent":100}
{"type":"tick","tick":3,"elapsed_sec":2.1,"allocated_mb":75,"heap_mb":117,"usage_percent":22.9,"target_percent":100}
{"type":"tick","tick":4,"elapsed_sec":3.1,"allocated_mb":100,"heap_mb":142,"usage_percent":27.8,"target_percent":100}
{"type":"tick","tick":5,"elapsed_sec":4.2,"allocated_mb":125,"heap_mb":167,"usage_percent":32.7,"target_percent":100}
{"type":"tick","tick":6,"elapsed_sec":5.2,"allocated_mb":150,"heap_mb":192,"usage_percent":37.6,"target_percent":100}
{"type":"tick","tick":7,"elapsed_sec":6.2,"allocated_mb":175,"heap_mb":217,"usage_percent":42.5,"target_percent":100}
{"type":"tick","tick":8,"elapsed_sec":7.2,"allocated_mb":200,"heap_mb":242,"usage_percent":47.3,"target_percent":100}
{"type":"tick","tick":9,"elapsed_sec":8.3,"allocated_mb":225,"heap_mb":267,"usage_percent":52.2,"target_percent":100}
{"type":"tick","tick":10,"elapsed_sec":9.3,"allocated_mb":250,"heap_mb":292,"usage_percent":57.1,"target_percent":100}
{"type":"tick","tick":11,"elapsed_sec":10.3,"allocated_mb":275,"heap_mb":317,"usage_percent":62,"target_percent":100}
{"type":"tick","tick":12,"elapsed_sec":11.3,"allocated_mb":300,"heap_mb":342,"usage_percent":66.9,"target_percent":100}
{"type":"tick","tick":13,"elapsed_sec":12.4,"allocated_mb":325,"heap_mb":367,"usage_percent":71.8,"target_percent":100}
{"type":"tick","tick":14,"elapsed_sec":13.4,"allocated_mb":350,"heap_mb":392,"usage_percent":76.6,"target_percent":100}
{"type":"tick","tick":15,"elapsed_sec":14.4,"allocated_mb":375,"heap_mb":417,"usage_percent":81.5,"target_percent":100}
{"type":"tick","tick":16,"elapsed_sec":15.4,"allocated_mb":400,"heap_mb":442,"usage_percent":86.4,"target_percent":100}
{"type":"tick","tick":17,"elapsed_sec":16.5,"allocated_mb":425,"heap_mb":467,"usage_percent":91.3,"target_percent":100}
{"type":"tick","tick":18,"elapsed_sec":17.5,"allocated_mb":450,"heap_mb":492,"usage_percent":96.2,"target_percent":100}
{"type":"oom","status":"Memory allocation failed - OOM","error":"Heap reached 99.4% of max memory limit (512 MB)","allocated_mb_before_oom":475,"iterations_before_oom":19,"elapsed_seconds":18.5,"memory":{"timestamp":"2026-02-25T01:12:45.000Z","memory_used_mb":509.0,"memory_available_mb":512.0,"memory_usage_percent":99.4,"total_app_memory_mb":512.0}}
```

**Outcome:** Heap usage grew to ~99.4% (509 MB of 512 MB max heap) before the app detected OOM threshold after 19 iterations (~18.5 seconds). The container remained running — the JVM caught the condition before `OutOfMemoryError` or the kernel OOM killer.

**What to watch for:**
- The `usage_percent` field in each `tick` line climbs toward 100%
- An `oom` message appears when heap reaches ≥99% of max memory
- If the JVM throws `OutOfMemoryError`, the error message will be included in the `oom` line
- If the container is killed entirely, `docker ps` will show no container (or `Exited` with code 137)

#### Check if the container was OOM-killed:

```bash
docker inspect java-memorystress --format='{{.State.OOMKilled}}'
```

### 4. Stop a Running Test

```bash
curl http://localhost:8084/stop
```

### 5. Reset Memory

```bash
curl http://localhost:8084/reset
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

- Uses `StreamingResponseBody` from Spring for real-time HTTP streaming.
- `Runtime.maxMemory()` determines the JVM heap ceiling; set `-Xmx` in your Dockerfile or compose file if needed.
- The JVM's `OutOfMemoryError` is caught and reported in the stream.

---

## Project Structure

```
java/
├── src/main/java/com/memorystress/
│   ├── MemoryStressApplication.java   # Spring Boot entry point
│   └── MemoryStressController.java    # REST controller (streaming stress)
├── src/main/resources/
│   └── application.properties         # Spring Boot config
├── pom.xml                            # Maven build (Spring Boot, Java 21)
├── Dockerfile                         # Multi-stage build (eclipse-temurin)
└── README.md                          # This file
```
