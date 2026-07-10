package api

import (
	"encoding/json"
	"net/http"
)

type HTTPServer struct {
	service *Service
	mux     *http.ServeMux
}

func NewHTTPServer(service *Service) *HTTPServer {
	server := &HTTPServer{service: service, mux: http.NewServeMux()}
	server.routes()
	return server
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *HTTPServer) routes() {
	s.mux.HandleFunc("GET /v1/snapshot", s.handleSnapshot)
	s.mux.HandleFunc("POST /v1/audit", s.handleAudit)
}

func (s *HTTPServer) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.service.Snapshot())
}

func (s *HTTPServer) handleAudit(w http.ResponseWriter, r *http.Request) {
	result := s.service.Audit("http-audit")
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
