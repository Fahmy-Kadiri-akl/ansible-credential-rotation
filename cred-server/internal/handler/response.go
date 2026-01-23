package handler

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

// ErrorResponse represents a JSON error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// respondJSON writes a JSON response with the given status code
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("Failed to encode JSON response")
	}
}

// respondOK writes a JSON response with 200 OK status
func respondOK(w http.ResponseWriter, data interface{}) {
	respondJSON(w, http.StatusOK, data)
}

// respondCreated writes a JSON response with 201 Created status
func respondCreated(w http.ResponseWriter, data interface{}) {
	respondJSON(w, http.StatusCreated, data)
}

// respondError writes a JSON error response with the given status code
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    status,
	})
}

// respondBadRequest writes a 400 Bad Request error
func respondBadRequest(w http.ResponseWriter, message string) {
	respondError(w, http.StatusBadRequest, message)
}

// respondUnauthorized writes a 401 Unauthorized error
func respondUnauthorized(w http.ResponseWriter, message string) {
	respondError(w, http.StatusUnauthorized, message)
}

// respondNotFound writes a 404 Not Found error
func respondNotFound(w http.ResponseWriter, message string) {
	respondError(w, http.StatusNotFound, message)
}

// respondInternalError writes a 500 Internal Server Error
func respondInternalError(w http.ResponseWriter, message string) {
	respondError(w, http.StatusInternalServerError, message)
}

// respondBadGateway writes a 502 Bad Gateway error
func respondBadGateway(w http.ResponseWriter, message string) {
	respondError(w, http.StatusBadGateway, message)
}
