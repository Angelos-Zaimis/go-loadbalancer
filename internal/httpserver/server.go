package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/go-ozzo/ozzo-validation/is"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// Server wraps http.Server with validation and graceful shutdown.
type Server struct {
	server *http.Server
}

// New creates a new HTTP server with the given address and handler.
// The address is validated before creating the server.
func New(addr string, handler http.Handler) (*Server, error) {
	if err := validateHost(addr); err != nil {
		return nil, err
	}

	srv := &Server{
		server: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}

	return srv, nil
}

// Start begins listening for HTTP requests.
// Returns an error unless the server is shut down cleanly.
func (s *Server) Start() error {
	err := s.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// Shutdown gracefully shuts down the server with a 5-second timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return s.server.Shutdown(shutdownCtx)
}

func validateHost(value interface{}) error {
	addr, ok := value.(string)
	if !ok {
		return validation.NewError("validation_invalid_type", "must be a string")
	}

	host, port, err := net.SplitHostPort(addr)

	if err != nil {
		return validation.NewError("validation_invalid_hostport", "must be in host:port format")
	}

	if port == "" {
		return validation.NewError("validation_invalid_port", "port cant be empty")
	}

	if host != "" {
		if err := is.Host.Validate(host); err != nil {
			return validation.NewError("validation_invalid_host", "invalid host")
		}
	}

	return err
}
