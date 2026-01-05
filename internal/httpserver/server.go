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

type Server struct {
	server *http.Server
}

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

func (s *Server) Start() error {
	err := s.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

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
