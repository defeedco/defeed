package server

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) searchActivities(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query         string   `json:"query"`
		SourceUIDs    []string `json:"sourceUIDs"`
		MinSimilarity float32  `json:"minSimilarity"`
		Limit         int      `json:"limit"`
		SortBy        string   `json:"sortBy"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate sortBy
	if req.SortBy != "" && req.SortBy != "similarity" && req.SortBy != "date" {
		http.Error(w, "Invalid sortBy value. Must be either 'similarity' or 'date'", http.StatusBadRequest)
		return
	}

	// Set default sort if not specified
	if req.SortBy == "" {
		req.SortBy = "similarity"
	}

	activities, err := h.registry.Search(r.Context(), req.Query, req.SourceUIDs, req.MinSimilarity, req.Limit, req.SortBy)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to search activities")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(activities); err != nil {
		h.logger.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
