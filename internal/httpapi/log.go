package httpapi

import (
	"log"
)

func (s *Server) logf(format string, args ...any) {
	log.Printf(format, args...)
}
