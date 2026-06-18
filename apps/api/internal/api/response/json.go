package response

import (
	"encoding/json"
	"net/http"
)

type ErrorDetails struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type ErrorEnvelope struct {
	Error ErrorDetails `json:"error"`
}

func WriteJSON(
	w http.ResponseWriter,
	status int,
	data any,
) error {
	// Encode before writing the status. If encoding fails, the response
	// has not been committed yet.
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	payload = append(payload, '\n')

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_, err = w.Write(payload)
	return err
}

func WriteError(
	w http.ResponseWriter,
	status int,
	code string,
	message string,
	requestID string,
) error {
	data := ErrorEnvelope{
		Error: ErrorDetails{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
	}

	return WriteJSON(w, status, data)
}
