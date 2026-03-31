package main

import (
	"log"
	"log/slog"
	"net/http"
	"net/http/httputil"

	"github.com/civil-labs/civil-api-go/civil/parcels/v1/parcelsv1connect"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		// Log fatal ensures the app exits with a non-zero status code
		log.Fatalf("Configuration Error: %v", err)
	}

	if cfg.Verbose {
		slog.Info("Starting proxy", slog.Any("address", cfg.TileServerAddress))
	}

	//ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()

	// Create the Reverse Proxy for the Tile Server with a custom Director
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {

			originalHost := req.Host

			if originalHost == "" {
				originalHost = req.URL.Host // Fallback
			}

			// Rewrite the request to target the tile server
			req.URL.Scheme = "http"
			req.URL.Host = cfg.TileServerAddress

			// Important: Update the Host header so the tile server accepts it
			req.Host = cfg.TileServerAddress

			// TELL THE BACKEND THE TRUTH
			// "The real host'"
			req.Header.Set("X-Forwarded-Host", originalHost)

			// "The user is using HTTPS (even if we are talking HTTP right now)"
			req.Header.Set("X-Forwarded-Proto", "https")

			// The user's real IP (Optional but good for logs)
			req.Header.Set("X-Real-IP", req.RemoteAddr)

			// Note: We do NOT touch req.URL.Path here.
			// It has already been stripped by the middleware below.
		},

		// This is needed to strip off any conflicting header details that the Tile Server attaches
		ModifyResponse: func(r *http.Response) error {

			// The Middleware already set these headers.
			// We MUST delete any versions sent by the backend to avoid the "Multiple Values" error.
			r.Header.Del("Access-Control-Allow-Origin")
			r.Header.Del("Access-Control-Allow-Methods")
			r.Header.Del("Access-Control-Allow-Headers")

			return nil
		},
	}

	auth, err := RequireAuth(cfg.Verbose, cfg.AuthServer, cfg.IDPAddress, cfg.AllowedClientsIds)

	parcelsServer := &ParcelServer{}

	mux := http.NewServeMux()

	path, handler := parcelsv1connect.NewParcelsServiceHandler(
		parcelsServer,
		connect.WithInterceptors(validate.NewInterceptor()),
	)

	mux.Handle(path, handler)

	mux.Handle("/tiles/", CORSMiddleware(auth(proxy), cfg.Verbose))
	mux.HandleFunc("/health", HealthCheckHandler(cfg.Verbose))

	p := new(http.Protocols)
	p.SetHTTP1(true)

	// Use h2c so we can serve HTTP/2 without TLS.
	p.SetUnencryptedHTTP2(true)
	s := http.Server{
		Addr:      ":" + cfg.HttpPort,
		Handler:   mux,
		Protocols: p,
	}

	log.Printf("Server listening on :%s", cfg.HttpPort)

	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}

}

func CORSMiddleware(next http.Handler, verbose bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if verbose {
			log.Println("CORS middleware activated")
		}

		// 1. ALWAYS set headers (Success, Failure, or Preflight)
		// This guarantees the browser never sees a "Missing Header" error.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// 2. Handle Preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 3. Pass to next handler
		next.ServeHTTP(w, r)
	})
}
