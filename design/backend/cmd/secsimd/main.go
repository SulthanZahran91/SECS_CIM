package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"secsim/design/backend/internal/api"
	"secsim/design/backend/internal/store"
)

func main() {
	addr := envOrDefault("SECSIM_ADDR", ":8080")
	state := store.New()

	mux := http.NewServeMux()
	api.Register(mux, state)
	registerFrontend(mux)

	log.Printf("SECSIM backend listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func registerFrontend(mux *http.ServeMux) {
	distDir, ok := findFrontendDist()
	if !ok {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "SECSIM backend is running. Build design/frontend or package web/dist to serve the UI here.")
		})
		return
	}

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		relativePath := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
		if relativePath == "." || relativePath == "" {
			http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
			return
		}
		target := filepath.Join(distDir, relativePath)
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			http.ServeFile(w, r, target)
			return
		}
		http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
	}))
}

func findFrontendDist() (string, bool) {
	candidates := make([]string, 0, 4)
	if envPath := os.Getenv("SECSIM_WEB_DIST"); envPath != "" {
		candidates = append(candidates, envPath)
	}

	if executablePath, err := os.Executable(); err == nil {
		executableDir := filepath.Dir(executablePath)
		candidates = append(candidates,
			filepath.Join(executableDir, "web", "dist"),
			filepath.Join(executableDir, "frontend", "dist"),
		)
	}

	candidates = append(candidates, filepath.Join("..", "frontend", "dist"))

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate, true
		}
	}

	return "", false
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
