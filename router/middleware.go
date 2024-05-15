package router

import (
	"log"
	"net/http"
)

type Middleware func(http.Handler) http.Handler

type BasicAuthMiddlewareOpts struct {
	Enabled  bool
	Password string
}

func RequireBasicAuth(opts BasicAuthMiddlewareOpts) Middleware {
	return func(next http.Handler) http.Handler {
		if !opts.Enabled {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, password, ok := r.BasicAuth()
			if !ok || password != opts.Password {
				w.Header().Add("WWW-Authenticate", `Basic realm="Access to saws.world"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func Debug(enable bool) Middleware {
	return func(next http.Handler) http.Handler {
		if !enable {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Println(r.URL.String())
			next.ServeHTTP(w, r)
		})
	}
}
