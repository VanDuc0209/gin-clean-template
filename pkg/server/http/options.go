package http_server

import (
	"net"
	"time"
)

const (
	_defaultAddr    = ":80"
	_defaultTimeout = 5 * time.Second
)

// Option -.
type Option func(*Server)

// Port -.
func Port(port string) Option {
	return func(s *Server) {
		s.address = net.JoinHostPort("", port)
	}
}

// Timeout -.
func Timeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.timeout = timeout
	}
}
