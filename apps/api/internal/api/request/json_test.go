package request

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testJSONRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func TestDecodeJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		body        string
		maxBytes    int64
		want        testJSONRequest
		wantErr     error
	}{
		{
			name:        "accepts valid JSON",
			contentType: "application/json",
			body: `{
				"email": "martin@example.com",
				"password": "correct horse battery staple"
			}`,
			want: testJSONRequest{
				Email:    "martin@example.com",
				Password: "correct horse battery staple",
			},
		},
		{
			name:        "accepts JSON content type with charset",
			contentType: "application/json; charset=utf-8",
			body: `{
				"email": "martin@example.com",
				"password": "correct horse battery staple"
			}`,
			want: testJSONRequest{
				Email:    "martin@example.com",
				Password: "correct horse battery staple",
			},
		},
		{
			name:        "uses default limit when limit is zero",
			contentType: "application/json",
			body: `{
				"email": "martin@example.com",
				"password": "correct horse battery staple"
			}`,
			maxBytes: 0,
			want: testJSONRequest{
				Email:    "martin@example.com",
				Password: "correct horse battery staple",
			},
		},
		{
			name:        "rejects missing content type",
			contentType: "",
			body:        `{}`,
			wantErr:     ErrUnsupportedContentType,
		},
		{
			name:        "rejects non JSON content type",
			contentType: "text/plain",
			body:        `{}`,
			wantErr:     ErrUnsupportedContentType,
		},
		{
			name:        "rejects malformed content type",
			contentType: "application/json; charset",
			body:        `{}`,
			wantErr:     ErrUnsupportedContentType,
		},
		{
			name:        "rejects malformed JSON",
			contentType: "application/json",
			body:        `{"email":`,
			wantErr:     ErrInvalidJSON,
		},
		{
			name:        "rejects empty body",
			contentType: "application/json",
			body:        "",
			wantErr:     ErrInvalidJSON,
		},
		{
			name:        "rejects unknown field",
			contentType: "application/json",
			body: `{
				"email": "martin@example.com",
				"password": "correct horse battery staple",
				"unexpected": true
			}`,
			wantErr: ErrInvalidJSON,
		},
		{
			name:        "rejects multiple JSON values",
			contentType: "application/json",
			body: `{
				"email": "martin@example.com",
				"password": "correct horse battery staple"
			}
			{
				"email": "second@example.com"
			}`,
			wantErr: ErrInvalidJSON,
		},
		{
			name:        "rejects oversized body",
			contentType: "application/json",
			body: `{
				"email": "martin@example.com",
				"password": "correct horse battery staple"
			}`,
			maxBytes: 16,
			wantErr:  ErrBodyTooLarge,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			httpRequest := httptest.NewRequest(
				http.MethodPost,
				"/v1/auth/register",
				strings.NewReader(test.body),
			)

			if test.contentType != "" {
				httpRequest.Header.Set(
					"Content-Type",
					test.contentType,
				)
			}

			recorder := httptest.NewRecorder()
			var got testJSONRequest

			err := DecodeJSON(
				recorder,
				httpRequest,
				&got,
				test.maxBytes,
			)

			if test.wantErr == nil {
				if err != nil {
					t.Fatalf(
						"DecodeJSON() returned unexpected error: %v",
						err,
					)
				}

				if got != test.want {
					t.Fatalf(
						"DecodeJSON() = %+v, want %+v",
						got,
						test.want,
					)
				}

				return
			}

			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"DecodeJSON() error = %v, want %v",
					err,
					test.wantErr,
				)
			}
		})
	}
}
