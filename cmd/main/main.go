package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/pdcgo/shared/authorization"
	"github.com/pdcgo/shared/configs"
	"github.com/pdcgo/shared/custom_connect"
	"github.com/pdcgo/shared/pkg/secret"
)

func IgnoreRoute(next http.Handler, jwtPhrase string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v4/withdrawal/health" {
			next.ServeHTTP(w, r)
		} else {
			authorization.MuxAuthMiddleware(nil, next, jwtPhrase).ServeHTTP(w, r)
		}

	})
}

func main() {
	var cfg configs.AppConfig
	var sec *secret.Secret
	var err error

	// getting configuration
	sec, err = secret.GetSecret("app_config_prod", "latest")
	if err != nil {
		panic(err)
	}
	err = sec.YamlDecode(&cfg)
	if err != nil {
		panic(err)
	}

	mux := runtime.NewServeMux()

	port := os.Getenv("PORT")
	host := ""
	if port == "" {
		port = "8080"
	}

	isDev := os.Getenv("DEV_MODE") != ""
	if isDev {
		host = "localhost"
	}

	authen := IgnoreRoute(mux, cfg.JwtSecret)
	log.Println("running withdrawal service")
	http.ListenAndServe(fmt.Sprintf("%s:%s", host, port), custom_connect.WithCORS(authen)) // App Engine uses 8080
}
