// Loadtest is a concurrent HTTP load testing tool that measures throughput,
// latency percentiles, and backend distribution for load balancer testing.
//
// Usage:
//
//	go run loadtest.go -url http://localhost:8080/create-course -concurrency 10 -requests 1000
//	go run loadtest.go -url http://localhost:8080 -concurrency 50 -requests 5000 -csv results.csv -out summary.json
//
// Features:
//   - Concurrent workers for high throughput testing
//   - Per-backend latency and distribution statistics
//   - CSV output with per-request details
//   - JSON summary with percentiles (p50, p90, p95, p99)
//   - Fake IP distribution via X-Forwarded-For header for IP-hash strategy testing
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	var (
		url         = flag.String("url", "http://localhost:8080/create-course", "Target URL")
		concurrency = flag.Int("concurrency", 10, "Number of concurrent workers")
		requests    = flag.Int("requests", 100, "Total number of requests to send")
		method      = flag.String("method", "POST", "HTTP method")
		body        = flag.String("body", `{"title":"T","description":"d"}`, "Request body")
		contentType = flag.String("content-type", "application/json", "Content-Type header")
		timeoutSec  = flag.Int("timeout", 10, "Per-request timeout in seconds")
	)

	outJSON := flag.String("out", "", "Write JSON summary to this file (optional)")
	outCSV := flag.String("csv", "", "Write per-request CSV to this file (optional)")
	verbose := flag.Bool("v", false, "Verbose per-request logging to stdout")
	flag.Parse()

	client := &http.Client{Timeout: time.Duration(*timeoutSec) * time.Second}

	jobs := make(chan int)
	var wg sync.WaitGroup

	var total int32
	var success int32
	var failure int32

	// BackendStats tracks statistics for a specific backend server.
	type BackendStats struct {
		Count     int32           `json:"count"`
		Success   int32           `json:"success"`
		Failure   int32           `json:"failure"`
		Latencies []time.Duration `json:"-"`
	}

	backendStats := make(map[string]*BackendStats)
	var backendMu sync.Mutex

	var allLatencies []time.Duration
	var latMu sync.Mutex

	statusCodes := make(map[int]int32)
	var statusMu sync.Mutex

	// open CSV if requested
	var csvFile *os.File
	var csvWriter *csv.Writer
	var csvMu sync.Mutex
	if *outCSV != "" {
		f, err := os.Create(*outCSV)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create csv file: %v\n", err)
			os.Exit(1)
		}
		csvFile = f
		csvWriter = csv.NewWriter(f)
		// header
		csvWriter.Write([]string{"idx", "timestamp", "backend", "status", "duration_ms"})
	}

	testStart := time.Now()

	// worker
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for idx := range jobs {
				atomic.AddInt32(&total, 1)
				start := time.Now()

				req, err := http.NewRequest(*method, *url, bytes.NewBufferString(*body))
				if err != nil {
					atomic.AddInt32(&failure, 1)
					continue
				}
				req.Header.Set("Content-Type", *contentType)

				// Fake different source IPs using X-Forwarded-For header
				fakeIP := fmt.Sprintf("192.168.1.%d", (idx%50)+1)
				req.Header.Set("X-Forwarded-For", fakeIP)

				resp, err := client.Do(req)
				dur := time.Since(start)

				// record overall latency
				latMu.Lock()
				allLatencies = append(allLatencies, dur)
				latMu.Unlock()

				if err != nil {
					atomic.AddInt32(&failure, 1)
					if *verbose {
						fmt.Printf("[%d] idx=%d error=%v\n", workerID, idx, err)
					}
					continue
				}

				// status code map
				statusMu.Lock()
				statusCodes[resp.StatusCode]++
				statusMu.Unlock()

				if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
					atomic.AddInt32(&success, 1)
				} else {
					atomic.AddInt32(&failure, 1)
				}

				backend := resp.Header.Get("X-Backend-Server")
				if backend == "" {
					backend = "(unknown)"
				}

				backendMu.Lock()
				bs, ok := backendStats[backend]
				if !ok {
					bs = &BackendStats{}
					backendStats[backend] = bs
				}
				bs.Count++
				if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
					bs.Success++
				} else {
					bs.Failure++
				}
				bs.Latencies = append(bs.Latencies, dur)
				backendMu.Unlock()

				// optional CSV row and verbose
				if csvWriter != nil {
					csvMu.Lock()
					csvWriter.Write([]string{
						fmt.Sprintf("%d", idx),
						time.Now().Format(time.RFC3339Nano),
						backend,
						fmt.Sprintf("%d", resp.StatusCode),
						fmt.Sprintf("%.3f", float64(dur.Microseconds())/1000.0),
					})
					csvMu.Unlock()
				}

				if *verbose {
					fmt.Printf("[%d] idx=%d backend=%s status=%d dur=%v\n", workerID, idx, backend, resp.StatusCode, dur)
				}

				// drain body
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}(i)
	}

	// send jobs
	go func() {
		for i := 0; i < *requests; i++ {
			jobs <- i
		}
		close(jobs)
	}()

	wg.Wait()
	testEnd := time.Now()

	if csvWriter != nil {
		csvWriter.Flush()
		csvFile.Close()
	}

	// summarize
	totalDuration := testEnd.Sub(testStart)
	throughput := float64(total) / totalDuration.Seconds()

	fmt.Println("--- Load Test Summary ---")
	fmt.Printf("Target: %s\n", *url)
	fmt.Printf("Requests: %d  Concurrency: %d\n", *requests, *concurrency)
	fmt.Printf("Total sent: %d  Success: %d  Failure: %d\n", total, success, failure)
	fmt.Printf("Duration: %v  Throughput: %.2f req/s\n", totalDuration, throughput)

	// status codes
	fmt.Println("\nStatus codes:")
	statusMu.Lock()
	var scKeys []int
	for k := range statusCodes {
		scKeys = append(scKeys, k)
	}
	sort.Ints(scKeys)
	for _, k := range scKeys {
		fmt.Printf("  %d -> %d\n", k, statusCodes[k])
	}
	statusMu.Unlock()

	// backends
	fmt.Println("\nBackend distribution & stats:")
	backendMu.Lock()
	var backendKeys []string
	for k := range backendStats {
		backendKeys = append(backendKeys, k)
	}
	sort.Strings(backendKeys)
	for _, k := range backendKeys {
		bs := backendStats[k]
		// compute latency stats for this backend
		var min, max time.Duration
		var sum time.Duration
		latCount := len(bs.Latencies)
		if latCount > 0 {
			min = bs.Latencies[0]
			for _, d := range bs.Latencies {
				if d < min {
					min = d
				}
				if d > max {
					max = d
				}
				sum += d
			}
		}
		var avg time.Duration
		if latCount > 0 {
			avg = sum / time.Duration(latCount)
		}

		// percentiles
		var p50, p90, p95, p99 time.Duration
		if latCount > 0 {
			// make a copy and sort
			tmp := make([]time.Duration, latCount)
			copy(tmp, bs.Latencies)
			sort.Slice(tmp, func(i, j int) bool { return tmp[i] < tmp[j] })
			p := func(pct float64) time.Duration {
				idx := int(float64(len(tmp)-1) * pct)
				if idx < 0 {
					idx = 0
				}
				if idx >= len(tmp) {
					idx = len(tmp) - 1
				}
				return tmp[idx]
			}
			p50 = p(0.50)
			p90 = p(0.90)
			p95 = p(0.95)
			p99 = p(0.99)
		}

		fmt.Printf("  %s -> total=%d success=%d failure=%d\n", k, bs.Count, bs.Success, bs.Failure)
		if latCount > 0 {
			fmt.Printf("    latencies: samples=%d min=%v avg=%v max=%v p50=%v p90=%v p95=%v p99=%v\n",
				latCount, min, avg, max, p50, p90, p95, p99)
		}
	}
	backendMu.Unlock()

	// overall latencies
	if len(allLatencies) > 0 {
		tmp := make([]time.Duration, len(allLatencies))
		copy(tmp, allLatencies)
		sort.Slice(tmp, func(i, j int) bool { return tmp[i] < tmp[j] })
		var sum time.Duration
		for _, d := range tmp {
			sum += d
		}
		avg := sum / time.Duration(len(tmp))
		fmt.Println("\nOverall latencies:")
		fmt.Printf("  samples=%d min=%v avg=%v max=%v p50=%v p90=%v p95=%v p99=%v\n",
			len(tmp), tmp[0], avg, tmp[len(tmp)-1], tmp[int(0.5*float64(len(tmp)-1))], tmp[int(0.9*float64(len(tmp)-1))], tmp[int(0.95*float64(len(tmp)-1))], tmp[int(0.99*float64(len(tmp)-1))])
	}

	// quick memory/CPU hint
	fmt.Printf("\nGOMAXPROCS=%d  NumGoroutine=%d\n", runtime.GOMAXPROCS(0), runtime.NumGoroutine())

	// optional JSON output
	if *outJSON != "" {
		type BackendSummary struct {
			Total   int32   `json:"total"`
			Success int32   `json:"success"`
			Failure int32   `json:"failure"`
			P50     float64 `json:"p50_ms"`
			P90     float64 `json:"p90_ms"`
			P95     float64 `json:"p95_ms"`
			P99     float64 `json:"p99_ms"`
		}
		report := map[string]interface{}{}
		report["target"] = *url
		report["requests"] = *requests
		report["concurrency"] = *concurrency
		report["total_sent"] = total
		report["success"] = success
		report["failure"] = failure
		report["duration_ms"] = totalDuration.Milliseconds()
		report["throughput_rps"] = throughput

		bsum := map[string]BackendSummary{}
		backendMu.Lock()
		for k, v := range backendStats {
			bs := BackendSummary{Total: v.Count, Success: v.Success, Failure: v.Failure}
			if len(v.Latencies) > 0 {
				tmp := make([]time.Duration, len(v.Latencies))
				copy(tmp, v.Latencies)
				sort.Slice(tmp, func(i, j int) bool { return tmp[i] < tmp[j] })
				pick := func(p float64) float64 { return float64(tmp[int(float64(len(tmp)-1)*p)].Milliseconds()) }
				bs.P50 = pick(0.50)
				bs.P90 = pick(0.90)
				bs.P95 = pick(0.95)
				bs.P99 = pick(0.99)
			}
			bsum[k] = bs
		}
		backendMu.Unlock()
		report["backends"] = bsum

		f, err := os.Create(*outJSON)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create json file: %v\n", err)
			os.Exit(1)
		}
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		enc.Encode(report)
		f.Close()
		fmt.Printf("\nWrote JSON summary to %s\n", *outJSON)
	}

	// exit with non-zero if there were failures
	if failure > 0 {
		os.Exit(2)
	}
}
