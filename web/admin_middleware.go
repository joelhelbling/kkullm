package web

import "net/http"

// RequireAdmin is the single chokepoint for admin-only / dangerous routes.
// Today it is a pass-through; when auth lands, enforcement goes here so
// every admin route inherits the gate without per-handler changes.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
