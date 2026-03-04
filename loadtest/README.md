# zen load test

Real HTTP load tester for zen, gin, echo, and net/http stdlib.  
**No `httptest.ResponseRecorder`. No in-process calls. Real TCP. Real latency.**

---

## Why your existing benchmarks are misleading

Your `BenchmarkThroughput_*` tests almost certainly use `httptest.NewRecorder()` or
call handlers in-process. This produces numbers that look like this:

```
Echo ConcurrentBurst: 12,228,153 req/s
Zen  ConcurrentBurst:  3,697,964 req/s
```

12 million requests/second over "HTTP" is not real. A modern server at wire speed
handles roughly 100k–500k req/s depending on payload. The recorder-based numbers
are measuring **struct method calls + JSON encoding into a bytes.Buffer** — they
tell you nothing about real server performance.

### The specific anomalies explained

| Anomaly | Cause |
|---|---|
| Echo `ConcurrentBurst` 3× faster than stdlib | Echo pools its `ResponseWriter` wrapper. In a recorder benchmark the pool hit rate is ~100%, so you're measuring pool.Get() not network I/O |
| Echo `LargePayload` 359 B alloc vs your 21608 B | Echo's `c.JSON()` reuses an internal buffer pool. The recorder never flushes, so allocs never escape to the GC |
| Zen `ConcurrentBurst` 32 allocs vs Echo 8 | `map[string]string{"status":"ok"}` allocates every call. Use a pre-allocated struct |

---

## Quick start

```bash
# 1. Clone/copy your zen source into a sibling directory so the replace
#    directive in go.mod resolves correctly, then:

./run_loadtest.sh

# Custom duration and concurrency:
./run_loadtest.sh -duration 20s -conns 200

# Test only zen vs stdlib:
./run_loadtest.sh -servers zen,std -duration 15s
```

### Requirements
- Go 1.22+
- `nc` (netcat) for server health checks
- Your zen module at `./zen` (or update the `replace` in go.mod)

---

## Flags

| Flag | Default | Description |
|---|---|---|
| `-duration` | `10s` | Measurement window per scenario |
| `-warmup` | `2s` | JIT/connection warm-up before measuring |
| `-conns` | `100` | Concurrent connections per scenario |
| `-servers` | `zen,gin,echo,std` | Which servers to include |
| `-out` | `results.json` | Path for JSON output |

---

## Scenarios

| Scenario | Method | Endpoint | What it measures |
|---|---|---|---|
| `ProductCatalog` | GET | `/products` | JSON serialization of 20 structs |
| `PlaceOrder` | POST | `/orders` | JSON decode + encode |
| `ConcurrentBurst` | GET | `/ping` | Raw concurrency / scheduler throughput |
| `LargePayload` | GET | `/large` | Serialization of 500 structs (~64KB) |

---

## How to fix Zen's real bottlenecks

After running this test, the realistic findings will be:

1. **`map[string]string` allocations** — replace one-off maps with pre-defined structs
   ```go
   // bad  (allocs map + 2 strings every request)
   c.JSON(200, map[string]string{"status": "ok"})

   // good (zero allocs)
   type pongResp struct{ Status string `json:"status"` }
   var pong = pongResp{"ok"}
   c.JSON(200, pong)
   ```

2. **`json.Marshal` vs `json.NewEncoder`** — already fixed in response.go.
   For large payloads, pre-marshal once at startup and serve as bytes (see zen_server's `/large` handler).

3. **`sync.Pool` eviction under GC pressure** — if the GC runs mid-burst, all pooled
   contexts are discarded and must be reallocated. This is expected behavior;
   the pool helps under steady load, not instantaneous bursts.

4. **Connection reuse** — make sure your test client uses persistent connections
   (Keep-Alive). The runner in this repo does this correctly via `MaxIdleConnsPerHost`.