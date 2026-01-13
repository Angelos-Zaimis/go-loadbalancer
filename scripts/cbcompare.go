// cbcompare runs circuit breaker comparison tests and generates a markdown report.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type TestResult struct {
	Name           string
	TotalRequests  int
	SuccessfulReqs int
	FailedReqs     int
	TotalDuration  time.Duration
	AvgLatency     time.Duration
	MinLatency     time.Duration
	MaxLatency     time.Duration
	RequestsPerSec float64
	ErrorMessages  []string
}

var (
	totalReqs   = flag.Int("requests", 100, "Total requests to send")
	killAfter   = flag.Int("kill-after", 30, "Kill backend after N requests")
	backendPort = flag.Int("backend-port", 8081, "Backend port to kill")
)

func main() {
	flag.Parse()

	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     CIRCUIT BREAKER COMPARISON TEST                            ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// TEST 1: With Circuit Breaker
	fmt.Println("━━━ TEST 1: With Circuit Breaker Enabled ━━━")
	cleanup()
	startBackends()
	time.Sleep(2 * time.Second)
	startLB(true)
	time.Sleep(4 * time.Second)
	waitForLB()
	result1 := runTest("With Circuit Breaker")
	cleanup()

	time.Sleep(2 * time.Second)

	// TEST 2: Without Circuit Breaker
	fmt.Println("\n━━━ TEST 2: Without Circuit Breaker ━━━")
	startBackends()
	time.Sleep(2 * time.Second)
	startLB(false)
	time.Sleep(4 * time.Second)
	waitForLB()
	result2 := runTest("Without Circuit Breaker")
	cleanup()

	// Generate report
	generateReport(result1, result2)
	fmt.Println("\n✓ Tests complete! Results saved to scripts/circuit_breaker_results.md")
}

func cleanup() {
	exec.Command("bash", "-c", "pkill -f 'go-loadbalancer' 2>/dev/null").Run()
	exec.Command("bash", "-c", "pkill -f 'scripts/backend' 2>/dev/null").Run()
	// Use -sTCP:LISTEN to only kill the listening process, not connections
	for port := 8080; port <= 8085; port++ {
		exec.Command("bash", "-c", fmt.Sprintf("lsof -ti:%d -sTCP:LISTEN | xargs kill 2>/dev/null", port)).Run()
	}
	exec.Command("bash", "-c", "lsof -ti:6060 -sTCP:LISTEN | xargs kill 2>/dev/null").Run()
}

func startBackends() {
	fmt.Println("  Starting backends...")
	cmd := exec.Command("./scripts/spawn_backends.sh")
	cmd.Dir = projectRoot()
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Run()
}

func startLB(withCB bool) {
	status := "enabled"
	if !withCB {
		status = "disabled"
	}
	fmt.Printf("  Starting load balancer (circuit breaker %s)...\n", status)

	cmd := exec.Command("go", "run", "./cmd/")
	cmd.Dir = projectRoot()
	cmd.Stdout = nil
	cmd.Stderr = nil

	if !withCB {
		// Disable circuit breaker AND retries to show true difference
		cmd.Env = append(os.Environ(),
			"CIRCUIT_BREAKER_ENABLED=false",
			"RETRY_MAX_RETRIES=0")
	}
	cmd.Start()
}

func waitForLB() {
	fmt.Print("  Waiting for load balancer...")
	client := &http.Client{Timeout: time.Second}
	for i := 0; i < 30; i++ {
		resp, err := client.Get("http://localhost:8080/health")
		if err == nil {
			resp.Body.Close()
			fmt.Println(" ready!")
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Println(" timeout (continuing anyway)")
}

func runTest(name string) TestResult {
	result := TestResult{
		Name:          name,
		TotalRequests: *totalReqs,
		MinLatency:    time.Hour,
	}

	client := &http.Client{Timeout: 5 * time.Second}
	var latencies []time.Duration
	backendKilled := false

	fmt.Printf("  Sending %d requests (killing backend after %d)...\n", *totalReqs, *killAfter)
	start := time.Now()

	for i := 0; i < *totalReqs; i++ {
		if i == *killAfter && !backendKilled {
			fmt.Printf("  [Request %d] Killing backend on port %d\n", i, *backendPort)
			// Kill by finding the backend server process specifically, not any process with the port open
			// Use fuser which only finds the listening process
			exec.Command("bash", "-c", fmt.Sprintf("fuser -k %d/tcp 2>/dev/null || (lsof -ti:%d -sTCP:LISTEN | xargs kill 2>/dev/null)", *backendPort, *backendPort)).Run()
			backendKilled = true
			time.Sleep(200 * time.Millisecond)
		}

		reqStart := time.Now()
		resp, err := client.Get("http://localhost:8080/health")
		latency := time.Since(reqStart)
		latencies = append(latencies, latency)

		if err != nil {
			result.FailedReqs++
			errMsg := err.Error()
			if strings.Contains(errMsg, ":8080") {
				result.ErrorMessages = append(result.ErrorMessages, "LB connection refused")
			} else {
				result.ErrorMessages = append(result.ErrorMessages, truncate(errMsg, 80))
			}
		} else {
			// 404 is OK - our test backends return 404 for /test but that means they're alive
			// Only 5xx errors or connection errors are real failures
			if resp.StatusCode < 500 {
				result.SuccessfulReqs++
			} else {
				result.FailedReqs++
				result.ErrorMessages = append(result.ErrorMessages, fmt.Sprintf("HTTP %d", resp.StatusCode))
			}
			resp.Body.Close()
		}

		if latency < result.MinLatency {
			result.MinLatency = latency
		}
		if latency > result.MaxLatency {
			result.MaxLatency = latency
		}

		time.Sleep(20 * time.Millisecond)
	}

	result.TotalDuration = time.Since(start)

	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	if len(latencies) > 0 {
		result.AvgLatency = total / time.Duration(len(latencies))
	}
	result.RequestsPerSec = float64(*totalReqs) / result.TotalDuration.Seconds()

	fmt.Printf("  ✓ Results: %d/%d successful (%.1f%%)\n",
		result.SuccessfulReqs, result.TotalRequests,
		float64(result.SuccessfulReqs)/float64(result.TotalRequests)*100)

	return result
}

func generateReport(with, without TestResult) {
	withRate := float64(with.SuccessfulReqs) / float64(with.TotalRequests) * 100
	withoutRate := float64(without.SuccessfulReqs) / float64(without.TotalRequests) * 100
	diff := with.SuccessfulReqs - without.SuccessfulReqs

	var conclusion string
	if diff > 0 {
		conclusion = fmt.Sprintf("✅ **Circuit Breaker improved reliability by %d requests (%.1f%% → %.1f%%)**\n\nThe retry logic successfully recovered from backend failures.", diff, withoutRate, withRate)
	} else if diff == 0 {
		conclusion = "⚖️ **Both configurations performed equally.**\n\nThe health checker may have detected the failure quickly in both cases."
	} else {
		conclusion = fmt.Sprintf("⚠️ **Unexpected: Without CB had %d more successes.**\n\nThis may be due to timing or CB overhead.", -diff)
	}

	report := fmt.Sprintf(`# Circuit Breaker Comparison Test Results

**Test Date:** %s  
**Configuration:**
- Total Requests: %d
- Backend Killed After: Request #%d
- Backend Port: %d

---

## 📊 Summary Table

| Metric | With Circuit Breaker | Without Circuit Breaker | Difference |
|--------|:--------------------:|:----------------------:|:----------:|
| **Success Rate** | %.1f%% | %.1f%% | %+.1f%% |
| **Successful** | %d | %d | %+d |
| **Failed** | %d | %d | %+d |
| **Avg Latency** | %v | %v | - |
| **Max Latency** | %v | %v | - |
| **Throughput** | %.1f req/s | %.1f req/s | - |

---

## 🎯 Conclusion

%s

---

## 📈 Detailed Results

### With Circuit Breaker
- Success Rate: **%.1f%%** (%d/%d)
- Failed Requests: %d
- Avg Latency: %v
- Unique Errors: %d

### Without Circuit Breaker  
- Success Rate: **%.1f%%** (%d/%d)
- Failed Requests: %d
- Avg Latency: %v
- Unique Errors: %d

---

## 🔧 How Circuit Breaker Helps

| Feature | Benefit |
|---------|---------|
| **Fail-Fast** | Opens circuit after %d failures, avoiding timeouts |
| **Retry Logic** | Retries GET/PUT/DELETE on different backend |
| **Health Tracking** | Records success/failure per backend |
| **Recovery** | Half-open state probes backend recovery |

---

## ❌ Errors Observed

### With Circuit Breaker
%s

### Without Circuit Breaker
%s
`,
		time.Now().Format("2006-01-02 15:04:05"),
		*totalReqs, *killAfter, *backendPort,

		withRate, withoutRate, withRate-withoutRate,
		with.SuccessfulReqs, without.SuccessfulReqs, diff,
		with.FailedReqs, without.FailedReqs, with.FailedReqs-without.FailedReqs,
		with.AvgLatency.Round(time.Microsecond), without.AvgLatency.Round(time.Microsecond),
		with.MaxLatency.Round(time.Millisecond), without.MaxLatency.Round(time.Millisecond),
		with.RequestsPerSec, without.RequestsPerSec,

		conclusion,

		withRate, with.SuccessfulReqs, with.TotalRequests, with.FailedReqs,
		with.AvgLatency.Round(time.Microsecond), countUnique(with.ErrorMessages),

		withoutRate, without.SuccessfulReqs, without.TotalRequests, without.FailedReqs,
		without.AvgLatency.Round(time.Microsecond), countUnique(without.ErrorMessages),

		5, // failure threshold from config

		formatErrors(with.ErrorMessages),
		formatErrors(without.ErrorMessages),
	)

	path := projectRoot() + "/scripts/circuit_breaker_results.md"
	os.WriteFile(path, []byte(report), 0644)
}

func projectRoot() string {
	wd, _ := os.Getwd()
	if strings.HasSuffix(wd, "/scripts") {
		return strings.TrimSuffix(wd, "/scripts")
	}
	return wd
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func countUnique(errors []string) int {
	m := make(map[string]bool)
	for _, e := range errors {
		m[e] = true
	}
	return len(m)
}

func formatErrors(errors []string) string {
	if len(errors) == 0 {
		return "_No errors_"
	}

	counts := make(map[string]int)
	for _, e := range errors {
		counts[e]++
	}

	var sb strings.Builder
	for err, count := range counts {
		sb.WriteString(fmt.Sprintf("- `%s` ×%d\n", err, count))
	}
	return sb.String()
}
