package middleware

import (
	"context"
	"hcs-full/models"
	"hcs-full/utils"
	"net/http"
	"strings"
	"time"
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

// SecurityHeadersMiddleware adds security headers to all responses
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// HSTS - HTTP Strict Transport Security
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// CSP - Content Security Policy
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' unpkg.com cdn.jsdelivr.net; " +
			"style-src 'self' 'unsafe-inline' unpkg.com cdnjs.cloudflare.com; " +
			"img-src 'self' data: static.photos; " +
			"font-src 'self' cdnjs.cloudflare.com; " +
			"connect-src 'self' cdn.jsdelivr.net; " +
			"frame-ancestors 'none';"
		w.Header().Set("Content-Security-Policy", csp)

		// COOP - Cross-Origin-Opener-Policy
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")

		// XFO - X-Frame-Options (additional protection alongside CSP frame-ancestors)
		w.Header().Set("X-Frame-Options", "DENY")

		// Additional security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}

// CacheHeadersMiddleware adds appropriate cache headers for static assets
func CacheHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set cache headers for static assets
		if strings.HasPrefix(r.URL.Path, "/static/") || r.URL.Path == "/robots.txt" {
			w.Header().Set("Cache-Control", "public, max-age=86400") // 24 hours
			w.Header().Set("Expires", time.Now().Add(24*time.Hour).Format(http.TimeFormat))
		} else {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}

		next.ServeHTTP(w, r)
	})
}


