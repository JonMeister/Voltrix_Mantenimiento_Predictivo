package client

import (
	"context"
	"time"

	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/domain"
)

// MockUpstreamClient provee datos de ejemplo determinísticos alineados a los contratos
// de Equipos 1, 3, 4 y 5 para el esqueleto desplegado en EC2 (Avance 1).
type MockUpstreamClient struct{}

func NewMockUpstreamClient() *MockUpstreamClient {
	return &MockUpstreamClient{}
}

func (m *MockUpstreamClient) catalog() []domain.ActivoInsumo {
	return []domain.ActivoInsumo{
		{ID: "PL-001-PS-01", PlantaID: "PL-001", TipoActivo: "panel_solar", EstadoOperativo: "operativo", CapacidadNominalKw: 12.5},
		{ID: "PL-001-PS-07", PlantaID: "PL-001", TipoActivo: "panel_solar", EstadoOperativo: "degradado", CapacidadNominalKw: 12.5},
		{ID: "PL-001-BB-01", PlantaID: "PL-001", TipoActivo: "banco_baterias", EstadoOperativo: "operativo", CapacidadNominalKw: 50.0},
		{ID: "PL-002-TB-02", PlantaID: "PL-002", TipoActivo: "turbina_eolica", EstadoOperativo: "operativo", CapacidadNominalKw: 80.0},
		{ID: "PL-002-BB-01", PlantaID: "PL-002", TipoActivo: "banco_baterias", EstadoOperativo: "fuera_de_servicio", CapacidadNominalKw: 60.0},
		{ID: "PL-003-PS-04", PlantaID: "PL-003", TipoActivo: "panel_solar", EstadoOperativo: "degradado", CapacidadNominalKw: 15.0},
	}
}

func (m *MockUpstreamClient) ObtenerActivos(_ context.Context, plantaIDs, activoIDs []string) ([]domain.ActivoInsumo, error) {
	all := m.catalog()
	if len(plantaIDs) == 0 && len(activoIDs) == 0 {
		return all, nil
	}
	plantaSet := make(map[string]bool, len(plantaIDs))
	for _, p := range plantaIDs {
		plantaSet[p] = true
	}
	activoSet := make(map[string]bool, len(activoIDs))
	for _, a := range activoIDs {
		activoSet[a] = true
	}

	var filtered []domain.ActivoInsumo
	for _, a := range all {
		if len(activoSet) > 0 {
			if activoSet[a.ID] {
				filtered = append(filtered, a)
			}
			continue
		}
		if plantaSet[a.PlantaID] {
			filtered = append(filtered, a)
		}
	}
	return filtered, nil
}

func (m *MockUpstreamClient) ObtenerAlertas(_ context.Context, _ []string) (map[string]domain.ResumenAlertasActivo, error) {
	return map[string]domain.ResumenAlertasActivo{
		"PL-001-PS-07": {AlertasActivas: 3, AlertasCriticas: 2, UltimaCondicion: "caida_generacion"},
		"PL-002-BB-01": {AlertasActivas: 5, AlertasCriticas: 4, UltimaCondicion: "sobrecalentamiento_bateria"},
		"PL-003-PS-04": {AlertasActivas: 2, AlertasCriticas: 1, UltimaCondicion: "inversor_degradado"},
	}, nil
}

func (m *MockUpstreamClient) ObtenerReportes(_ context.Context, _ []string, _, _ time.Time) (map[string]domain.HistoricoGeneracionActivo, error) {
	return map[string]domain.HistoricoGeneracionActivo{
		"PL-001-PS-01": {EficienciaPct: 94.0, SerieKwh: []float64{95, 94, 95, 93, 94}},
		"PL-001-PS-07": {EficienciaPct: 75.7, SerieKwh: []float64{94, 86, 78, 70, 64}},
		"PL-001-BB-01": {EficienciaPct: 91.5, SerieKwh: []float64{92, 91, 92, 91, 91}},
		"PL-002-TB-02": {EficienciaPct: 89.0, SerieKwh: []float64{90, 89, 90, 88, 89}},
		"PL-002-BB-01": {EficienciaPct: 58.0, SerieKwh: []float64{82, 74, 66, 54, 42}},
		"PL-003-PS-04": {EficienciaPct: 79.0, SerieKwh: []float64{88, 84, 80, 76, 72}},
	}, nil
}

func (m *MockUpstreamClient) ObtenerPronosticos(_ context.Context, _ []string) (map[string]domain.PronosticoPlantaInsumo, error) {
	return map[string]domain.PronosticoPlantaInsumo{
		"PL-001": {PlantaID: "PL-001", DesviacionEsperadaPct: -18.2, PromedioProyectadoKwh: 40.2},
		"PL-002": {PlantaID: "PL-002", DesviacionEsperadaPct: -31.5, PromedioProyectadoKwh: 32.0},
		"PL-003": {PlantaID: "PL-003", DesviacionEsperadaPct: -12.0, PromedioProyectadoKwh: 45.5},
	}, nil
}

