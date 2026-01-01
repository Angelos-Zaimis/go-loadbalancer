// Backend is a simple test HTTP server used for load balancer testing.
// It provides /create-course and /health endpoints.
//
// Usage:
//
//	go run backend.go -port 8081
//
// The server logs all requests and returns JSON responses with unique UUIDs.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
)

// newUUID generates a random v4 UUID per RFC 4122.
func newUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	// set version (4) and variant bits per RFC 4122
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	// format as hex groups
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

// Course represents a course entity with unique identifier.
type Course struct {
	UUID        string `json:"uuid"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// CreateCourseRequest is the request payload for creating a course.
type CreateCourseRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func main() {
	port := flag.Int("port", 8081, "port to listen on")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/create-course", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// log request for visibility when running multiple backends
		clientAddr := r.RemoteAddr
		log.Printf("request: method=%s path=%s from=%s body=%s", r.Method, r.URL.Path, clientAddr, string(body))
		var req CreateCourseRequest
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
		}
		if req.Title == "" {
			req.Title = "Default Course"
		}
		course := Course{
			UUID:        newUUID(),
			Title:       req.Title,
			Description: req.Description,
		}

		resp := map[string]any{"course": course}
		b, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(b)
	})

	// simple health endpoint used by the load balancer health checker
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("starting backend on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
