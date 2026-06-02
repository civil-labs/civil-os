package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/civil-labs/civil-api-go/civil/public/dex/v1/dexv1connect"
	"github.com/civil-labs/civil-api-go/civil/public/parcels/v1/parcelsv1connect"
	"github.com/dexidp/dex/api/v2"
	"google.golang.org/grpc"

	meshparcelsv1connect "github.com/civil-labs/civil-api-go/civil/mesh/parcels/v1/parcelsv1connect"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/validate"
)

func main() {
	// Create context, logger, and config first
	_, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()

	var programLevel = new(slog.LevelVar)

	programLevel.Set(slog.LevelInfo)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: programLevel,
	}))

	config, err := LoadConfig(logger)
	if err != nil {
		logger.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	if config.Verbose {
		programLevel.Set(slog.LevelDebug)
	}

	logger.Info("Starting proxy", slog.Any("address", config.TileServerHost))

	// Create the Reverse Proxy for the Tile Server with a custom Director
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {

			originalHost := req.Host

			if originalHost == "" {
				originalHost = req.URL.Host // Fallback
			}

			// Rewrite the request to target the tile server
			req.URL.Scheme = "http"
			req.URL.Host = config.TileServerHost

			// Update the Host header so the tile server accepts it
			req.Host = config.TileServerHost

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

	auth, err := RequireAuth(config.AuthServer, config.IDPHost, config.AllowedClientsIds, logger)

	dbReaderAddress := "http://" + config.DBReaderHost

	meshClient := meshparcelsv1connect.NewParcelsServiceClient(
		http.DefaultClient,
		dbReaderAddress, // The Envoy-routable address for the db-reader service
	)

	mux := http.NewServeMux()

	parcelsServer := &ParcelServer{
		dbReaderClient: meshClient,
		logger:         logger,
	}

	parcelsPath, parcelsHandler := parcelsv1connect.NewParcelsServiceHandler(
		parcelsServer,
		connect.WithInterceptors(validate.NewInterceptor()),
	)

	mux.Handle(parcelsPath, CORSMiddleware(auth(parcelsHandler), logger))

	// Create gRPC connection to Dex if an address is provided
	if config.DexGrpcAddress != "" {

		conn, err := grpc.NewClient(config.DexGrpcAddress)

		if err != nil {

			logger.Error("failed to connect to Dex's gRPC endpoint", slog.Any("error", err))

		}

		dexServer := &DexServer{
			dexClient: api.NewDexClient(conn),
			logger:    logger,
		}

		dexPath, dexHandler := dexv1connect.NewDexServiceHandler(
			dexServer,
			connect.WithInterceptors(validate.NewInterceptor()),
		)

		mux.Handle(dexPath, CORSMiddleware(auth(dexHandler), logger))

	}

	mux.Handle("/tiles/", CORSMiddleware(auth(proxy), logger))
	mux.HandleFunc("/health", HealthCheckHandler())

	// Pass the fully qualified name of the service so the health check
	// can report on this specific service, as well as the global server status.
	checker := grpchealth.NewStaticChecker(
		parcelsv1connect.ParcelsServiceName,
	)

	healthPath, healthHandler := grpchealth.NewHandler(checker)
	mux.Handle(healthPath, healthHandler)

	listenPort := fmt.Sprintf(":%d", config.Port)

	p := new(http.Protocols)
	p.SetHTTP1(true)

	// Use h2c so we can serve HTTP/2 without TLS.
	p.SetUnencryptedHTTP2(true)
	httpSrv := http.Server{
		Addr:      listenPort,
		Handler:   mux,
		Protocols: p,
	}

	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, os.Interrupt, syscall.SIGTERM)

	serverErr := make(chan error, 1)

	// Start the HTTP server in a background goroutine
	go func() {
		logger.Info("starting connect server", slog.Int("port", int(config.Port)))
		serverErr <- httpSrv.ListenAndServe()
	}()

	// This is inited by default to go's int zero value, zero
	var exitCode int

	// Block main() until something happens
	select {
	case err := <-serverErr:
		// The server crashed prematurely
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server crashed", slog.Any("error", err))
			exitCode = 1
		}
	case sig := <-shutdownSig:
		// Graceful shutdown signal received
		logger.Info("received shutdown signal", slog.String("signal", sig.String()))

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()

		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP graceful shutdown failed", slog.Any("error", err))
			exitCode = 1
		}
	}

	// This block runs no matter how the select statement unblocked.
	slog.Info("stopping background workers...")
	cancelApp()

	slog.Info("teardown complete. exiting.")
	os.Exit(exitCode)

}

func CORSMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		logger.Info("CORS middleware activated")

		// 1. ALWAYS set headers (Success, Failure, or Preflight)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", strings.Join([]string{
			"Authorization",
			"Content-Type",
			"Connect-Protocol-Version",
			"Connect-Timeout-Ms",
			"Connect-Accept-Encoding",
			"Connect-Content-Encoding",
			"Grpc-Timeout",
			"X-Grpc-Web",
			"X-User-Agent",
		}, ", "))
		w.Header().Set("Access-Control-Expose-Headers", strings.Join([]string{
			"Connect-Protocol-Version",
			"Connect-Timeout-Ms",
			"Grpc-Status",
			"Grpc-Message",
			"Grpc-Status-Details-Bin",
		}, ", "))
		w.Header().Set("Access-Control-Max-Age", "7200")

		// 2. Handle Preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 3. Pass to next handler
		next.ServeHTTP(w, r)
	})
}
