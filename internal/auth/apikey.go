package auth

import (
	"crypto/subtle"
	"net/http"
)

// Middleware возвращает HTTP middleware, проверяющий заголовок X-API-Key.
// Сравнение ключей — constant-time через crypto/subtle.
// При невалидном/отсутствующем ключе отвечает 401 Unauthorized.
func Middleware(expectedKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := r.Header.Get("X-API-Key")
			if subtle.ConstantTimeCompare([]byte(got), []byte(expectedKey)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
