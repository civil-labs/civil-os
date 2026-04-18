package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/civil-labs/civil-api-go/civil/public/parcels/v1/parcelsv1connect"

	meshparcelsv1connect "github.com/civil-labs/civil-api-go/civil/mesh/parcels/v1/parcelsv1connect"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
)

func main() {
	// Create context, logger, and config first
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := LoadConfig(logger)
	if err != nil {
		logger.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	if cfg.Verbose {
		logger.Info("Starting proxy", slog.Any("address", cfg.TileServerAddress))
	}

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

			// Update the Host header so the tile server accepts it
			req.Host = cfg.TileServerAddress

			// TELL THE BACKEND THE TRUTH
			// "The real host"
			req.Header.Set("X-Forwarded-Host", originalHost)

			// "The user is using HTTPS (even if we are talking HTTP right now)"
			req.Header.Set("X-Forwarded-Proto", "https")

			// The user's real IP (Optional but good for logs)
			// Safely extract JUST the IP address, dropping the ephemeral port
			ip, _, err := net.SplitHostPort(req.RemoteAddr)
			if err == nil {
				req.Header.Set("X-Real-IP", ip)
			} else {
				// Fallback if RemoteAddr was somehow just an IP without a port
				req.Header.Set("X-Real-IP", req.RemoteAddr)
			}

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

	auth, err := RequireAuth(cfg.Verbose, cfg.AuthServer, cfg.IDPAddress, cfg.AllowedClientsIds, logger)

	meshClient := meshparcelsv1connect.NewParcelsServiceClient(
		http.DefaultClient,
		"http://db-reader", // The Envoy-routable address for the db-reader service
	)

	parcelsServer := &ParcelServer{
		dbReaderClient: meshClient,
	}

	mux := http.NewServeMux()

	path, handler := parcelsv1connect.NewParcelsServiceHandler(
		parcelsServer,
		connect.WithInterceptors(validate.NewInterceptor()),
	)

	mux.Handle(path, handler)

	mux.Handle("/tiles/", CORSMiddleware(auth(proxy), cfg.Verbose, logger))
	mux.HandleFunc("/health", HealthCheckHandler(cfg.Verbose))

	listenPort := fmt.Sprintf(":%d", cfg.Port)

	p := new(http.Protocols)
	p.SetHTTP1(true)

	// Use h2c so we can serve HTTP/2 without TLS.
	p.SetUnencryptedHTTP2(true)
	s := http.Server{
		Addr:      listenPort,
		Handler:   mux,
		Protocols: p,
	}

	logger.Info("starting connect server", slog.Int("port", int(cfg.Port)))

	if err := s.ListenAndServe(); err != nil {
		logger.Error("Server crashed", slog.Any("error", err))
		os.Exit(1)
	}

}

func CORSMiddleware(next http.Handler, verbose bool, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if verbose {
			logger.Info("CORS middleware activated")
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
