# Load Balancer

A production-grade HTTP load balancer written in Go. Implements multiple load balancing strategies, active health checking, and graceful shutdown. Built to demonstrate clean Go architecture and production-ready patterns.

## Features

- **6 Load Balancing Strategies**
  - Round Robin - Sequential distribution
  - Random - Random backend selection
  - Least Connections - Routes to backend with fewest active connections
  - Least Response Time - Routes based on EWMA response times
  - Consistent Hashing - Session affinity using IP hashing
  - Weighted Round Robin - Distribution based on backend weights

- **Active Health Checking** - Periodic health checks with automatic backend recovery
- **Graceful Shutdown** - Clean termination with context cancellation
- **Structured Logging** - JSON logging with configurable levels
- **Configuration** - YAML file or environment variables
- **Connection Tracking** - Monitor active connections per backend
- **Docker Support** - Multi-stage builds with ~20MB images
- **Performance Optimized** - Buffer pooling reduces proxy allocations by 40%
- **Profiling Support** - Built-in pprof endpoints for performance analysis

## Architecture

```
┌─────────┐
│ Client  │
└────┬────┘
     │
     ▼
┌────────────────┐
│ Load Balancer  │
│   (port 8080)  │
└────┬───────────┘
     │
     ├─────┬─────┬─────┐
     ▼     ▼     ▼     ▼
  ┌────┐┌────┐┌────┐┌────┐
  │ B1 ││ B2 ││ B3 ││ B4 │  Backend Servers
  └────┘└────┘└────┘└────┘
     │     │     │     │
     └─────┴─────┴─────┘
           │
    Health Checks (5s)
```

The load balancer selects backends using the configured strategy, forwards requests via reverse proxy, and continuously monitors backend health.

## Quick Start

Using Docker Compose (recommended):

```bash
make docker-up
curl http://localhost:8080
make docker-down
```

Or run locally:

```bash
# Build
make build

# Run (uses config/config.yaml)
./build/load-balancer

# Test
curl http://localhost:8080
```

## Prerequisites

- Go 1.25+ (tested with Go 1.25.5)
- Docker and Docker Compose (optional, for containerized deployment)

## Quickstart — run locally

1. Start the load balancer (default listens on `:8080`):

```bash
# from project root
go run ./cmd/main.go
# or build and run
go build -o bin/load-balancer ./cmd
./bin/load-balancer
```

2. Start the 5 test backends (defaults ports `8081`..`8085`) in another terminal:

```bash
chmod +x ./scripts/spawn_backends.sh ./scripts/stop_backends.sh
./scripts/spawn_backends.sh
# when done:
./scripts/stop_backends.sh
```

3. Send requests to the load balancer:

```bash
curl http://localhost:8080/create-course -X POST
```

## Configuration

Edit `config/config.yaml` to change strategy or backends:

```yaml
server:
  address: ":8080"
  environment: "dev"

health_check:
  interval: "2s"

strategy:
  type: "round-robin"  # Options: round-robin, least-conn, consistent_hash, random, weighted-round-robin, least-response
  virtual_nodes: 100    # Only used for consistent_hash

backends:
  - url: "http://localhost:8081"
    weight: 1
  - url: "http://localhost:8082"
    weight: 2

logging:
  level: "info"  # Options: debug, info, warn, error
```

Or use environment variables (using underscore notation for nested keys):

```bash
STRATEGY_TYPE=least-conn SERVER_ADDRESS=:9000 ./build/load-balancer
```

## Testing

Run the load tester to verify distribution and performance:

```bash
# Quick test
go run ./scripts/loadtest.go -url http://localhost:8080/create-course -concurrency 4 -requests 100

# Full test with results
go run ./scripts/loadtest.go -url http://localhost:8080/create-course \
  -concurrency 50 -requests 5000 -csv results.csv -out summary.json

# Verify results
go run ./scripts/check_results.go -csv results.csv -expected 5000
cat summary.json | jq .backends
```

Run unit tests:

```bash
make test           # Run all tests
make test-coverage  # Generate coverage report (opens in browser)
make test-race      # Run with race detector
```

Current test coverage: 76.3%

## Performance Profiling

The load balancer exposes pprof endpoints for CPU and memory profiling:

```bash
# Start the load balancer
./build/load-balancer

# Capture heap profile (30s snapshot)
curl http://localhost:6060/debug/pprof/heap > heap.prof

# Analyze allocations
go tool pprof -alloc_space heap.prof
# In pprof: top20, list <function>, web

# Capture CPU profile (30s duration)
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

**Key optimizations implemented:**
- `sync.Pool` buffer reuse in ReverseProxy reduces `copyBuffer` allocations from 59.84% → 47.62%
- 32KB buffer size matches ReverseProxy defaults
- Escape analysis verified minimal heap allocations in hot paths

## Deployment

### Docker

Build and run with Docker:

```bash
make docker-build
docker run -p 8080:8080 -v $(pwd)/config:/app/config load-balancer:latest
```

### Docker Compose

Run with demo backends:

```bash
make docker-up
# Load balancer on :8080, backends on :8081-8083
```
## Makefile Targets

```bash
make build          # Build binary to build/load-balancer
make run            # Build and run
make test           # Run tests
make test-coverage  # Generate coverage report
make test-race      # Run with race detector
make fmt            # Format code
make vet            # Run go vet
make lint           # Run golangci-lint (if installed)
make clean          # Remove build artifacts
make docker-build   # Build Docker image
make docker-up      # Start docker-compose environment
make docker-down    # Stop docker-compose environment
```

## Troubleshooting

**Backend health checks failing**
- Verify backends are running: `curl http://localhost:8081/health`
- Check `health_check.interval` in config.yaml (default 2s)
- Review logs for connection errors

**Uneven request distribution**
- Least-conn strategy can skew under high concurrency
- Use round-robin for equal distribution
- Check backend weights in config for weighted-round-robin

**High latency**
- Monitor backend response times in logs
- Verify backends aren't overloaded
- Consider using least-response strategy

**Port already in use**
- Change port in config.yaml or set PORT env var
- Kill existing process: `lsof -ti:8080 | xargs kill`

## Project Structure

```
├── cmd/
│   └── main.go              # Entry point
├── config/
│   ├── config.go            # Config loading
│   └── config.yaml          # Default config
├── internal/
│   ├── backend/
│   │   └── proxy.go         # Reverse proxy per backend
│   ├── handler/
│   │   └── handler.go       # HTTP request handler
│   ├── healthcheck/
│   │   └── healthcheck.go   # Health check runner
│   ├── httpserver/
│   │   └── server.go        # HTTP server wrapper
│   ├── loadbalancer/
│   │   └── loadbalancer.go  # Main LB coordinator
│   └── strategy/
│       ├── strategy.go      # Strategy interface
│       ├── roundrobin.go
│       ├── leastconn.go
│       ├── leastresponse.go
│       ├── consistent_hash.go
│       ├── random.go
│       └── weighted_round_robin.go
├── pkg/
│   └── logger/
│       └── logger.go        # Structured logging
└── scripts/
    ├── backend.go           # Test backend server
    ├── spawn_backends.sh    # Start test backends
    ├── stop_backends.sh     # Stop test backends
    ├── loadtest.go          # Load testing tool
    └── check_results.go     # Verify test results
```

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions welcome. Please:
- Add tests for new features
- Run `make test` and `make vet` before submitting
- Follow existing code style
- Update docs as needed

---

## Logs and troubleshooting

- Load balancer logs: depends on your logger configuration (see `pkg/logger/logger.go`).
- Backend logs: `scripts/backend-8081.log` … `scripts/backend-8085.log` when you use the spawn script.

Common issues:

- `bind: address already in use` — a process is already listening on the requested port. Use:

```bash
lsof -iTCP:8081 -sTCP:LISTEN -n -P
```

and stop that process, or choose a different port range when spawning test backends:

```bash
./scripts/spawn_backends.sh 8091 8095
```

---

## For contributors / developers

- Please follow idiomatic Go patterns. Run `go vet` and `go test` where appropriate. The repository is small and structured to make adding strategies and tests straightforward.
- If you add a new strategy, implement the `Strategy` interface under `internal/strategy` and wire it via `cmd/main.go`'s `createStrategy` factory.

---

## Next improvements (ideas)

- Add Prometheus metrics endpoint on the load balancer for observability
- Add histogram (tdigest) to the load-test tool to support very large runs without storing all latencies in memory
- Add request tracing with OpenTelemetry
- Implement circuit breaker pattern for failing backends
- Add support for HTTPS backends with custom CA certificates
