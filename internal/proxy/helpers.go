package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func WriteOpenAIError(w http.ResponseWriter, status int, message string, errorType string) {
	WriteJSON(w, status, map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    errorType,
		},
	})
}

func WriteAuthenticationError(w http.ResponseWriter) {
	WriteOpenAIError(w, http.StatusUnauthorized, "Invalid authentication", "authentication_error")
}

func ReadRequestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

func ToBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.ToLower(val) == "true"
	case float64:
		return val != 0
	case json.Number:
		return val.String() != "0"
	default:
		return false
	}
}
