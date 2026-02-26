// Memory Stress Test Application - Go
// Gradually increases memory usage by a target percentage each second,
// with detailed monitoring output and OOM handling.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	memoryHog     [][]byte
	mu            sync.Mutex
	stressRunning bool
	stopChan      chan struct{}
)

// readCgroupMemoryLimit reads the container memory limit from cgroup v1 or v2.
// Returns 0 if not running in a container or limit is not set.
func readCgroupMemoryLimit() uint64 {
	paths := []string{
		"/sys/fs/cgroup/memory/memory.limit_in_bytes", // cgroup v1
		"/sys/fs/cgroup/memory.max",                    // cgroup v2
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := strings.TrimSpace(string(data))
		if s == "max" {
			continue
		}
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			continue
		}
		// Ignore unreasonably large values (= unlimited)
		if v < 1<<60 {
			return v
		}
	}
	return 0
}

// getAvailableBytes returns the effective memory limit for the process.
func getAvailableBytes() uint64 {
	if limit := readCgroupMemoryLimit(); limit > 0 {
		return limit
	}
	// Fallback to runtime Sys
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	if m.Sys > 0 {
		return m.Sys
	}
	return 512 * 1024 * 1024
}

// readRSSBytes reads the actual resident set size from /proc/self/status.
// Falls back to runtime.MemStats.Alloc if /proc is unavailable.
func readRSSBytes() uint64 {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.Alloc
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					return v * 1024 // VmRSS is in kB
				}
			}
		}
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

// memorySnapshot returns core memory metrics with friendly names.
func memorySnapshot() map[string]interface{} {
	rssBytes := readRSSBytes()
	available := getAvailableBytes()
	usedMB := float64(rssBytes) / 1024 / 1024
	availableMB := float64(available) / 1024 / 1024
	usagePercent := 0.0
	if available > 0 {
		usagePercent = float64(rssBytes) * 100.0 / float64(available)
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return map[string]interface{}{
		"timestamp":            time.Now().UTC().Format(time.RFC3339Nano),
		"memory_used_mb":       round2(usedMB),
		"memory_available_mb":  round2(availableMB),
		"memory_usage_percent": round1(usagePercent),
		"total_app_memory_mb":  round2(float64(m.Sys) / 1024 / 1024),
	}
}

func round1(v float64) float64 { return float64(int(v*10)) / 10 }
func round2(v float64) float64 { return float64(int(v*100)) / 100 }

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", healthHandler)
	http.HandleFunc("/stress", stressHandler)
	http.HandleFunc("/stop", stopHandler)
	http.HandleFunc("/reset", resetHandler)
	http.HandleFunc("/memory", memoryHandler)

	log.Printf("Memory Stress Server running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Go App Memory Stress Test Sample\nHealthy"))
}

func stressHandler(w http.ResponseWriter, r *http.Request) {
	targetPercent := 80
	ratePercent := 5
	if v, err := strconv.Atoi(r.URL.Query().Get("target")); err == nil && v > 0 && v <= 100 {
		targetPercent = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("rate")); err == nil && v > 0 && v <= 100 {
		ratePercent = v
	}

	if stressRunning {
		writeJSON(w, 409, map[string]string{"status": "error", "message": "A stress test is already running. Call /stop first."})
		return
	}

	stressRunning = true
	stopChan = make(chan struct{})

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, 500, map[string]string{"error": "Streaming not supported"})
		stressRunning = false
		return
	}

	log.Println("══════════════════════════════════════════════════════════════")
	log.Printf("  MEMORY STRESS STARTED  target=%d%%  rate=%d%%/sec", targetPercent, ratePercent)
	log.Println("══════════════════════════════════════════════════════════════")

	start := time.Now()
	totalAllocatedMB := 0
	iteration := 0

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	availableBytes := getAvailableBytes()
	targetBytes := uint64(float64(availableBytes) * float64(targetPercent) / 100.0)
	chunkSizeMB := int(float64(availableBytes) * float64(ratePercent) / 100.0 / 1024 / 1024)
	if chunkSizeMB < 1 {
		chunkSizeMB = 1
	}
	chunkSizeBytes := chunkSizeMB * 1024 * 1024

	fmt.Fprintf(w, "Go App Memory Stress Test Sample\n")
	flusher.Flush()

	startLine, _ := json.Marshal(map[string]interface{}{
		"type":                 "start",
		"target_percent":       targetPercent,
		"rate_percent_per_sec": ratePercent,
		"available_mb":         availableBytes / 1024 / 1024,
		"target_mb":            targetBytes / 1024 / 1024,
		"chunk_mb":             chunkSizeMB,
	})
	fmt.Fprintf(w, "%s\n", startLine)
	flusher.Flush()

	log.Printf("  Available memory : %d MB", availableBytes/1024/1024)
	log.Printf("  Target usage     : %d MB (%d%%)", targetBytes/1024/1024, targetPercent)
	log.Printf("  Chunk per tick   : %d MB (%d%%/sec)", chunkSizeMB, ratePercent)

	defer func() {
		if rec := recover(); rec != nil {
			stressRunning = false
			elapsed := time.Since(start).Seconds()
			log.Println("██████████████████████████████████████████████████████████████")
			log.Printf("  *** OUT OF MEMORY (panic) ***")
			log.Printf("  Iteration: %d  Allocated: %d MB  Elapsed: %.1fs", iteration, totalAllocatedMB, elapsed)
			log.Println("██████████████████████████████████████████████████████████████")

			oomLine, _ := json.Marshal(map[string]interface{}{
				"type":                    "oom",
				"status":                  "Memory allocation failed - OOM",
				"error":                   fmt.Sprintf("%v", rec),
				"allocated_mb_before_oom": totalAllocatedMB,
				"iterations_before_oom":   iteration,
				"elapsed_seconds":         round1(elapsed),
				"memory":                  memorySnapshot(),
			})
			fmt.Fprintf(w, "%s\n", oomLine)
			flusher.Flush()
		}
	}()

	for {
		select {
		case <-stopChan:
			stressRunning = false
			elapsed := time.Since(start).Seconds()
			log.Printf("  STRESS STOPPED by user — %d MB allocated in %.1fs", totalAllocatedMB, elapsed)

			stopLine, _ := json.Marshal(map[string]interface{}{
				"type":            "stopped",
				"status":          "Stress test stopped by user",
				"allocated_mb":    totalAllocatedMB,
				"iterations":      iteration,
				"elapsed_seconds": round1(elapsed),
				"memory":          memorySnapshot(),
			})
			fmt.Fprintf(w, "%s\n", stopLine)
			flusher.Flush()
			return
		default:
		}

		iteration++

		// Allocate FIRST — if this panics with "out of memory", defer/recover catches it
		mu.Lock()
		chunk := make([]byte, chunkSizeBytes)
		for i := range chunk {
			chunk[i] = 'X'
		}
		memoryHog = append(memoryHog, chunk)
		totalAllocatedMB += chunkSizeMB
		mu.Unlock()

		// Use actual RSS (what the OOM killer watches) for the target check
		currentRSS := readRSSBytes()
		usagePercent := float64(currentRSS) * 100.0 / float64(availableBytes)
		elapsed := time.Since(start).Seconds()

		// OOM detection: when RSS reaches ≥99% of container memory, report OOM and stop.
		// We do this because the kernel OOM killer sends SIGKILL (uncatchable),
		// so we must detect the threshold and report before the kernel acts.
		if usagePercent >= 99.0 && targetPercent == 100 {
			log.Println("██████████████████████████████████████████████████████████████")
			log.Printf("  *** OUT OF MEMORY — RSS at %.1f%% of container limit ***", usagePercent)
			log.Printf("  Iteration: %d  Allocated: %d MB  RSS: %d MB  Elapsed: %.1fs", iteration, totalAllocatedMB, currentRSS/1024/1024, elapsed)
			log.Println("██████████████████████████████████████████████████████████████")

			stressRunning = false
			oomLine, _ := json.Marshal(map[string]interface{}{
				"type":                    "oom",
				"status":                  "Memory allocation failed - OOM",
				"error":                   fmt.Sprintf("RSS reached %.1f%% of container memory limit (%d MB)", usagePercent, availableBytes/1024/1024),
				"allocated_mb_before_oom": totalAllocatedMB,
				"iterations_before_oom":   iteration,
				"elapsed_seconds":         round1(elapsed),
				"rss_mb":                  currentRSS / 1024 / 1024,
				"usage_percent":           round1(usagePercent),
				"memory":                  memorySnapshot(),
			})
			fmt.Fprintf(w, "%s\n", oomLine)
			flusher.Flush()
			return
		}

		// For target < 100%, break when we reach the target after a successful allocation
		if targetPercent < 100 && currentRSS >= targetBytes {
			log.Printf("  %d  %.1fs  %d MB  rss=%d MB  %.1f%%", iteration, elapsed, totalAllocatedMB, currentRSS/1024/1024, usagePercent)
			break
		}

		log.Printf("  %d  %.1fs  %d MB  rss=%d MB  %.1f%%", iteration, elapsed, totalAllocatedMB, currentRSS/1024/1024, usagePercent)

		tickLine, _ := json.Marshal(map[string]interface{}{
			"type":           "tick",
			"tick":           iteration,
			"elapsed_sec":    round1(elapsed),
			"allocated_mb":   totalAllocatedMB,
			"rss_mb":         currentRSS / 1024 / 1024,
			"usage_percent":  round1(usagePercent),
			"target_percent": targetPercent,
		})
		fmt.Fprintf(w, "%s\n", tickLine)
		flusher.Flush()

		time.Sleep(1 * time.Second)
	}

	stressRunning = false
	elapsed := time.Since(start).Seconds()
	log.Printf("  STRESS COMPLETE — %d MB allocated in %.1fs", totalAllocatedMB, elapsed)

	doneLine, _ := json.Marshal(map[string]interface{}{
		"type":                 "done",
		"status":               "Memory stress complete",
		"target_percent":       targetPercent,
		"rate_percent_per_sec": ratePercent,
		"allocated_mb":         totalAllocatedMB,
		"iterations":           iteration,
		"elapsed_seconds":      round1(elapsed),
		"memory":               memorySnapshot(),
	})
	fmt.Fprintf(w, "%s\n", doneLine)
	flusher.Flush()
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if !stressRunning {
		jsonBytes, _ := json.Marshal(map[string]string{"status": "No stress test running"})
		fmt.Fprintf(w, "Go App Memory Stress Test Sample\n%s", jsonBytes)
		return
	}
	close(stopChan)
	jsonBytes, _ := json.Marshal(map[string]string{"status": "Stop signal sent"})
	fmt.Fprintf(w, "Go App Memory Stress Test Sample\n%s", jsonBytes)
}

func resetHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	memoryHog = nil
	mu.Unlock()

	runtime.GC()

	snapshot := memorySnapshot()
	log.Printf("  MEMORY RESET — heap now: %v MB", snapshot["memory_used_mb"])

	resp := map[string]interface{}{
		"status": "Memory cleared",
		"memory": snapshot,
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	jsonBytes, _ := json.Marshal(resp)
	fmt.Fprintf(w, "Go App Memory Stress Test Sample\n%s", jsonBytes)
}

func memoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	jsonBytes, _ := json.Marshal(memorySnapshot())
	fmt.Fprintf(w, "Go App Memory Stress Test Sample\n%s", jsonBytes)
}
