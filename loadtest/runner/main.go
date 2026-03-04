// loadtest/runner/main.go
//
// Real HTTP load tester — fires actual TCP connections at each server.
// No httptest.ResponseRecorder, no in-process calls. This is wall-clock truth.
//
// Usage:
//
//	go run ./runner [flags]
//
// Flags:
//
//	-duration   test duration per scenario (default 10s)
//	-warmup     warmup duration before measuring (default 2s)
//	-conns      number of concurrent connections (default 100)
//	-servers    comma-separated list: zen,gin,echo,std  (default all)
//
// Each server must already be running on its default port:
//
//	zen  :8081   gin  :8082   echo :8083   std  :8084
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ── config ──────────────────────────────────────────────────────────────────

type serverCfg struct {
	name string
	addr string
}

var servers = []serverCfg{
	{"zen", "http://localhost:8081"},
	{"gin", "http://localhost:8082"},
	{"echo", "http://localhost:8083"},
	{"std", "http://localhost:8084"},
}

type scenario struct {
	name   string
	method string
	path   string
	body   []byte // nil → GET
}

var scenarios = []scenario{
	{
		name:   "ProductCatalog",
		method: "GET",
		path:   "/products",
	},
	{
		name:   "PlaceOrder",
		method: "POST",
		path:   "/orders",
		body:   []byte(`{"product_id":1,"quantity":2,"user_id":"u-999"}`),
	},
	{
		name:   "ConcurrentBurst",
		method: "GET",
		path:   "/ping",
	},
	{
		name:   "LargePayload",
		method: "GET",
		path:   "/large",
	},
}

// ── latency histogram ────────────────────────────────────────────────────────

type histogram struct {
	mu      sync.Mutex
	buckets []int64 // microseconds
}

func newHistogram() *histogram { return &histogram{} }

func (h *histogram) record(d time.Duration) {
	us := d.Microseconds()
	h.mu.Lock()
	h.buckets = append(h.buckets, us)
	h.mu.Unlock()
}

func (h *histogram) percentile(p float64) time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.buckets) == 0 {
		return 0
	}
	sorted := make([]int64, len(h.buckets))
	copy(sorted, h.buckets)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	return time.Duration(sorted[idx]) * time.Microsecond
}

func (h *histogram) mean() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.buckets) == 0 {
		return 0
	}
	var sum int64
	for _, v := range h.buckets {
		sum += v
	}
	return time.Duration(sum/int64(len(h.buckets))) * time.Microsecond
}

func (h *histogram) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.buckets)
}

// ── result ───────────────────────────────────────────────────────────────────

type result struct {
	server   string
	scenario string
	rps      float64
	mean     time.Duration
	p50      time.Duration
	p95      time.Duration
	p99      time.Duration
	errors   int64
	duration time.Duration
	total    int
}

// ── runner ───────────────────────────────────────────────────────────────────

func buildClient(conns int) *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			// Each worker goroutine gets its own persistent connection.
			MaxIdleConnsPerHost:   conns + 10,
			MaxConnsPerHost:       conns + 10,
			IdleConnTimeout:       90 * time.Second,
			DisableCompression:    true, // measure raw throughput
			ResponseHeaderTimeout: 5 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   2 * time.Second,
				KeepAlive: 60 * time.Second,
			}).DialContext,
		},
	}
}

func run(srv serverCfg, sc scenario, client *http.Client,
	concurrency int, warmup, duration time.Duration) result {

	url := srv.addr + sc.path

	var (
		errCount int64
		hist     = newHistogram()
		stop     = make(chan struct{})
	)

	worker := func(measuring bool) {
		for {
			select {
			case <-stop:
				return
			default:
			}

			var reqBody io.Reader
			if sc.body != nil {
				reqBody = bytes.NewReader(sc.body)
			}

			req, err := http.NewRequest(sc.method, url, reqBody)
			if err != nil {
				atomic.AddInt64(&errCount, 1)
				continue
			}
			if sc.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}

			t0 := time.Now()
			resp, err := client.Do(req)
			elapsed := time.Since(t0)

			if err != nil {
				atomic.AddInt64(&errCount, 1)
				continue
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			if resp.StatusCode >= 400 {
				atomic.AddInt64(&errCount, 1)
				continue
			}

			if measuring {
				hist.record(elapsed)
			}
		}
	}

	// warmup
	for i := 0; i < concurrency; i++ {
		go worker(false)
	}
	time.Sleep(warmup)
	close(stop)

	// reset
	stop = make(chan struct{})
	hist = newHistogram()
	errCount = 0

	// real measurement
	t0 := time.Now()
	for i := 0; i < concurrency; i++ {
		go worker(true)
	}
	time.Sleep(duration)
	close(stop)
	// let goroutines drain
	time.Sleep(100 * time.Millisecond)

	elapsed := time.Since(t0)
	total := hist.count()
	rps := float64(total) / elapsed.Seconds()

	return result{
		server:   srv.name,
		scenario: sc.name,
		rps:      rps,
		mean:     hist.mean(),
		p50:      hist.percentile(50),
		p95:      hist.percentile(95),
		p99:      hist.percentile(99),
		errors:   errCount,
		duration: elapsed,
		total:    total,
	}
}

// ── health check ─────────────────────────────────────────────────────────────

func healthCheck(addr string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(addr + "/ping")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// ── printer ──────────────────────────────────────────────────────────────────

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
)

func printResults(results []result) {
	// group by scenario
	byScenario := make(map[string][]result)
	order := []string{}
	for _, r := range results {
		if _, ok := byScenario[r.scenario]; !ok {
			order = append(order, r.scenario)
		}
		byScenario[r.scenario] = append(byScenario[r.scenario], r)
	}

	fmt.Printf("\n%s%s═══════════════════════════════════════════════════════════════════════════════%s\n",
		colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s  REAL HTTP LOAD TEST RESULTS  (actual TCP connections, no mocks)%s\n",
		colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s═══════════════════════════════════════════════════════════════════════════════%s\n\n",
		colorBold, colorCyan, colorReset)

	for _, sc := range order {
		rs := byScenario[sc]
		fmt.Printf("%s%s[ %s ]%s\n", colorBold, colorYellow, sc, colorReset)
		fmt.Printf("  %-8s  %10s  %10s  %10s  %10s  %10s  %8s\n",
			"FRAMEWORK", "RPS", "MEAN", "P50", "P95", "P99", "ERRORS")
		fmt.Printf("  %s\n", strings.Repeat("─", 72))

		// find winner (highest RPS)
		maxRPS := 0.0
		for _, r := range rs {
			if r.rps > maxRPS {
				maxRPS = r.rps
			}
		}

		// sort by RPS desc
		sort.Slice(rs, func(i, j int) bool { return rs[i].rps > rs[j].rps })

		for _, r := range rs {
			winner := "  "
			col := colorReset
			if r.rps == maxRPS {
				winner = "🏆"
				col = colorGreen
			}
			fmt.Printf("  %s%-8s%s  %s%10.0f%s  %10s  %10s  %10s  %10s  %8d\n",
				col, r.server, colorReset,
				col, r.rps, colorReset,
				r.mean.Round(time.Microsecond),
				r.p50.Round(time.Microsecond),
				r.p95.Round(time.Microsecond),
				r.p99.Round(time.Microsecond),
				r.errors,
			)
			_ = winner
		}
		fmt.Println()
	}
}

func printJSON(results []result, path string) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write JSON results: %v\n", err)
		return
	}
	defer f.Close()

	type jsonResult struct {
		Server   string  `json:"server"`
		Scenario string  `json:"scenario"`
		RPS      float64 `json:"rps"`
		MeanUs   int64   `json:"mean_us"`
		P50Us    int64   `json:"p50_us"`
		P95Us    int64   `json:"p95_us"`
		P99Us    int64   `json:"p99_us"`
		Errors   int64   `json:"errors"`
		Total    int     `json:"total"`
	}

	out := make([]jsonResult, len(results))
	for i, r := range results {
		out[i] = jsonResult{
			Server:   r.server,
			Scenario: r.scenario,
			RPS:      r.rps,
			MeanUs:   r.mean.Microseconds(),
			P50Us:    r.p50.Microseconds(),
			P95Us:    r.p95.Microseconds(),
			P99Us:    r.p99.Microseconds(),
			Errors:   r.errors,
			Total:    r.total,
		}
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.Encode(out)
	fmt.Printf("Results written to %s\n", path)
}

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	durFlag := flag.Duration("duration", 10*time.Second, "test duration per scenario")
	warmupFlag := flag.Duration("warmup", 2*time.Second, "warmup duration before measuring")
	connsFlag := flag.Int("conns", 100, "number of concurrent connections")
	serversFlag := flag.String("servers", "zen,gin,echo,std", "comma-separated servers to test")
	outFlag := flag.String("out", "results.json", "path to write JSON results")
	flag.Parse()

	wanted := strings.Split(*serversFlag, ",")
	wantedSet := make(map[string]bool)
	for _, s := range wanted {
		wantedSet[strings.TrimSpace(s)] = true
	}

	var active []serverCfg
	for _, srv := range servers {
		if wantedSet[srv.name] {
			active = append(active, srv)
		}
	}

	// health check
	fmt.Printf("%sChecking servers...%s\n", colorBold, colorReset)
	var ready []serverCfg
	for _, srv := range active {
		if healthCheck(srv.addr) {
			fmt.Printf("  %-8s %s✓ ready%s at %s\n", srv.name, colorGreen, colorReset, srv.addr)
			ready = append(ready, srv)
		} else {
			fmt.Printf("  %-8s %s✗ not reachable%s at %s — skipping\n",
				srv.name, colorRed, colorReset, srv.addr)
		}
	}

	if len(ready) == 0 {
		fmt.Println("No servers reachable. Start them first:")
		fmt.Println("  go run ./cmd/zen_server &")
		fmt.Println("  go run ./cmd/gin_server &")
		fmt.Println("  go run ./cmd/echo_server &")
		fmt.Println("  go run ./cmd/std_server &")
		os.Exit(1)
	}

	fmt.Printf("\n%sConfig:%s  conns=%d  warmup=%s  duration=%s\n\n",
		colorBold, colorReset, *connsFlag, *warmupFlag, *durFlag)

	client := buildClient(*connsFlag)
	var results []result

	total := len(ready) * len(scenarios)
	done := 0
	for _, sc := range scenarios {
		for _, srv := range ready {
			done++
			fmt.Printf("[%d/%d] %-8s  %-20s ...", done, total, srv.name, sc.name)
			r := run(srv, sc, client, *connsFlag, *warmupFlag, *durFlag)
			results = append(results, r)
			fmt.Printf(" %s%.0f rps%s  mean=%s  errs=%d\n",
				colorGreen, r.rps, colorReset,
				r.mean.Round(time.Microsecond), r.errors)
		}
	}

	printResults(results)
	printJSON(results, *outFlag)
}
