package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/client"
	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/config"
	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/handler"
	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/repository"
	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/service"
)

func main() {
	cfg := config.Load()

	level := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	if cfg.DatabaseURL == "" {
		logger.Error("DATABASE_URL es obligatoria y debe inyectarse por variable de entorno")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repo, err := repository.NewPostgresRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("no se pudo inicializar el pool de PostgreSQL", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer repo.Close()

	// Esperar disponibilidad de PostgreSQL al arrancar contenedor
	var pingErr error
	for attempt := 1; attempt <= 15; attempt++ {
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
		pingErr = repo.Ping(pingCtx)
		pingCancel()
		if pingErr == nil {
			break
		}
		logger.Warn("esperando PostgreSQL", slog.Int("intento", attempt), slog.String("error", pingErr.Error()))
		time.Sleep(2 * time.Second)
	}
	if pingErr != nil {
		logger.Error("PostgreSQL no respondió tras reintentos", slog.String("error", pingErr.Error()))
		os.Exit(1)
	}

	if err := repo.RunMigrations(context.Background(), cfg.SeedDemoData); err != nil {
		logger.Error("error aplicando migraciones SQL", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("migraciones aplicadas correctamente", slog.Bool("seedDemoData", cfg.SeedDemoData))

	var upstream client.UpstreamProvider
	if cfg.DataSource == "http" {
		upstream = client.NewHTTPUpstreamClient(
			cfg.URLInventario,
			cfg.URLAlertas,
			cfg.URLReportes,
			cfg.URLPronosticos,
			cfg.UpstreamTimeout,
		)
		logger.Info("fuente de datos upstream configurada en modo HTTP real")
	} else {
		upstream = client.NewMockUpstreamClient()
		logger.Info("fuente de datos upstream configurada en modo mock (esqueleto Avance 1)")
	}

	svc := service.NewMantenimientoService(repo, upstream, cfg.DefaultWorkers)
	apiHandler := handler.NewAPIHandler(svc, logger, cfg.Version, cfg.MaxConcurrentJobs, cfg.RetryAfterSeconds)

	mux := http.NewServeMux()
	apiHandler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler.ObservabilityMiddleware(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("servidor Mantenimiento Predictivo escuchando",
			slog.String("port", cfg.Port),
			slog.String("version", cfg.Version),
			slog.Int("defaultWorkers", cfg.DefaultWorkers),
			slog.Int("maxConcurrentJobs", cfg.MaxConcurrentJobs),
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("fallo en el servidor HTTP", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("apagando servidor gracefully")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("error durante shutdown", slog.String("error", err.Error()))
	}
}

