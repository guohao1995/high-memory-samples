package com.memorystress;

import java.io.IOException;
import java.io.OutputStream;
import java.lang.management.ManagementFactory;
import java.lang.management.MemoryMXBean;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicBoolean;

import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.servlet.mvc.method.annotation.StreamingResponseBody;

import com.fasterxml.jackson.databind.ObjectMapper;

/**
 * REST Controller for memory stress testing endpoints.
 * Gradually increases memory usage by a target percentage each second,
 * streaming real-time JSON lines to the HTTP response.
 */
@RestController
public class MemoryStressController {

    private static final List<byte[]> memoryHog = Collections.synchronizedList(new ArrayList<>());
    private static final ObjectMapper mapper = new ObjectMapper();
    private static final AtomicBoolean stressRunning = new AtomicBoolean(false);
    private static final AtomicBoolean stopRequested = new AtomicBoolean(false);

    @GetMapping("/")
    public ResponseEntity<String> health() {
        return ResponseEntity.ok("Java App Memory Stress Test Sample\nHealthy");
    }

    /**
     * Returns core memory metrics with friendly names.
     */
    private Map<String, Object> getMemorySnapshot() {
        MemoryMXBean bean = ManagementFactory.getMemoryMXBean();
        Runtime rt = Runtime.getRuntime();
        long heapUsed = bean.getHeapMemoryUsage().getUsed();
        long heapMax = rt.maxMemory();
        double pct = heapMax > 0 ? Math.round(heapUsed * 1000.0 / heapMax) / 10.0 : 0;

        Map<String, Object> snap = new LinkedHashMap<>();
        snap.put("timestamp", Instant.now().toString());
        snap.put("memory_used_mb", Math.round(heapUsed / 1024.0 / 1024.0 * 100.0) / 100.0);
        snap.put("memory_available_mb", Math.round(heapMax / 1024.0 / 1024.0 * 100.0) / 100.0);
        snap.put("memory_usage_percent", pct);
        snap.put("total_app_memory_mb", Math.round(rt.totalMemory() / 1024.0 / 1024.0 * 100.0) / 100.0);
        return snap;
    }

    /**
     * Triggers gradual memory increase. Streams JSON lines (text/plain).
     */
    @GetMapping("/stress")
    public ResponseEntity<StreamingResponseBody> stress(
            @RequestParam(defaultValue = "80") int target,
            @RequestParam(defaultValue = "5") int rate) {

        int targetPercent = (target < 1 || target > 100) ? 80 : target;
        int ratePercent = (rate < 1 || rate > 100) ? 5 : rate;

        if (!stressRunning.compareAndSet(false, true)) {
            return ResponseEntity.status(409)
                    .contentType(MediaType.APPLICATION_JSON)
                    .body(out -> out.write("{\"status\":\"error\",\"message\":\"A stress test is already running. Call /stop first.\"}".getBytes(StandardCharsets.UTF_8)));
        }
        stopRequested.set(false);

        StreamingResponseBody body = (OutputStream out) -> {
            try {
                Runtime rt = Runtime.getRuntime();
                long availableBytes = rt.maxMemory();
                long targetBytes = availableBytes * targetPercent / 100;
                int chunkSizeMB = Math.max(1, (int) (availableBytes * ratePercent / 100 / 1024 / 1024));
                int chunkSizeBytes = chunkSizeMB * 1024 * 1024;

                System.out.println("══════════════════════════════════════════════════════════════");
                System.out.printf("  MEMORY STRESS STARTED  target=%d%%  rate=%d%%/sec%n", targetPercent, ratePercent);
                System.out.println("══════════════════════════════════════════════════════════════");
                System.out.printf("  Available memory : %d MB%n", availableBytes / 1024 / 1024);
                System.out.printf("  Target usage     : %d MB (%d%%)%n", targetBytes / 1024 / 1024, targetPercent);
                System.out.printf("  Chunk per tick   : %d MB (%d%%/sec)%n", chunkSizeMB, ratePercent);

                out.write("Java App Memory Stress Test Sample\n".getBytes(StandardCharsets.UTF_8));
                out.flush();

                Map<String, Object> startObj = new LinkedHashMap<>();
                startObj.put("type", "start");
                startObj.put("target_percent", targetPercent);
                startObj.put("rate_percent_per_sec", ratePercent);
                startObj.put("available_mb", availableBytes / 1024 / 1024);
                startObj.put("target_mb", targetBytes / 1024 / 1024);
                startObj.put("chunk_mb", chunkSizeMB);
                writeLine(out, startObj);

                long startTime = System.currentTimeMillis();
                int totalAllocatedMB = 0;
                int iteration = 0;

                while (true) {
                    if (stopRequested.get()) {
                        double elapsed = (System.currentTimeMillis() - startTime) / 1000.0;
                        System.out.printf("  STRESS STOPPED by user — %d MB in %.1fs%n", totalAllocatedMB, elapsed);
                        Map<String, Object> stopObj = new LinkedHashMap<>();
                        stopObj.put("type", "stopped");
                        stopObj.put("status", "Stress test stopped by user");
                        stopObj.put("allocated_mb", totalAllocatedMB);
                        stopObj.put("iterations", iteration);
                        stopObj.put("elapsed_seconds", Math.round(elapsed * 10) / 10.0);
                        stopObj.put("memory", getMemorySnapshot());
                        writeLine(out, stopObj);
                        return;
                    }

                    iteration++;
                    byte[] chunk = new byte[chunkSizeBytes];
                    Arrays.fill(chunk, (byte) 'X');
                    memoryHog.add(chunk);
                    totalAllocatedMB += chunkSizeMB;

                    MemoryMXBean bean = ManagementFactory.getMemoryMXBean();
                    long heapUsed = bean.getHeapMemoryUsage().getUsed();
                    double usagePct = heapUsed * 100.0 / availableBytes;
                    double elapsed = (System.currentTimeMillis() - startTime) / 1000.0;

                    // OOM detection: when heap reaches ≥99% of max, report OOM and stop.
                    // The kernel OOM killer sends SIGKILL (uncatchable), so we detect
                    // the threshold and report before the kernel acts.
                    if (usagePct >= 99.0 && targetPercent == 100) {
                        System.out.println("██████████████████████████████████████████████████████████████");
                        System.out.printf("  *** OUT OF MEMORY — Heap at %.1f%% of max ***%n", usagePct);
                        System.out.printf("  Iteration: %d  Allocated: %d MB  Elapsed: %.1fs%n", iteration, totalAllocatedMB, elapsed);
                        System.out.println("██████████████████████████████████████████████████████████████");

                        Map<String, Object> oomObj = new LinkedHashMap<>();
                        oomObj.put("type", "oom");
                        oomObj.put("status", "Memory allocation failed - OOM");
                        oomObj.put("error", String.format("Heap reached %.1f%% of max memory limit (%d MB)", usagePct, availableBytes / 1024 / 1024));
                        oomObj.put("allocated_mb_before_oom", totalAllocatedMB);
                        oomObj.put("iterations_before_oom", iteration);
                        oomObj.put("elapsed_seconds", Math.round(elapsed * 10) / 10.0);
                        oomObj.put("memory", getMemorySnapshot());
                        writeLine(out, oomObj);
                        return;
                    }

                    // For target < 100%, stop when we reach the target
                    if (targetPercent < 100 && heapUsed >= targetBytes) {
                        System.out.printf("  STRESS COMPLETE — %d MB in %.1fs%n", totalAllocatedMB, elapsed);
                        Map<String, Object> doneObj = new LinkedHashMap<>();
                        doneObj.put("type", "done");
                        doneObj.put("status", "Memory stress complete");
                        doneObj.put("target_percent", targetPercent);
                        doneObj.put("rate_percent_per_sec", ratePercent);
                        doneObj.put("allocated_mb", totalAllocatedMB);
                        doneObj.put("iterations", iteration);
                        doneObj.put("elapsed_seconds", Math.round(elapsed * 10) / 10.0);
                        doneObj.put("memory", getMemorySnapshot());
                        writeLine(out, doneObj);
                        return;
                    }

                    System.out.printf("  %d  %.1fs  %d MB  heap=%d MB  %.1f%%%n",
                            iteration, elapsed, totalAllocatedMB, heapUsed / 1024 / 1024, usagePct);

                    Map<String, Object> tickObj = new LinkedHashMap<>();
                    tickObj.put("type", "tick");
                    tickObj.put("tick", iteration);
                    tickObj.put("elapsed_sec", Math.round(elapsed * 10) / 10.0);
                    tickObj.put("allocated_mb", totalAllocatedMB);
                    tickObj.put("heap_mb", heapUsed / 1024 / 1024);
                    tickObj.put("usage_percent", Math.round(usagePct * 10) / 10.0);
                    tickObj.put("target_percent", targetPercent);
                    writeLine(out, tickObj);

                    Thread.sleep(1000);
                }

            } catch (OutOfMemoryError e) {
                System.out.println("██████████████████████████████████████████████████████████████");
                System.out.println("  *** OUT OF MEMORY ***");
                System.out.println("  Error: " + e.getMessage());
                System.out.println("██████████████████████████████████████████████████████████████");

                Map<String, Object> oomObj = new LinkedHashMap<>();
                oomObj.put("type", "oom");
                oomObj.put("status", "Memory allocation failed - OOM");
                oomObj.put("error", String.valueOf(e.getMessage()));
                oomObj.put("memory", getMemorySnapshot());
                try { writeLine(out, oomObj); } catch (IOException ignored) {}

            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            } finally {
                stressRunning.set(false);
            }
        };

        return ResponseEntity.ok()
                .contentType(MediaType.parseMediaType("text/plain; charset=utf-8"))
                .header("Cache-Control", "no-cache")
                .header("X-Accel-Buffering", "no")
                .body(body);
    }

    @GetMapping("/stop")
    public ResponseEntity<String> stop() throws Exception {
        String banner = "Java App Memory Stress Test Sample\n";
        if (!stressRunning.get()) {
            return ResponseEntity.ok(banner + mapper.writeValueAsString(Map.of("status", "No stress test running")));
        }
        stopRequested.set(true);
        return ResponseEntity.ok(banner + mapper.writeValueAsString(Map.of("status", "Stop signal sent")));
    }

    @GetMapping("/reset")
    public ResponseEntity<String> reset() throws Exception {
        memoryHog.clear();
        System.gc();
        Map<String, Object> snapshot = getMemorySnapshot();
        System.out.printf("  MEMORY RESET — heap now: %s MB%n", snapshot.get("memory_used_mb"));
        Map<String, Object> resp = new LinkedHashMap<>();
        resp.put("status", "Memory cleared");
        resp.put("memory", snapshot);
        return ResponseEntity.ok("Java App Memory Stress Test Sample\n" + mapper.writeValueAsString(resp));
    }

    @GetMapping("/memory")
    public ResponseEntity<String> memory() throws Exception {
        return ResponseEntity.ok("Java App Memory Stress Test Sample\n" + mapper.writeValueAsString(getMemorySnapshot()));
    }

    private void writeLine(OutputStream out, Map<String, Object> obj) throws IOException {
        out.write(mapper.writeValueAsBytes(obj));
        out.write('\n');
        out.flush();
    }
}
