package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/civil-labs/civil-os/internal/envconfig"

	"connectrpc.com/grpchealth"

	"github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/improvements/v1/improvementsv1connect"
	"github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/landuses/v1/landusesv1connect"
	meshneighborhoodsv1connect "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/neighborhoods/v1/neighborhoodsv1connect"
	"github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1/parcelsv1connect"
	meshvaluationsv1connect "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/valuations/v1/valuationsv1connect"
	"github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/zoning/v1/zoningv1connect"
)

type APIServer struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

func main() {
	// Log at INFO until the configured level is known; a LevelVar lets the
	// level change without rebuilding the logger.
	logLevel := new(slog.LevelVar)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	config, err := LoadConfig(envconfig.FromOS())
	if err != nil {
		logger.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}
	logLevel.Set(config.LogLevel)

	logger.Info("service initialized",
		slog.Int("grpc_port", int(config.Port)),
		slog.String("log_level", config.LogLevel.String()),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, config, logger); err != nil {
		logger.Error("reader stopped with an error", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("teardown complete. exiting.")
}

// run connects to the database and serves until ctx is cancelled
// (SIGINT/SIGTERM), then shuts down gracefully and closes the pool.
func run(ctx context.Context, config *Config, logger *slog.Logger) error {
	dbPoolConfig, err := BuildPoolConfig(config)
	if err != nil {
		return fmt.Errorf("invalid database configuration: %w", err)
	}
	dbPool, err := pgxpool.NewWithConfig(ctx, dbPoolConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize database pool: %w", err)
	}
	defer func() {
		logger.Info("closing database connections...")
		dbPool.Close()
	}()

	// Quick connection test
	pingCtx, cancelPing := context.WithTimeout(ctx, 2*time.Second)
	defer cancelPing()
	if err := dbPool.Ping(pingCtx); err != nil {
		logger.Warn("could not ping database", slog.Any("error", err))
	} else {
		logger.Info("ping test to PostgreSQL/PostGIS succeeded")
	}

	p := new(http.Protocols)
	p.SetHTTP1(true)
	p.SetUnencryptedHTTP2(true)
	httpSrv := http.Server{
		Addr:      fmt.Sprintf(":%d", config.Port),
		Handler:   newHandler(dbPool, logger),
		Protocols: p,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting connect server", slog.Int("port", int(config.Port)))
		serverErr <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server crashed: %w", err)
		}
		return nil
	case <-ctx.Done():
		logger.Info("received shutdown signal")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("HTTP graceful shutdown failed: %w", err)
		}
		return nil
	}
}

// newHandler builds the service's Connect handlers and gRPC health check.
func newHandler(pool *pgxpool.Pool, logger *slog.Logger) http.Handler {
	srv := &APIServer{
		db:     pool,
		logger: logger,
	}
	mux := http.NewServeMux()

	path, handler := parcelsv1connect.NewParcelReadServiceHandler(srv)
	mux.Handle(path, handler)

	path2, handler2 := improvementsv1connect.NewImprovementReadServiceHandler(srv)
	mux.Handle(path2, handler2)

	path3, handler3 := landusesv1connect.NewLandUseReadServiceHandler(srv)
	mux.Handle(path3, handler3)

	path4, handler4 := zoningv1connect.NewZoningReadServiceHandler(srv)
	mux.Handle(path4, handler4)

	path5, handler5 := meshvaluationsv1connect.NewValuationReadServiceHandler(srv)
	mux.Handle(path5, handler5)

	path6, handler6 := meshneighborhoodsv1connect.NewNeighborhoodReadServiceHandler(srv)
	mux.Handle(path6, handler6)

	// Pass the fully qualified name of the service so the health check
	// can report on this specific service, as well as the global server status.
	checker := grpchealth.NewStaticChecker(
		parcelsv1connect.ParcelReadServiceName,
		improvementsv1connect.ImprovementReadServiceName,
		landusesv1connect.LandUseReadServiceName,
		zoningv1connect.ZoningReadServiceName,
		meshvaluationsv1connect.ValuationReadServiceName,
		meshneighborhoodsv1connect.NeighborhoodReadServiceName,
	)

	healthPath, healthHandler := grpchealth.NewHandler(checker)
	mux.Handle(healthPath, healthHandler)

	return mux
}
