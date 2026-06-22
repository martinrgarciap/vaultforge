package request

import (
	"errors"
	"io"
	"net/http"
)

var ErrBodyNotAllowed = errors.New("request body is not allowed")

func RequireEmptyBody(w http.ResponseWriter, r *http.Request) error {
	if r == nil {
		return ErrBodyNotAllowed
	}

	if r.Body == nil {
		return nil
	}

	r.Body = http.MaxBytesReader(w, r.Body, 0)

	if _, err := io.Copy(io.Discard, r.Body); err != nil {
		return ErrBodyNotAllowed
	}

	return nil
}
