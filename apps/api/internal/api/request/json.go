package request

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

var (
	ErrUnsupportedContentType = errors.New(
		"unsupported content type",
	)
	ErrBodyTooLarge = errors.New(
		"request body is too large",
	)
	ErrInvalidJSON = errors.New(
		"request body contains invalid JSON",
	)
)

const DefaultMaxJSONBodyBytes int64 = 64 * 1024

func DecodeJSON(
	w http.ResponseWriter,
	r *http.Request,
	destination any,
	maxBytes int64,
) error {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxJSONBodyBytes
	}

	if err := validateJSONContentType(r); err != nil {
		return err
	}

	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		maxBytes,
	)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		var maxBytesError *http.MaxBytesError

		if errors.As(err, &maxBytesError) {
			return ErrBodyTooLarge
		}

		return ErrInvalidJSON
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		var maxBytesError *http.MaxBytesError

		if errors.As(err, &maxBytesError) {
			return ErrBodyTooLarge
		}

		return ErrInvalidJSON
	}

	return nil
}

func validateJSONContentType(
	r *http.Request,
) error {
	contentType := r.Header.Get(
		"Content-Type",
	)
	if contentType == "" {
		return ErrUnsupportedContentType
	}

	mediaType, _, err := mime.ParseMediaType(
		contentType,
	)
	if err != nil ||
		mediaType != "application/json" {
		return ErrUnsupportedContentType
	}

	return nil
}
