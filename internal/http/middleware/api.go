package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// CORSMiddleware adds CORS headers for API responses
// In production, configure allowedOrigins from config
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin == "" {
				referer := r.Header.Get("Referer")
				if referer != "" {
					if u, err := url.Parse(referer); err == nil {
						origin = u.Scheme + "://" + u.Host
					}
				}

				if origin == "" {
					scheme := "http"
					if r.TLS != nil {
						scheme = "https"
					}
					origin = scheme + "://" + r.Host
				}
			}

			if len(allowedOrigins) == 0 {
				if origin != "" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			} else if contains(allowedOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddlewareDefault creates CORS middleware with default settings (allow all)
func CORSMiddlewareDefault(next http.Handler) http.Handler {
	return CORSMiddleware(nil)(next)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

// JSONMiddleware ensures all API responses are JSON
func JSONMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
