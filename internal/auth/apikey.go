package auth

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
)

type ctxKey struct{}

// KeyNameFromContext возвращает имя API-ключа, прошедшего аутентификацию.
// Пустая строка — если контекст не содержит информации о ключе.
func KeyNameFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}

// Middleware возвращает HTTP middleware, проверяющий заголовок X-API-Key
// по одному ключу. Сравнение — constant-time через crypto/subtle.
// При невалидном/отсутствующем ключе отвечает 401 Unauthorized.
func Middleware(expectedKey string) func(http.Handler) http.Handler {
	return MultiKeyMiddleware([]Key{{Value: expectedKey, Name: "default"}})
}

// MultiKeyMiddleware возвращает HTTP middleware, проверяющий заголовок X-API-Key
// по списку допустимых ключей. Все сравнения — constant-time.
// При совпадении имя ключа сохраняется в context и логируется.
func MultiKeyMiddleware(keys []Key) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := []byte(r.Header.Get("X-API-Key"))

			var matched string
			for _, k := range keys {
				if subtle.ConstantTimeCompare(got, []byte(k.Value)) == 1 {
					matched = k.Name
					break
				}
			}

			if matched == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			log.Printf("auth: request from %q", matched)
			ctx := context.WithValue(r.Context(), ctxKey{}, matched)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
