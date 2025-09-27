package middleware

import (
	"context"
	"hcs-full/models"
	"hcs-full/utils"
	"net/http"
)

// AuthMiddleware checks for a valid JWT in the cookie and redirects if not found.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("token")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		tokenStr := c.Value
		claims, err := utils.ParseJWT(tokenStr)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), "userClaims", claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminMiddleware checks if the user has admin privileges.
// This should be chained AFTER AuthMiddleware.
func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value("userClaims").(*models.Claims)
		if !ok {
			http.Error(w, "User claims not found", http.StatusInternalServerError)
			return
		}

		if !claims.IsAdmin {
			http.Error(w, "Forbidden: You do not have admin privileges.", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// SoftAuthMiddleware tries to authenticate but does not redirect on failure.
// This is used for WebSocket connections where a redirect would break the handshake.
func SoftAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("token")
		if err != nil { // No cookie, just proceed without claims
			next.ServeHTTP(w, r)
			return
		}

		tokenStr := c.Value
		claims, err := utils.ParseJWT(tokenStr)
		if err != nil { // Invalid token, just proceed without claims
			next.ServeHTTP(w, r)
			return
		}

		// Valid token, add claims to context
		ctx := context.WithValue(r.Context(), "userClaims", claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}


