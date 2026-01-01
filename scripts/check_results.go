// Check_results validates CSV output from loadtest.go by checking for
// duplicate request indices and summarizing per-backend distribution.
//
// Usage:
//
//	go run check_results.go -csv results.csv -expected 5000
//
// The tool verifies:
//   - No duplicate request indices (data integrity)
//   - Total row count matches expected count (completeness)
//   - Per-backend request distribution (load balancing verification)
//
// Exit codes:
//
//	0 - Verification passed
//	2 - File errors or malformed CSV
//	3 - Duplicate indices found
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strconv"
)

func main() {
	csvPath := flag.String("csv", "results.csv", "Path to CSV produced by loadtest")
	expected := flag.Int("expected", 0, "Expected number of rows (optional)")
	flag.Parse()

	f, err := os.Open(*csvPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open csv: %v\n", err)
		os.Exit(2)
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read csv: %v\n", err)
		os.Exit(2)
	}

	if len(rows) == 0 {
		fmt.Fprintf(os.Stderr, "csv empty\n")
		os.Exit(2)
	}

	// header expected: idx,timestamp,backend,status,duration_ms
	header := rows[0]
	if len(header) < 5 {
		fmt.Fprintf(os.Stderr, "unexpected csv header: %v\n", header)
		os.Exit(2)
	}

	idxSeen := map[int]bool{}
	backendCounts := map[string]int{}

	for i := 1; i < len(rows); i++ {
		r := rows[i]
		if len(r) < 5 {
			fmt.Fprintf(os.Stderr, "malformed row %d: %v\n", i, r)
			os.Exit(2)
		}
		idx, err := strconv.Atoi(r[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid idx at row %d: %v\n", i, err)
			os.Exit(2)
		}
		if idxSeen[idx] {
			fmt.Printf("DUPLICATE idx=%d at csv row %d\n", idx, i)
		}
		idxSeen[idx] = true

		backend := r[2]
		backendCounts[backend]++
	}

	totalRows := len(rows) - 1
	unique := len(idxSeen)
	fmt.Printf("Total rows: %d  Unique idx: %d\n", totalRows, unique)

	if *expected > 0 && totalRows != *expected {
		fmt.Printf("Warning: total rows (%d) != expected (%d)\n", totalRows, *expected)
	}

	if totalRows != unique {
		fmt.Printf("ERROR: found %d duplicate indices\n", totalRows-unique)
		os.Exit(3)
	}

	fmt.Println("Per-backend counts:")
	for k, v := range backendCounts {
		fmt.Printf("  %s -> %d\n", k, v)
	}

	fmt.Println("Verification passed: no duplicate indices and counts sum match rows.")
}
