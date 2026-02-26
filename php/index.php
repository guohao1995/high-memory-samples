<?php
/**
 * Memory Stress Test Application - PHP/Apache
 * Gradually increases memory usage by a target percentage each second,
 * with detailed monitoring output and OOM handling.
 */

// Allow up to 2 GB of memory
ini_set('memory_limit', '2G');
// Disable execution time limit
set_time_limit(0);
// Disable output buffering for streaming
ini_set('output_buffering', 'Off');
ini_set('zlib.output_compression', 'Off');
ini_set('implicit_flush', 1);

// Global array to hold memory allocations
$GLOBALS['memoryHog'] = [];

// Stop signal file (shared between requests)
define('STOP_FILE', '/tmp/memory_stress_stop');
define('RUNNING_FILE', '/tmp/memory_stress_running');

/**
 * Simple router
 */
$requestUri = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);

switch ($requestUri) {
    case '/':
        handleHealth();
        break;
    case '/stress':
        handleStress();
        break;
    case '/stop':
        handleStop();
        break;
    case '/reset':
        handleReset();
        break;
    case '/memory':
        handleMemory();
        break;
    default:
        http_response_code(404);
        header('Content-Type: application/json');
        echo json_encode(['error' => 'Not Found']);
        break;
}

/**
 * Returns core memory metrics with friendly names.
 */
function getMemorySnapshot() {
    // Prefer cgroup usage (real container RSS) over PHP's internal counter
    $cgroupUsage = readCgroupMemoryUsage();
    $usedBytes = $cgroupUsage > 0 ? $cgroupUsage : memory_get_usage(true);
    $limitBytes = getAvailableBytes();
    $usedMB = round($usedBytes / 1024 / 1024, 2);
    $availMB = round($limitBytes / 1024 / 1024, 2);
    $pct = $limitBytes > 0 ? round($usedBytes / $limitBytes * 100, 1) : 0;

    return [
        'timestamp' => gmdate('Y-m-d\TH:i:s.000\Z'),
        'memory_used_mb' => $usedMB,
        'memory_available_mb' => $availMB,
        'memory_usage_percent' => $pct,
        'total_app_memory_mb' => $usedMB,
    ];
}

/**
 * Parse PHP shorthand memory notation (e.g. 2G, 512M).
 */
function returnBytes(string $val): int {
    $val = trim($val);
    $last = strtolower($val[strlen($val) - 1]);
    $num = (int) $val;
    switch ($last) {
        case 'g': $num *= 1024;
        case 'm': $num *= 1024;
        case 'k': $num *= 1024;
    }
    return $num;
}

/**
 * Read container memory limit from cgroup v1 or v2.
 * Returns 0 if not running in a container or limit is not set.
 */
function readCgroupMemoryLimit(): int {
    $paths = [
        '/sys/fs/cgroup/memory/memory.limit_in_bytes', // cgroup v1
        '/sys/fs/cgroup/memory.max',                    // cgroup v2
    ];
    foreach ($paths as $p) {
        if (!is_readable($p)) continue;
        $val = trim(file_get_contents($p));
        if ($val === 'max') continue;
        $limit = intval($val);
        if ($limit > 0 && $limit < (1 << 60)) return $limit;
    }
    return 0;
}

/**
 * Read actual container memory usage from cgroup v1 or v2.
 * This reflects the real RSS of the entire container (Apache + PHP + data),
 * not just PHP's internal allocator.
 * Returns 0 if not available.
 */
function readCgroupMemoryUsage(): int {
    $paths = [
        '/sys/fs/cgroup/memory/memory.usage_in_bytes', // cgroup v1
        '/sys/fs/cgroup/memory.current',                // cgroup v2
    ];
    foreach ($paths as $p) {
        if (!is_readable($p)) continue;
        $val = trim(file_get_contents($p));
        $usage = intval($val);
        if ($usage > 0) return $usage;
    }
    return 0;
}

/**
 * Get available memory bytes — prefers cgroup limit, falls back to PHP memory_limit.
 */
function getAvailableBytes(): int {
    $cgroupLimit = readCgroupMemoryLimit();
    $phpLimit = returnBytes(ini_get('memory_limit'));
    if ($cgroupLimit > 0) return $cgroupLimit;
    return $phpLimit;
}

/**
 * Write a JSON line and flush immediately.
 */
function writeLine(array $data): void {
    echo json_encode($data) . "\n";
    if (ob_get_level()) {
        ob_flush();
    }
    flush();
}

/**
 * Health check endpoint.
 */
function handleHealth() {
    http_response_code(200);
    header('Content-Type: text/plain');
    echo "PHP App Memory Stress Test Sample\nHealthy";
}

/**
 * Triggers gradual memory increase. Streams JSON lines (text/plain).
 * Query params:
 *   ?target=80  target memory usage % (default 80)
 *   ?rate=5     % of available memory to add per second (default 5)
 */
function handleStress() {
    $targetPercent = isset($_GET['target']) ? intval($_GET['target']) : 80;
    $ratePercent = isset($_GET['rate']) ? intval($_GET['rate']) : 5;
    if ($targetPercent < 1 || $targetPercent > 100) $targetPercent = 80;
    if ($ratePercent < 1 || $ratePercent > 100) $ratePercent = 5;

    if (file_exists(RUNNING_FILE)) {
        http_response_code(409);
        header('Content-Type: application/json');
        echo json_encode(['status' => 'error', 'message' => 'A stress test is already running. Call /stop first.']);
        return;
    }

    file_put_contents(RUNNING_FILE, '1');
    @unlink(STOP_FILE);

    // Stream plain text
    http_response_code(200);
    header('Content-Type: text/plain; charset=utf-8');
    header('Cache-Control: no-cache');
    header('X-Accel-Buffering: no');

    // Clear any existing output buffer
    while (ob_get_level()) {
        ob_end_flush();
    }

    $limitBytes = getAvailableBytes();
    $targetBytes = (int) ($limitBytes * $targetPercent / 100);
    $chunkSizeMB = max(1, (int) ($limitBytes * $ratePercent / 100 / 1024 / 1024));
    $chunkSizeBytes = $chunkSizeMB * 1024 * 1024;

    error_log("══════════════════════════════════════════════════════════════");
    error_log("  MEMORY STRESS STARTED  target={$targetPercent}%  rate={$ratePercent}%/sec");
    error_log("══════════════════════════════════════════════════════════════");
    error_log("  Available memory : " . ($limitBytes / 1024 / 1024) . " MB");
    error_log("  Target usage     : " . ($targetBytes / 1024 / 1024) . " MB ({$targetPercent}%)");
    error_log("  Chunk per tick   : {$chunkSizeMB} MB ({$ratePercent}%/sec)");

    echo "PHP App Memory Stress Test Sample\n";
    flush();

    writeLine([
        'type' => 'start',
        'target_percent' => $targetPercent,
        'rate_percent_per_sec' => $ratePercent,
        'available_mb' => (int) ($limitBytes / 1024 / 1024),
        'target_mb' => (int) ($targetBytes / 1024 / 1024),
        'chunk_mb' => $chunkSizeMB,
    ]);

    $startTime = microtime(true);
    $totalAllocatedMB = 0;
    $iteration = 0;

    while (true) {
        // Check stop signal
        if (file_exists(STOP_FILE)) {
            @unlink(STOP_FILE);
            @unlink(RUNNING_FILE);
            $elapsed = round(microtime(true) - $startTime, 1);
            error_log("  STRESS STOPPED by user — {$totalAllocatedMB} MB in {$elapsed}s");
            writeLine([
                'type' => 'stopped',
                'status' => 'Stress test stopped by user',
                'allocated_mb' => $totalAllocatedMB,
                'iterations' => $iteration,
                'elapsed_seconds' => $elapsed,
                'memory' => getMemorySnapshot(),
            ]);
            return;
        }

        $iteration++;

        try {
            $chunk = str_repeat('X', $chunkSizeBytes);
            $GLOBALS['memoryHog'][] = $chunk;
            $totalAllocatedMB += $chunkSizeMB;
        } catch (\Error $e) {
            // PHP 8 throws Error on OOM
            @unlink(RUNNING_FILE);
            $elapsed = round(microtime(true) - $startTime, 1);
            error_log("██████████████████████████████████████████████████████████████");
            error_log("  *** OUT OF MEMORY ***");
            error_log("  Iteration: {$iteration}  Allocated: {$totalAllocatedMB} MB  Elapsed: {$elapsed}s");
            error_log("  Error: " . $e->getMessage());
            error_log("██████████████████████████████████████████████████████████████");
            writeLine([
                'type' => 'oom',
                'status' => 'Memory allocation failed - OOM',
                'error' => $e->getMessage(),
                'allocated_mb_before_oom' => $totalAllocatedMB,
                'iterations_before_oom' => $iteration,
                'elapsed_seconds' => $elapsed,
                'memory' => getMemorySnapshot(),
            ]);
            return;
        }

        // Read actual container RSS from cgroup for accurate OOM detection
        $cgroupUsed = readCgroupMemoryUsage();
        $currentUsed = $cgroupUsed > 0 ? $cgroupUsed : memory_get_usage(true);
        $usagePct = $limitBytes > 0 ? round($currentUsed / $limitBytes * 100, 1) : 0;

        // OOM detection: when memory reaches >=99% of container limit, report OOM and stop.
        // The kernel OOM killer sends SIGKILL (uncatchable), so we detect
        // the threshold and report before the kernel acts.
        if ($usagePct >= 99.0 && $targetPercent === 100) {
            @unlink(RUNNING_FILE);
            $elapsed = round(microtime(true) - $startTime, 1);
            error_log("██████████████████████████████████████████████████████████████");
            error_log("  *** OUT OF MEMORY — usage at {$usagePct}% of container limit ***");
            error_log("  Iteration: {$iteration}  Allocated: {$totalAllocatedMB} MB  Elapsed: {$elapsed}s");
            error_log("██████████████████████████████████████████████████████████████");
            writeLine([
                'type' => 'oom',
                'status' => 'Memory allocation failed - OOM',
                'error' => "Memory usage reached {$usagePct}% of container memory limit (" . round($limitBytes / 1024 / 1024) . " MB)",
                'allocated_mb_before_oom' => $totalAllocatedMB,
                'iterations_before_oom' => $iteration,
                'elapsed_seconds' => $elapsed,
                'memory' => getMemorySnapshot(),
            ]);
            return;
        }

        // For target < 100%, stop when we reach the target
        if ($targetPercent < 100 && $currentUsed >= $targetBytes) {
            @unlink(RUNNING_FILE);
            $elapsed = round(microtime(true) - $startTime, 1);
            error_log("  STRESS COMPLETE — {$totalAllocatedMB} MB in {$elapsed}s");
            writeLine([
                'type' => 'done',
                'status' => 'Memory stress complete',
                'target_percent' => $targetPercent,
                'rate_percent_per_sec' => $ratePercent,
                'allocated_mb' => $totalAllocatedMB,
                'iterations' => $iteration,
                'elapsed_seconds' => $elapsed,
                'memory' => getMemorySnapshot(),
            ]);
            return;
        }
        $elapsed = round(microtime(true) - $startTime, 1);

        error_log("  {$iteration}  {$elapsed}s  {$totalAllocatedMB} MB  used=" . round($currentUsed / 1024 / 1024) . " MB  {$usagePct}%");

        writeLine([
            'type' => 'tick',
            'tick' => $iteration,
            'elapsed_sec' => $elapsed,
            'allocated_mb' => $totalAllocatedMB,
            'used_mb' => round($currentUsed / 1024 / 1024),
            'usage_percent' => $usagePct,
            'target_percent' => $targetPercent,
        ]);

        sleep(1);
    }
}

/**
 * Stop signal — creates a temp file that the stress loop checks.
 */
function handleStop() {
    header('Content-Type: text/plain; charset=utf-8');
    if (!file_exists(RUNNING_FILE)) {
        http_response_code(200);
        echo "PHP App Memory Stress Test Sample\n" . json_encode(['status' => 'No stress test running']);
        return;
    }
    file_put_contents(STOP_FILE, '1');
    http_response_code(200);
    echo "PHP App Memory Stress Test Sample\n" . json_encode(['status' => 'Stop signal sent']);
}

/**
 * Clears allocated memory.
 */
function handleReset() {
    header('Content-Type: text/plain; charset=utf-8');
    $GLOBALS['memoryHog'] = [];
    gc_collect_cycles();

    $snapshot = getMemorySnapshot();
    error_log("  MEMORY RESET — used now: {$snapshot['memory_used_mb']} MB");

    http_response_code(200);
    echo "PHP App Memory Stress Test Sample\n" . json_encode(['status' => 'Memory cleared', 'memory' => $snapshot]);
}

/**
 * Memory usage endpoint for monitoring.
 */
function handleMemory() {
    header('Content-Type: text/plain; charset=utf-8');
    http_response_code(200);
    echo "PHP App Memory Stress Test Sample\n" . json_encode(getMemorySnapshot());
}
