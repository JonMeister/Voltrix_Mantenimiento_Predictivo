package service

import (
	"context"
	"time"

	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/client"
	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/domain"
	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/repository"
	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/workerpool"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

type MantenimientoService struct {
	repo           *repository.PostgresRepository
	upstream       client.UpstreamProvider
	defaultWorkers int
}

func NewMantenimientoService(repo *repository.PostgresRepository, upstream client.UpstreamProvider, defaultWorkers int) *MantenimientoService {
	return &MantenimientoService{
		repo:           repo,
		upstream:       upstream,
		defaultWorkers: defaultWorkers,
	}
}

func (s *MantenimientoService) PingDB(ctx context.Context) error {
	return s.repo.Ping(ctx)
}

func (s *MantenimientoService) ListarOrdenes(ctx context.Context, filtro domain.FiltroOrdenes) (domain.PaginaOrdenesMantenimiento, error) {
	return s.repo.List(ctx, filtro)
}

func (s *MantenimientoService) ConsultarOrden(ctx context.Context, id string) (domain.OrdenMantenimiento, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *MantenimientoService) ActualizarOrden(ctx context.Context, id string, req domain.SolicitudActualizacionOrden) (domain.OrdenMantenimiento, error) {
	return s.repo.Patch(ctx, id, req)
}

func (s *MantenimientoService) GenerarOrdenes(
	ctx context.Context,
	req domain.SolicitudGeneracionOrdenes,
	desde, hasta time.Time,
	modo string,
	workers int,
) (domain.ResultadoGeneracionOrdenes, error) {
	if modo == "" {
		modo = "paralelo"
	}
	if workers <= 0 {
		workers = s.defaultWorkers
	}
	if modo == "secuencial" {
		workers = 1
	}

	umbral := 50.0
	if req.UmbralRiesgo != nil {
		umbral = *req.UmbralRiesgo
	}
	tamanoLote := 0
	if req.TamanoLote != nil {
		tamanoLote = *req.TamanoLote
	}

	// Fase 1: Recolección de activos desde Equipo 1 (Inventario)
	activos, err := s.upstream.ObtenerActivos(ctx, req.PlantaIDs, req.ActivoIDs)
	if err != nil {
		return domain.ResultadoGeneracionOrdenes{}, err
	}

	// Extraer conjunto único de plantas reales presentes en los activos
	plantaMap := make(map[string]struct{})
	for _, a := range activos {
		plantaMap[a.PlantaID] = struct{}{}
	}
	plantaIDs := make([]string, 0, len(plantaMap))
	for p := range plantaMap {
		plantaIDs = append(plantaIDs, p)
	}

	// Recolección concurrente I/O de Equipos 3, 4 y 5 mediante errgroup
	var (
		alertas     map[string]domain.ResumenAlertasActivo
		reportes    map[string]domain.HistoricoGeneracionActivo
		pronosticos map[string]domain.PronosticoPlantaInsumo
	)

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		alertas, err = s.upstream.ObtenerAlertas(gCtx, plantaIDs)
		return err
	})
	g.Go(func() error {
		var err error
		reportes, err = s.upstream.ObtenerReportes(gCtx, plantaIDs, desde, hasta)
		return err
	})
	g.Go(func() error {
		var err error
		pronosticos, err = s.upstream.ObtenerPronosticos(gCtx, plantaIDs)
		return err
	})
	if err := g.Wait(); err != nil {
		return domain.ResultadoGeneracionOrdenes{}, err
	}

	// Fase 2: Evaluación medida (secuencial vs. paralelo por lotes de activos)
	snap := workerpool.SnapshotInsumos{
		Activos:     activos,
		Alertas:     alertas,
		Reportes:    reportes,
		Pronosticos: pronosticos,
	}

	eval := workerpool.EvaluarLotes(snap, workerpool.EvalParams{
		Modo:         modo,
		Workers:      workers,
		TamanoLote:   tamanoLote,
		UmbralRiesgo: umbral,
		Now:          time.Now().UTC(),
	})

	// Fase 3: Consolidación y persistencia transaccional por lote
	loteID := uuid.New().String()
	if err := s.repo.BatchInsert(ctx, loteID, eval.Ordenes); err != nil {
		return domain.ResultadoGeneracionOrdenes{}, err
	}

	var resumen domain.ResumenPorPrioridad
	var costoTotal float64
	for _, o := range eval.Ordenes {
		costoTotal += o.CostoEstimado
		switch o.Prioridad {
		case "baja":
			resumen.Baja++
		case "media":
			resumen.Media++
		case "alta":
			resumen.Alta++
		case "critica":
			resumen.Critica++
		}
	}

	// En Avance 1 (esqueleto), la ejecución es secuencial (workers=1).
	// En Avance 2 se habilitará el Worker Pool paralelo para modo=paralelo.
	modoEjecutado := "secuencial"
	if eval.Workers > 1 {
		modoEjecutado = "paralelo"
	}

	return domain.ResultadoGeneracionOrdenes{
		LoteID:              loteID,
		ActivosEvaluados:    len(activos),
		Generados:           len(eval.Ordenes),
		CostoTotalEstimado:  costoTotal,
		ResumenPorPrioridad: resumen,
		Ordenes:             eval.Ordenes,
		Metricas: domain.MetricasProcesamiento{
			Modo:       modoEjecutado,
			Workers:    eval.Workers,
			Elementos:  len(activos),
			DuracionMs: eval.DuracionMs,
		},
	}, nil
}

