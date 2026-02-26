"""
Memory Stress Test Application - Python/Flask
Gradually increases memory usage by a target percentage each second,
with detailed monitoring output and OOM handling.
"""

import gc
import json
import os
import resource
import time
import threading

from flask import Flask, Response, jsonify, request, stream_with_context

app = Flask(__name__)

# Global list to hold memory allocations (prevents garbage collection)
memory_hog = []

# Track active stress test
stress_running = False
stop_requested = False
_lock = threading.Lock()


def get_memory_snapshot():
    """Returns core memory metrics with friendly names."""
    try:
        # Read RSS from /proc/self/status for accurate container memory usage
        with open("/proc/self/status") as f:
            status = f.read()
        vm_rss = 0
        for line in status.splitlines():
            if line.startswith("VmRSS:"):
                vm_rss = int(line.split()[1])  # kB

        used_bytes = vm_rss * 1024
        used_mb = round(used_bytes / 1024 / 1024, 2)

        # Use cgroup limit as available memory (real container limit)
        cgroup_limit = _read_cgroup_memory_limit()
        if cgroup_limit:
            avail_bytes = cgroup_limit
        else:
            avail_bytes = used_bytes  # fallback
        avail_mb = round(avail_bytes / 1024 / 1024, 2)
        usage_pct = round(used_bytes / avail_bytes * 100, 1) if avail_bytes > 0 else 0

        return {
            "timestamp": time.strftime("%Y-%m-%dT%H:%M:%S.000Z", time.gmtime()),
            "memory_used_mb": used_mb,
            "memory_available_mb": avail_mb,
            "memory_usage_percent": usage_pct,
            "total_app_memory_mb": used_mb,
        }
    except Exception:
        # Fallback for non-Linux
        import psutil
        proc = psutil.Process(os.getpid())
        mem = proc.memory_info()
        used_mb = round(mem.rss / 1024 / 1024, 2)
        return {
            "timestamp": time.strftime("%Y-%m-%dT%H:%M:%S.000Z", time.gmtime()),
            "memory_used_mb": used_mb,
            "memory_available_mb": used_mb,
            "memory_usage_percent": 0,
            "total_app_memory_mb": used_mb,
        }


def _read_cgroup_memory_limit():
    """Read container memory limit from cgroup (v1 or v2)."""
    for path in [
        "/sys/fs/cgroup/memory/memory.limit_in_bytes",  # cgroup v1
        "/sys/fs/cgroup/memory.max",                      # cgroup v2
    ]:
        try:
            with open(path) as f:
                val = f.read().strip()
                if val == "max":
                    return None
                limit = int(val)
                if limit < 2**60:  # ignore unreasonably large (= unlimited)
                    return limit
        except Exception:
            continue
    return None


@app.route("/")
def health():
    """Health check endpoint for container probes."""
    return "Python App Memory Stress Test Sample\nHealthy", 200


@app.route("/stress")
def stress():
    """
    Triggers gradual memory increase.
    Streams JSON lines (text/plain) so browsers display inline.
    Query params:
      ?target=80  target memory usage % (default 80)
      ?rate=5     % of available memory to add per second (default 5)
    """
    global stress_running, stop_requested

    target_percent = request.args.get("target", 80, type=int)
    rate_percent = request.args.get("rate", 5, type=int)
    if target_percent < 1 or target_percent > 100:
        target_percent = 80
    if rate_percent < 1 or rate_percent > 100:
        rate_percent = 5

    with _lock:
        if stress_running:
            return jsonify({"status": "error", "message": "A stress test is already running. Call /stop first."}), 409
        stress_running = True
        stop_requested = False

    # Disable GC for maximum retention
    gc.disable()

    def generate():
        global stress_running, stop_requested

        try:
            # Determine available memory
            cgroup_limit = _read_cgroup_memory_limit()
            if cgroup_limit:
                available_bytes = cgroup_limit
            else:
                try:
                    with open("/proc/meminfo") as f:
                        for line in f:
                            if line.startswith("MemTotal:"):
                                available_bytes = int(line.split()[1]) * 1024
                                break
                except Exception:
                    available_bytes = 2 * 1024 * 1024 * 1024  # fallback 2GB

            target_bytes = int(available_bytes * target_percent / 100)
            chunk_size_mb = max(1, int(available_bytes * rate_percent / 100 / 1024 / 1024))
            chunk_size_bytes = chunk_size_mb * 1024 * 1024

            yield "Python App Memory Stress Test Sample\n"

            start_line = json.dumps({
                "type": "start",
                "target_percent": target_percent,
                "rate_percent_per_sec": rate_percent,
                "available_mb": available_bytes // (1024 * 1024),
                "target_mb": target_bytes // (1024 * 1024),
                "chunk_mb": chunk_size_mb,
            })
            yield start_line + "\n"

            print("=" * 62)
            print(f"  MEMORY STRESS STARTED  target={target_percent}%  rate={rate_percent}%/sec")
            print("=" * 62)
            print(f"  Available memory : {available_bytes // (1024 * 1024)} MB")
            print(f"  Target usage     : {target_bytes // (1024 * 1024)} MB ({target_percent}%)")
            print(f"  Chunk per tick   : {chunk_size_mb} MB ({rate_percent}%/sec)")

            start_time = time.time()
            total_allocated_mb = 0
            iteration = 0

            while True:
                # Check stop
                if stop_requested:
                    elapsed = time.time() - start_time
                    print(f"  STRESS STOPPED by user — {total_allocated_mb} MB in {elapsed:.1f}s")
                    stop_line = json.dumps({
                        "type": "stopped",
                        "status": "Stress test stopped by user",
                        "allocated_mb": total_allocated_mb,
                        "iterations": iteration,
                        "elapsed_seconds": round(elapsed, 1),
                        "memory": get_memory_snapshot(),
                    })
                    yield stop_line + "\n"
                    return

                # Allocate FIRST, then check — so we detect threshold after allocation
                iteration += 1
                chunk = b"X" * chunk_size_bytes
                memory_hog.append(chunk)
                total_allocated_mb += chunk_size_mb

                # Read current RSS after allocation
                try:
                    with open("/proc/self/status") as f:
                        status = f.read()
                    vm_rss_kb = 0
                    for line in status.splitlines():
                        if line.startswith("VmRSS:"):
                            vm_rss_kb = int(line.split()[1])
                            break
                    current_rss = vm_rss_kb * 1024
                except Exception:
                    current_rss = total_allocated_mb * 1024 * 1024

                usage_pct = current_rss / available_bytes * 100 if available_bytes > 0 else 0

                # OOM detection: when RSS reaches >=99% of container memory, report OOM and stop.
                # The kernel OOM killer sends SIGKILL (uncatchable), so we detect
                # the threshold and report before the kernel acts.
                if usage_pct >= 99.0 and target_percent == 100:
                    elapsed = time.time() - start_time
                    print("█" * 62)
                    print(f"  *** OUT OF MEMORY — RSS at {usage_pct:.1f}% of container limit ***")
                    print(f"  Iteration: {iteration}  Allocated: {total_allocated_mb} MB  RSS: {current_rss // (1024*1024)} MB  Elapsed: {elapsed:.1f}s")
                    print("█" * 62)
                    oom_line = json.dumps({
                        "type": "oom",
                        "status": "Memory allocation failed - OOM",
                        "error": f"RSS reached {usage_pct:.1f}% of container memory limit ({available_bytes // (1024*1024)} MB)",
                        "allocated_mb_before_oom": total_allocated_mb,
                        "iterations_before_oom": iteration,
                        "elapsed_seconds": round(elapsed, 1),
                        "rss_mb": current_rss // (1024 * 1024),
                        "usage_percent": round(usage_pct, 1),
                        "memory": get_memory_snapshot(),
                    })
                    yield oom_line + "\n"
                    return

                # For target < 100%, stop when we reach the target
                if target_percent < 100 and current_rss >= target_bytes:
                    elapsed = time.time() - start_time
                    print(f"  STRESS COMPLETE — {total_allocated_mb} MB in {elapsed:.1f}s")
                    done_line = json.dumps({
                        "type": "done",
                        "status": "Memory stress complete",
                        "target_percent": target_percent,
                        "rate_percent_per_sec": rate_percent,
                        "allocated_mb": total_allocated_mb,
                        "iterations": iteration,
                        "elapsed_seconds": round(elapsed, 1),
                        "memory": get_memory_snapshot(),
                    })
                    yield done_line + "\n"
                    return

                elapsed = time.time() - start_time

                print(f"  {iteration}  {elapsed:.1f}s  {total_allocated_mb} MB  rss={current_rss // (1024 * 1024)} MB  {usage_pct:.1f}%")

                tick_line = json.dumps({
                    "type": "tick",
                    "tick": iteration,
                    "elapsed_sec": round(elapsed, 1),
                    "allocated_mb": total_allocated_mb,
                    "rss_mb": current_rss // (1024 * 1024),
                    "usage_percent": round(usage_pct, 1),
                    "target_percent": target_percent,
                })
                yield tick_line + "\n"

                time.sleep(1)

        except MemoryError as e:
            elapsed = time.time() - start_time
            print("█" * 62)
            print("  *** OUT OF MEMORY ***")
            print(f"  Iteration: {iteration}  Allocated: {total_allocated_mb} MB  Elapsed: {elapsed:.1f}s")
            print(f"  Error: {e}")
            print("█" * 62)
            oom_line = json.dumps({
                "type": "oom",
                "status": "Memory allocation failed - OOM",
                "error": str(e),
                "allocated_mb_before_oom": total_allocated_mb,
                "iterations_before_oom": iteration,
                "elapsed_seconds": round(elapsed, 1),
                "memory": get_memory_snapshot(),
            })
            yield oom_line + "\n"

        finally:
            with _lock:
                stress_running = False

    return Response(
        stream_with_context(generate()),
        mimetype="text/plain; charset=utf-8",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
        },
    )


@app.route("/stop")
def stop():
    """Signal the running stress test to stop."""
    global stop_requested
    if not stress_running:
        return "Python App Memory Stress Test Sample\n" + json.dumps({"status": "No stress test running"}), 200, {"Content-Type": "text/plain; charset=utf-8"}
    stop_requested = True
    return "Python App Memory Stress Test Sample\n" + json.dumps({"status": "Stop signal sent"}), 200, {"Content-Type": "text/plain; charset=utf-8"}


@app.route("/reset")
def reset():
    """Clears allocated memory and re-enables garbage collection."""
    global memory_hog
    memory_hog.clear()
    gc.enable()
    gc.collect()
    snapshot = get_memory_snapshot()
    print(f"  MEMORY RESET — used now: {snapshot['memory_used_mb']} MB")
    return "Python App Memory Stress Test Sample\n" + json.dumps({"status": "Memory cleared", "memory": snapshot}), 200, {"Content-Type": "text/plain; charset=utf-8"}


@app.route("/memory")
def memory():
    """Memory usage endpoint for monitoring."""
    return "Python App Memory Stress Test Sample\n" + json.dumps(get_memory_snapshot()), 200, {"Content-Type": "text/plain; charset=utf-8"}


if __name__ == "__main__":
    port = int(os.environ.get("PORT", 8080))
    app.run(host="0.0.0.0", port=port)
