package httpapi

import (
	"net/http"

	"ai-gateway/internal/gateway"
)

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request, id *gateway.Identity) {
	s.GW.HandleChat(w, r, id)
}

func (s *Server) handleCompletions(w http.ResponseWriter, r *http.Request, id *gateway.Identity) {
	s.GW.HandleCompletions(w, r, id)
}

func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request, id *gateway.Identity) {
	s.GW.HandleEmbeddings(w, r, id)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request, id *gateway.Identity) {
	s.GW.HandleModels(w, r, id)
}
