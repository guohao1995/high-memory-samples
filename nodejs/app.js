/**
 * Memory Stress Test Application - Node.js/Express
 * Gradually increases memory usage by a target percentage each second,
 * with detailed monitoring output and OOM handling.
 */

const express = require('express');
const fs = require('fs');
const app = express();

// Global array to hold memory allocations (prevents garbage collection)
const memoryHog = [];

// Track active stress test
let stressRunning = false;
let stopRequested = false;

// Port configuration
const PORT = process.env.PORT || 8080;

/**
 * Read container memory limit from cgroup v1 or v2.
 * Returns 0 if not running in a container or limit is not set.
 */
function readCgroupMemoryLimit() {
    const paths = [
        '/sys/fs/cgroup/memory/memory.limit_in_bytes', // cgroup v1
        '/sys/fs/cgroup/memory.max',                    // cgroup v2
    ];
    for (const p of paths) {
        try {
            const val = fs.readFileSync(p, 'utf8').trim();
            if (val === 'max') continue;
            const limit = parseInt(val, 10);
            if (!isNaN(limit) && limit < 2 ** 60) return limit;
        } catch (e) {
            // not available
        }
    }
    return 0;
}

/**
 * Get available memory bytes — prefers cgroup limit, falls back to RSS estimate.
 */
function getAvailableBytes() {
    const limit = readCgroupMemoryLimit();
    if (limit > 0) return limit;
    // Fallback: estimate from RSS + headroom
    return process.memoryUsage().rss + 800 * 1024 * 1024;
}

/**
 * Returns core memory metrics with friendly names.
 * Uses RSS as "used" and cgroup limit as "available" for accurate container metrics.
 */
function getMemorySnapshot() {
    const mem = process.memoryUsage();
    const rssMB = mem.rss / 1024 / 1024;
    const availableBytes = getAvailableBytes();
    const availableMB = availableBytes / 1024 / 1024;
    const usagePct = availableBytes > 0 ? mem.rss / availableBytes * 100 : 0;
    return {
        timestamp: new Date().toISOString(),
        memory_used_mb: Math.round(rssMB * 100) / 100,
        memory_available_mb: Math.round(availableMB * 100) / 100,
        memory_usage_percent: Math.round(usagePct * 10) / 10,
        total_app_memory_mb: Math.round(rssMB * 100) / 100
    };
}

/**
 * Health check endpoint for container probes
 */
app.get('/', (req, res) => {
    res.status(200).type('text/plain').send('Node.js App Memory Stress Test Sample\nHealthy');
});

/**
 * Triggers gradual memory increase.
 * Streams JSON lines (text/plain) so browsers display inline.
 * Query params:
 *   ?target=80  target memory usage % (default 80)
 *   ?rate=5     % of RSS to add per second (default 5)
 */
app.get('/stress', (req, res) => {
    let targetPercent = parseInt(req.query.target) || 80;
    let ratePercent = parseInt(req.query.rate) || 5;
    if (targetPercent < 1 || targetPercent > 100) targetPercent = 80;
    if (ratePercent < 1 || ratePercent > 100) ratePercent = 5;

    if (stressRunning) {
        return res.status(409).json({ status: 'error', message: 'A stress test is already running. Call /stop first.' });
    }

    stressRunning = true;
    stopRequested = false;

    // Stream plain text so browsers display inline
    res.setHeader('Content-Type', 'text/plain; charset=utf-8');
    res.setHeader('Cache-Control', 'no-cache');
    res.setHeader('X-Accel-Buffering', 'no');
    res.status(200);

    console.log('══════════════════════════════════════════════════════════════');
    console.log(`  MEMORY STRESS STARTED  target=${targetPercent}%  rate=${ratePercent}%/sec`);
    console.log('══════════════════════════════════════════════════════════════');

    const startTime = Date.now();
    let totalAllocatedMB = 0;
    let iteration = 0;

    // Read container memory limit from cgroup
    const availableBytes = getAvailableBytes();
    const targetBytes = availableBytes * targetPercent / 100;
    const chunkSizeMB = Math.max(1, Math.floor(availableBytes * ratePercent / 100 / 1024 / 1024));
    const chunkSizeBytes = chunkSizeMB * 1024 * 1024;

    res.write('Node.js App Memory Stress Test Sample\n');

    const startLine = JSON.stringify({
        type: 'start',
        target_percent: targetPercent,
        rate_percent_per_sec: ratePercent,
        available_mb: Math.floor(availableBytes / 1024 / 1024),
        target_mb: Math.floor(targetBytes / 1024 / 1024),
        chunk_mb: chunkSizeMB
    });
    res.write(startLine + '\n');

    console.log(`  Available memory : ${Math.floor(availableBytes / 1024 / 1024)} MB`);
    console.log(`  Target usage     : ${Math.floor(targetBytes / 1024 / 1024)} MB (${targetPercent}%)`);
    console.log(`  Chunk per tick   : ${chunkSizeMB} MB (${ratePercent}%/sec)`);

    const interval = setInterval(() => {
        try {
            if (stopRequested) {
                clearInterval(interval);
                stressRunning = false;
                const elapsed = (Date.now() - startTime) / 1000;
                console.log(`  STRESS STOPPED by user — ${totalAllocatedMB} MB allocated in ${elapsed.toFixed(1)}s`);

                const stopLine = JSON.stringify({
                    type: 'stopped',
                    status: 'Stress test stopped by user',
                    allocated_mb: totalAllocatedMB,
                    iterations: iteration,
                    elapsed_seconds: Math.round(elapsed * 10) / 10,
                    memory: getMemorySnapshot()
                });
                res.write(stopLine + '\n');
                res.end();
                return;
            }

            const currentMem = process.memoryUsage();

            // Allocate FIRST, then check — so we detect the threshold after the allocation
            iteration++;
            const chunk = Buffer.alloc(chunkSizeBytes, 'X');
            memoryHog.push(chunk);
            totalAllocatedMB += chunkSizeMB;

            const mem = process.memoryUsage();
            const usagePercent = mem.rss * 100 / availableBytes;
            const elapsed = (Date.now() - startTime) / 1000;

            // OOM detection: when RSS reaches ≥99% of container memory, report OOM and stop.
            // The kernel OOM killer sends SIGKILL (uncatchable), so we detect
            // the threshold and report before the kernel acts.
            if (usagePercent >= 99.0 && targetPercent === 100) {
                clearInterval(interval);
                stressRunning = false;
                console.log('██████████████████████████████████████████████████████████████');
                console.log(`  *** OUT OF MEMORY — RSS at ${usagePercent.toFixed(1)}% of container limit ***`);
                console.log(`  Iteration: ${iteration}  Allocated: ${totalAllocatedMB} MB  RSS: ${Math.floor(mem.rss / 1024 / 1024)} MB  Elapsed: ${elapsed.toFixed(1)}s`);
                console.log('██████████████████████████████████████████████████████████████');

                const oomLine = JSON.stringify({
                    type: 'oom',
                    status: 'Memory allocation failed - OOM',
                    error: `RSS reached ${usagePercent.toFixed(1)}% of container memory limit (${Math.floor(availableBytes / 1024 / 1024)} MB)`,
                    allocated_mb_before_oom: totalAllocatedMB,
                    iterations_before_oom: iteration,
                    elapsed_seconds: Math.round(elapsed * 10) / 10,
                    rss_mb: Math.floor(mem.rss / 1024 / 1024),
                    usage_percent: Math.round(usagePercent * 10) / 10,
                    memory: getMemorySnapshot()
                });
                res.write(oomLine + '\n');
                res.end();
                return;
            }

            // For target < 100%, stop when we reach the target
            if (targetPercent < 100 && mem.rss >= targetBytes) {
                clearInterval(interval);
                stressRunning = false;
                console.log(`  STRESS COMPLETE — ${totalAllocatedMB} MB allocated in ${elapsed.toFixed(1)}s`);

                const doneLine = JSON.stringify({
                    type: 'done',
                    status: 'Memory stress complete',
                    target_percent: targetPercent,
                    rate_percent_per_sec: ratePercent,
                    allocated_mb: totalAllocatedMB,
                    iterations: iteration,
                    elapsed_seconds: Math.round(elapsed * 10) / 10,
                    memory: getMemorySnapshot()
                });
                res.write(doneLine + '\n');
                res.end();
                return;
            }

            console.log(`  ${iteration}  ${elapsed.toFixed(1)}s  ${totalAllocatedMB} MB  ${Math.floor(mem.heapUsed / 1024 / 1024)} MB  ${Math.floor(mem.rss / 1024 / 1024)} MB  ${usagePercent.toFixed(1)}%`);

            const tickLine = JSON.stringify({
                type: 'tick',
                tick: iteration,
                elapsed_sec: Math.round(elapsed * 10) / 10,
                allocated_mb: totalAllocatedMB,
                heap_mb: Math.floor(mem.heapUsed / 1024 / 1024),
                rss_mb: Math.floor(mem.rss / 1024 / 1024),
                usage_percent: Math.round(usagePercent * 10) / 10,
                target_percent: targetPercent
            });
            res.write(tickLine + '\n');

        } catch (error) {
            clearInterval(interval);
            stressRunning = false;
            const elapsed = (Date.now() - startTime) / 1000;
            console.log('██████████████████████████████████████████████████████████████');
            console.log('  *** OUT OF MEMORY ***');
            console.log(`  Iteration: ${iteration}  Allocated: ${totalAllocatedMB} MB  Elapsed: ${elapsed.toFixed(1)}s`);
            console.log(`  Error: ${error.message}`);
            console.log('██████████████████████████████████████████████████████████████');

            const oomLine = JSON.stringify({
                type: 'oom',
                status: 'Memory allocation failed - OOM',
                error: error.message,
                allocated_mb_before_oom: totalAllocatedMB,
                iterations_before_oom: iteration,
                elapsed_seconds: Math.round(elapsed * 10) / 10,
                memory: getMemorySnapshot()
            });
            res.write(oomLine + '\n');
            res.end();
        }
    }, 1000);

    // Handle client disconnect
    req.on('close', () => {
        if (stressRunning) {
            clearInterval(interval);
            stressRunning = false;
            console.log('  Client disconnected — stress test stopped');
        }
    });
});

/**
 * Stop an in-progress stress test
 */
app.get('/stop', (req, res) => {
    if (!stressRunning) {
        return res.status(200).type('text/plain').send('Node.js App Memory Stress Test Sample\n' + JSON.stringify({ status: 'No stress test running' }));
    }
    stopRequested = true;
    res.status(200).type('text/plain').send('Node.js App Memory Stress Test Sample\n' + JSON.stringify({ status: 'Stop signal sent' }));
});

/**
 * Clears allocated memory
 */
app.get('/reset', (req, res) => {
    // Explicitly release every Buffer reference before clearing the array
    for (let i = 0; i < memoryHog.length; i++) {
        memoryHog[i] = null;
    }
    memoryHog.length = 0;

    // Force multiple GC passes — Buffers are allocated outside V8 heap
    // and may need several cycles to fully reclaim
    if (global.gc) {
        global.gc();
        global.gc();
    }

    const snapshot = getMemorySnapshot();
    console.log(`  MEMORY RESET — RSS now: ${snapshot.memory_used_mb} MB`);
    res.status(200).type('text/plain').send('Node.js App Memory Stress Test Sample\n' + JSON.stringify({ status: 'Memory cleared', memory: snapshot }));
});

/**
 * Memory usage endpoint for monitoring
 */
app.get('/memory', (req, res) => {
    res.status(200).type('text/plain').send('Node.js App Memory Stress Test Sample\n' + JSON.stringify(getMemorySnapshot()));
});

app.listen(PORT, '0.0.0.0', () => {
    console.log(`Memory Stress Server running on port ${PORT}`);
});
