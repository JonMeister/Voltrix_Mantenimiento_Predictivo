package client

import (
	"context"
	"fmt"
	"time"

	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/domain"
)

// DependencyError representa una falla de comunicación o estado 5xx de otra célula,
// mapeada directamente a 502 DEPENDENCIA_FALLIDA.
type DependencyError struct {
	Equipo   string
	Endpoint string
	Mensaje  string
	Err      error
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("%s (%s): %s", e.Equipo, e.Endpoint, e.Mensaje)
}

func (e *DependencyError) Unwrap() error { return e.Err }

// UpstreamProvider agrupa los puertos adaptadores hacia Equipos 1, 3, 4 y 5.
type UpstreamProvider interface {
	ObtenerActivos(ctx context.Context, plantaIDs, activoIDs []string) ([]domain.ActivoInsumo, error)
	ObtenerAlertas(ctx context.Context, plantaIDs []string) (map[string]domain.ResumenAlertasActivo, error)
	ObtenerReportes(ctx context.Context, plantaIDs []string, desde, hasta time.Time) (map[string]domain.HistoricoGeneracionActivo, error)
	ObtenerPronosticos(ctx context.Context, plantaIDs []string) (map[string]domain.PronosticoPlantaInsumo, error)
}

