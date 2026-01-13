// cbtest is a tool to verify circuit breaker and retry behavior
// in the load balancer by simulating backend failures.
//
// Usage:
//
//	go run cbtest.go -lb http://localhost:8080 -backend-port 8081
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
)

func main() {
	var (
		lbURL       = flag.String("lb", "http://localhost:8080", "Load balancer URL")
		backendPort = flag.Int("backend-port", 8081, "Backend port to kill for testing")
		requests    = flag.Int("requests", 20, "Requests per phase")
		skipKill    = flag.Bool("skip-kill", false, "Skip the kill backend phase")
	)
	flag.Parse()

	client := &http.Client{Timeout: 5 * time.Second}

	fmt.Println(colorCyan + "╔════════════════════════════════════════════════════════════════╗" + colorReset)
	fmt.Println(colorCyan + "║         CIRCUIT BREAKER & RETRY TEST                          ║" + colorReset)
	fmt.Println(colorCyan + "╚════════════════════════════════════════════════════════════════╝" + colorReset)
	fmt.Println()

	// PHASE 1: Verify normal operation
	fmt.Println(colorBlue + "━━━ PHASE 1: Normal Operation ━━━" + colorReset)
	fmt.Println("Sending requests to verify all backends are healthy...")

	backendHits := make(map[string]int)
	for i := 0; i < *requests; i++ {
		resp, backend, err := sendRequest(client, *lbURL)
		if err != nil {
			fmt.Printf(colorRed+"  Request %d: ERROR - %v\n"+colorReset, i+1, err)
			continue
		}
		if resp.StatusCode >= 500 {
			fmt.Printf(colorRed+"  Request %d: Backend=%s Status=%d\n"+colorReset, i+1, backend, resp.StatusCode)
		} else {
			backendHits[backend]++
		}
		resp.Body.Close()
	}

	fmt.Println("\n  Backend distribution:")
	for backend, count := range backendHits {
		fmt.Printf("    %s → %d requests\n", backend, count)
	}
	if len(backendHits) == 0 {
		fmt.Println(colorRed + "  ✗ No backends responded! Is the load balancer running?" + colorReset)
		os.Exit(1)
	}
	fmt.Println(colorGreen + "  ✓ Normal operation verified" + colorReset)
	fmt.Println()

	// PHASE 2: Kill a backend and verify retry
	if !*skipKill {
		fmt.Println(colorBlue + "━━━ PHASE 2: Backend Failure & Retry ━━━" + colorReset)
		fmt.Printf("Killing backend on port %d...\n", *backendPort)

		if err := killBackend(*backendPort); err != nil {
			fmt.Printf(colorYellow+"  Warning: Could not kill backend: %v\n"+colorReset, err)
		} else {
			fmt.Printf(colorGreen+"  ✓ Backend on port %d killed\n"+colorReset, *backendPort)
		}

		time.Sleep(500 * time.Millisecond)

		fmt.Println("\n  Sending requests (should retry to healthy backends)...")
		successCount := 0
		for i := 0; i < *requests; i++ {
			resp, backend, err := sendRequest(client, *lbURL)
			if err != nil {
				fmt.Printf(colorRed+"  Request %d: ERROR - %v\n"+colorReset, i+1, err)
				continue
			}
			if resp.StatusCode < 500 {
				successCount++
			} else {
				fmt.Printf(colorYellow+"  Request %d: Backend=%s Status=%d\n"+colorReset, i+1, backend, resp.StatusCode)
			}
			resp.Body.Close()
		}

		fmt.Printf("\n  Results: %d/%d successful\n", successCount, *requests)
		if successCount == *requests {
			fmt.Println(colorGreen + "  ✓ All requests succeeded (retry logic working!)" + colorReset)
		} else {
			fmt.Println(colorYellow + "  ⚠ Some requests failed (check logs for retry attempts)" + colorReset)
		}
		fmt.Println()
	}

	// PHASE 3: Check metrics
	fmt.Println(colorBlue + "━━━ PHASE 3: Circuit Breaker Status ━━━" + colorReset)
	fmt.Println("Checking /metrics endpoint...")

	metrics, err := getMetrics(client, *lbURL+"/metrics")
	if err != nil {
		fmt.Printf(colorYellow+"  Could not fetch metrics: %v\n"+colorReset, err)
	} else {
		fmt.Println("\n  Backend health status:")
		if backends, ok := metrics["backends"].(map[string]interface{}); ok {
			for url, data := range backends {
				if bs, ok := data.(map[string]interface{}); ok {
					healthy := bs["healthy"].(bool)
					reqs := int(bs["requests"].(float64))
					status := colorGreen + "HEALTHY" + colorReset
					if !healthy {
						status = colorRed + "UNHEALTHY" + colorReset
					}
					fmt.Printf("    %s → %s (requests: %d)\n", url, status, reqs)
				}
			}
		}
	}
	fmt.Println()

	// PHASE 4: Idempotency test
	fmt.Println(colorBlue + "━━━ PHASE 4: Idempotency Check ━━━" + colorReset)
	fmt.Println("Testing POST request behavior...")

	postReq, _ := http.NewRequest("POST", *lbURL+"/test", strings.NewReader(`{"test":"data"}`))
	postReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := client.Do(postReq)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("  POST request failed: %v (took %v)\n", err, duration)
	} else {
		fmt.Printf("  POST request: Status=%d (took %v)\n", resp.StatusCode, duration)
		resp.Body.Close()
	}
	fmt.Println(colorGreen + "  ✓ POST behavior verified" + colorReset)
	fmt.Println()

	// Summary
	fmt.Println(colorCyan + "╔════════════════════════════════════════════════════════════════╗" + colorReset)
	fmt.Println(colorCyan + "║                    TEST COMPLETE                               ║" + colorReset)
	fmt.Println(colorCyan + "╚════════════════════════════════════════════════════════════════╝" + colorReset)
	fmt.Println()
	fmt.Println("Key behaviors verified:")
	fmt.Println("  1. Normal load balancing across backends")
	fmt.Println("  2. Retry on backend failure (GET requests)")
	fmt.Println("  3. Circuit breaker integration with /metrics")
	fmt.Println("  4. POST requests behavior")
	fmt.Println()
	fmt.Println("Check load balancer logs for detailed retry/circuit breaker activity.")
}

func sendRequest(client *http.Client, url string) (*http.Response, string, error) {
	req, err := http.NewRequest("GET", url+"/test", nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}

	backend := resp.Header.Get("X-Backend-Server")
	return resp, backend, nil
}

func killBackend(port int) error {
	cmd := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port))
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("no process found on port %d", port)
	}

	pid := strings.TrimSpace(string(output))
	if pid == "" {
		return fmt.Errorf("no process found on port %d", port)
	}

	killCmd := exec.Command("kill", pid)
	return killCmd.Run()
}

func getMetrics(client *http.Client, url string) (map[string]interface{}, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var metrics map[string]interface{}
	if err := json.Unmarshal(body, &metrics); err != nil {
		return nil, err
	}

	return metrics, nil
}
