/**
 * Memory Stress Test Application - .NET/ASP.NET Core
 * Gradually increases memory usage by a target percentage each second,
 * with detailed monitoring output and OOM handling.
 */

using System.Diagnostics;
using System.Text.Json;

var builder = WebApplication.CreateBuilder(args);

// Configure Kestrel to listen on all interfaces
builder.WebHost.ConfigureKestrel(options =>
{
    options.ListenAnyIP(8080);
});

var app = builder.Build();

// Global list to hold memory allocations (prevents garbage collection)
var memoryHog = new List<byte[]>();
var lockObject = new object();

// Track active stress test so only one runs at a time
var stressRunning = false;
var stressCts = new CancellationTokenSource();

// Helper: get current memory stats (core metrics with friendly names)
static object GetMemorySnapshot()
{
    var process = Process.GetCurrentProcess();
    var gcInfo = GC.GetGCMemoryInfo();
    long totalAvailable = gcInfo.TotalAvailableMemoryBytes;
    long used = GC.GetTotalMemory(false);
    return new
    {
        timestamp = DateTime.UtcNow.ToString("o"),
        memory_used_mb = Math.Round(used / 1024.0 / 1024.0, 2),
        memory_available_mb = Math.Round(totalAvailable / 1024.0 / 1024.0, 2),
        memory_usage_percent = Math.Round(used * 100.0 / totalAvailable, 1),
        total_app_memory_mb = Math.Round(process.WorkingSet64 / 1024.0 / 1024.0, 2)
    };
}

// Health check endpoint for container probes
app.MapGet("/", () => Results.Text(".NET App Memory Stress Test Sample\nHealthy"));

// Triggers gradual memory increase — allocates ~targetPercent% of available memory,
// growing by ~increasePerSecond% each second until the target is reached or OOM occurs.
// Streams memory usage as JSON lines (NDJSON) so the caller can monitor in real-time.
// Query params:
//   ?target=80          target memory usage % (default 80)
//   ?rate=5             % of available memory to add per second (default 5)
app.MapGet("/stress", async (HttpContext ctx) =>
{
    // Parse parameters
    int targetPercent = 80;
    int ratePercent = 5;
    if (int.TryParse(ctx.Request.Query["target"], out var t) && t is > 0 and <= 100) targetPercent = t;
    if (int.TryParse(ctx.Request.Query["rate"], out var r) && r is > 0 and <= 100) ratePercent = r;

    if (stressRunning)
    {
        ctx.Response.StatusCode = 409;
        ctx.Response.ContentType = "application/json";
        await ctx.Response.WriteAsync(JsonSerializer.Serialize(new { status = "error", message = "A stress test is already running. Call /stop first." }));
        return;
    }

    stressRunning = true;
    stressCts = new CancellationTokenSource();
    var token = stressCts.Token;

    // Stream JSON lines as plain text so browsers display inline (not download)
    ctx.Response.StatusCode = 200;
    ctx.Response.ContentType = "text/plain; charset=utf-8";
    ctx.Response.Headers["Cache-Control"] = "no-cache";
    ctx.Response.Headers["X-Accel-Buffering"] = "no";

    Console.WriteLine("══════════════════════════════════════════════════════════════");
    Console.WriteLine($"  MEMORY STRESS STARTED  target={targetPercent}%  rate={ratePercent}%/sec");
    Console.WriteLine("══════════════════════════════════════════════════════════════");

    var sw = Stopwatch.StartNew();
    int totalAllocatedMB = 0;
    int iteration = 0;

    try
    {
        // Disable GC compaction for LOH to maximize memory pressure
        System.Runtime.GCSettings.LargeObjectHeapCompactionMode =
            System.Runtime.GCLargeObjectHeapCompactionMode.Default;

        var gcInfo = GC.GetGCMemoryInfo();
        long availableBytes = gcInfo.TotalAvailableMemoryBytes;
        long targetBytes = (long)(availableBytes * targetPercent / 100.0);
        int chunkSizeMB = Math.Max(1, (int)(availableBytes * ratePercent / 100.0 / 1024 / 1024));
        int chunkSizeBytes = chunkSizeMB * 1024 * 1024;

        // Write banner as the very first line
        await ctx.Response.WriteAsync(".NET App Memory Stress Test Sample\n");
        await ctx.Response.Body.FlushAsync();

        // Write initial info line
        var initLine = JsonSerializer.Serialize(new
        {
            type = "start",
            target_percent = targetPercent,
            rate_percent_per_sec = ratePercent,
            available_mb = availableBytes / 1024 / 1024,
            target_mb = targetBytes / 1024 / 1024,
            chunk_mb = chunkSizeMB
        });
        await ctx.Response.WriteAsync(initLine + "\n");
        await ctx.Response.Body.FlushAsync();

        Console.WriteLine($"  Available memory : {availableBytes / 1024 / 1024} MB");
        Console.WriteLine($"  Target usage     : {targetBytes / 1024 / 1024} MB ({targetPercent}%)");
        Console.WriteLine($"  Chunk per tick   : {chunkSizeMB} MB ({ratePercent}%/sec)");
        Console.WriteLine("──────────────────────────────────────────────────────────────");
        Console.WriteLine($"  {"Tick",-5} {"Elapsed",-10} {"Allocated",-12} {"Heap",-10} {"WorkSet",-10} {"Usage%",-8}");
        Console.WriteLine("──────────────────────────────────────────────────────────────");

        while (!token.IsCancellationRequested)
        {
            iteration++;
            long currentHeap = GC.GetTotalMemory(false);
            if (currentHeap >= targetBytes) break;

            lock (lockObject)
            {
                var chunk = new byte[chunkSizeBytes];
                Array.Fill(chunk, (byte)'X');
                memoryHog.Add(chunk);
                totalAllocatedMB += chunkSizeMB;
            }

            // Collect stats
            var process = Process.GetCurrentProcess();
            long heapNow = GC.GetTotalMemory(false);
            double usagePercent = heapNow * 100.0 / availableBytes;

            // Log to console
            Console.WriteLine($"  {iteration,-5} {sw.Elapsed.TotalSeconds,7:F1}s  {totalAllocatedMB,8} MB  {heapNow / 1024 / 1024,6} MB  {process.WorkingSet64 / 1024 / 1024,6} MB  {usagePercent,6:F1}%");

            // Stream to HTTP response
            var tickLine = JsonSerializer.Serialize(new
            {
                type = "tick",
                tick = iteration,
                elapsed_sec = Math.Round(sw.Elapsed.TotalSeconds, 1),
                allocated_mb = totalAllocatedMB,
                heap_mb = heapNow / 1024 / 1024,
                working_set_mb = process.WorkingSet64 / 1024 / 1024,
                usage_percent = Math.Round(usagePercent, 1),
                target_percent = targetPercent
            });
            await ctx.Response.WriteAsync(tickLine + "\n");
            await ctx.Response.Body.FlushAsync();

            // Wait ~1 second before next allocation
            await Task.Delay(1000, token);
        }

        sw.Stop();
        stressRunning = false;

        Console.WriteLine("──────────────────────────────────────────────────────────────");
        Console.WriteLine($"  STRESS COMPLETE — {totalAllocatedMB} MB allocated in {sw.Elapsed.TotalSeconds:F1}s");
        Console.WriteLine("══════════════════════════════════════════════════════════════");

        var doneLine = JsonSerializer.Serialize(new
        {
            type = "done",
            status = "Memory stress complete",
            target_percent = targetPercent,
            rate_percent_per_sec = ratePercent,
            allocated_mb = totalAllocatedMB,
            iterations = iteration,
            elapsed_seconds = Math.Round(sw.Elapsed.TotalSeconds, 1),
            memory = GetMemorySnapshot()
        });
        await ctx.Response.WriteAsync(doneLine + "\n");
        await ctx.Response.Body.FlushAsync();
    }
    catch (OutOfMemoryException ex)
    {
        sw.Stop();
        stressRunning = false;

        var process = Process.GetCurrentProcess();
        Console.WriteLine("██████████████████████████████████████████████████████████████");
        Console.WriteLine("  *** OUT OF MEMORY EXCEPTION ***");
        Console.WriteLine($"  Time elapsed     : {sw.Elapsed.TotalSeconds:F1}s");
        Console.WriteLine($"  Iteration        : {iteration}");
        Console.WriteLine($"  Allocated before  : {totalAllocatedMB} MB");
        Console.WriteLine($"  Working set      : {process.WorkingSet64 / 1024 / 1024} MB");
        Console.WriteLine($"  GC heap          : {GC.GetTotalMemory(false) / 1024 / 1024} MB");
        Console.WriteLine($"  Exception        : {ex.Message}");
        Console.WriteLine("██████████████████████████████████████████████████████████████");

        var oomLine = JsonSerializer.Serialize(new
        {
            type = "oom",
            status = "Memory allocation failed - OOM",
            error = ex.Message,
            allocated_mb_before_oom = totalAllocatedMB,
            iterations_before_oom = iteration,
            elapsed_seconds = Math.Round(sw.Elapsed.TotalSeconds, 1),
            memory = GetMemorySnapshot()
        });
        await ctx.Response.WriteAsync(oomLine + "\n");
        await ctx.Response.Body.FlushAsync();
    }
    catch (TaskCanceledException)
    {
        sw.Stop();
        stressRunning = false;

        Console.WriteLine("──────────────────────────────────────────────────────────────");
        Console.WriteLine($"  STRESS STOPPED by user — {totalAllocatedMB} MB allocated in {sw.Elapsed.TotalSeconds:F1}s");
        Console.WriteLine("══════════════════════════════════════════════════════════════");

        var stopLine = JsonSerializer.Serialize(new
        {
            type = "stopped",
            status = "Stress test stopped by user",
            allocated_mb = totalAllocatedMB,
            iterations = iteration,
            elapsed_seconds = Math.Round(sw.Elapsed.TotalSeconds, 1),
            memory = GetMemorySnapshot()
        });
        await ctx.Response.WriteAsync(stopLine + "\n");
        await ctx.Response.Body.FlushAsync();
    }
});

// Stop an in-progress stress test
app.MapGet("/stop", () =>
{
    if (!stressRunning)
    {
        var json1 = JsonSerializer.Serialize(new { status = "No stress test running" });
        return Results.Text($".NET App Memory Stress Test Sample\n{json1}");
    }
    stressCts.Cancel();
    var json2 = JsonSerializer.Serialize(new { status = "Stop signal sent" });
    return Results.Text($".NET App Memory Stress Test Sample\n{json2}");
});

// Clears allocated memory and forces garbage collection
app.MapGet("/reset", () =>
{
    lock (lockObject)
    {
        memoryHog.Clear();
    }

    // Force full garbage collection including LOH
    GC.Collect(GC.MaxGeneration, GCCollectionMode.Forced, true, true);
    GC.WaitForPendingFinalizers();
    GC.Collect();

    var snapshot = GetMemorySnapshot();
    Console.WriteLine($"  MEMORY RESET — heap now: {GC.GetTotalMemory(false) / 1024 / 1024} MB");

    var json = JsonSerializer.Serialize(new { status = "Memory cleared", memory = snapshot });
    return Results.Text($".NET App Memory Stress Test Sample\n{json}");
});

// Memory usage endpoint for monitoring
app.MapGet("/memory", () =>
{
    var json = JsonSerializer.Serialize(GetMemorySnapshot());
    return Results.Text($".NET App Memory Stress Test Sample\n{json}");
});

app.Run();
