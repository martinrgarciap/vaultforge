package middleware

import (
	"net/http"
	"net/netip"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
)

func RequireLoopback(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peerIP, ok := PeerIP(r)
		if !ok {
			writeInternalNotFound(w, r)
			return
		}

		address, err := netip.ParseAddr(peerIP)
		if err != nil || !address.IsLoopback() {
			writeInternalNotFound(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeInternalNotFound(w http.ResponseWriter, r *http.Request) {
	_ = response.WriteError(
		w,
		http.StatusNotFound,
		"not_found",
		"The requested resource was not found.",
		chimiddleware.GetReqID(r.Context()),
	)
}
