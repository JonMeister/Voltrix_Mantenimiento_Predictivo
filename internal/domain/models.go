package domain

import (
	"regexp"
	"time"
)

var (
	IdentificadorRegex    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	EquipoConsumidorRegex = regexp.MustCompile(`^equipo-[1-7]$`)
)

// --- Esquemas COMÚN de la plantilla Voltrix Energy ---

type ErrorDetalle struct {
	Campo    string `json:"campo"`
	Problema string `json:"problema"`
}

type ErrorResponse struct {
	Codigo    string         `json:"codigo"`
	Mensaje   string         `json:"mensaje"`
	RequestID string         `json:"requestId"`
	Detalles  []ErrorDetalle `json:"detalles,omitempty"`
}

type Salud struct {
	Estado          string `json:"estado"`
	BaseDatos       string `json:"baseDatos"`
	Version         string `json:"version,omitempty"`
	TiempoActivoSeg int64  `json:"tiempoActivoSeg"`
}

type MetricasProcesamiento struct {
	Modo       string `json:"modo"`
	Workers    int    `json:"workers"`
	Elementos  int    `json:"elementos"`
	DuracionMs int64  `json:"duracionMs"`
}

// --- Esquemas propios de Mantenimiento Predictivo ---

type FactoresEvaluados struct {
	EstadoActivo            string  `json:"estadoActivo"`
	AlertasActivas          int     `json:"alertasActivas"`
	AlertasCriticas         int     `json:"alertasCriticas"`
	EficienciaHistoricaPct  float64 `json:"eficienciaHistoricaPct"`
	DesviacionPronosticoPct float64 `json:"desviacionPronosticoPct"`
}

type OrdenMantenimiento struct {
	ID                string             `json:"id"`
	PlantaID          string             `json:"plantaId"`
	ActivoID          string             `json:"activoId"`
	TipoActivo        string             `json:"tipoActivo"`
	TipoMantenimiento string             `json:"tipoMantenimiento"`
	Prioridad         string             `json:"prioridad"`
	Estado            string             `json:"estado"`
	PuntajeRiesgo     float64            `json:"puntajeRiesgo"`
	Motivo            string             `json:"motivo"`
	CostoEstimado     float64            `json:"costoEstimado"`
	Moneda            string             `json:"moneda"`
	FechaProgramada   string             `json:"fechaProgramada"`
	GeneradoEn        string             `json:"generadoEn"`
	ActualizadoEn     *string            `json:"actualizadoEn,omitempty"`
	Observaciones     *string            `json:"observaciones,omitempty"`
	FactoresEvaluados *FactoresEvaluados `json:"factoresEvaluados,omitempty"`
}

type PaginaOrdenesMantenimiento struct {
	Datos              []OrdenMantenimiento `json:"datos"`
	Total              int                  `json:"total"`
	CostoTotalEstimado float64              `json:"costoTotalEstimado"`
	Limit              int                  `json:"limit"`
	Offset             int                  `json:"offset"`
}

type SolicitudActualizacionOrden struct {
	Estado          *string  `json:"estado,omitempty"`
	FechaProgramada *string  `json:"fechaProgramada,omitempty"`
	CostoEstimado   *float64 `json:"costoEstimado,omitempty"`
	Observaciones   *string  `json:"observaciones,omitempty"`
}

type SolicitudGeneracionOrdenes struct {
	PlantaIDs    []string `json:"plantaIds,omitempty"`
	ActivoIDs    []string `json:"activoIds,omitempty"`
	Desde        string   `json:"desde"`
	Hasta        string   `json:"hasta"`
	UmbralRiesgo *float64 `json:"umbralRiesgo,omitempty"`
	TamanoLote   *int     `json:"tamanoLote,omitempty"`
}

type ResumenPorPrioridad struct {
	Baja    int `json:"baja"`
	Media   int `json:"media"`
	Alta    int `json:"alta"`
	Critica int `json:"critica"`
}

type ResultadoGeneracionOrdenes struct {
	LoteID              string                `json:"loteId"`
	ActivosEvaluados    int                   `json:"activosEvaluados"`
	Generados           int                   `json:"generados"`
	CostoTotalEstimado  float64               `json:"costoTotalEstimado"`
	ResumenPorPrioridad ResumenPorPrioridad   `json:"resumenPorPrioridad"`
	Ordenes             []OrdenMantenimiento  `json:"ordenes"`
	Metricas            MetricasProcesamiento `json:"metricas"`
}

type FiltroOrdenes struct {
	PlantaID          string
	ActivoID          string
	Estado            string
	Prioridad         string
	TipoMantenimiento string
	Desde             *time.Time
	Hasta             *time.Time
	Limit             int
	Offset            int
}

// --- Modelos internos normalizados (puertos hacia Equipos 1, 3, 4 y 5) ---

type ActivoInsumo struct {
	ID                 string
	PlantaID           string
	TipoActivo         string // normalizado: panel_solar | turbina_eolica | banco_baterias
	EstadoOperativo    string // operativo | degradado | fuera_de_servicio | en_mantenimiento | falla
	CapacidadNominalKw float64
}

type ResumenAlertasActivo struct {
	AlertasActivas  int
	AlertasCriticas int
	UltimaCondicion string
}

type HistoricoGeneracionActivo struct {
	EficienciaPct float64
	SerieKwh      []float64
}

type PronosticoPlantaInsumo struct {
	PlantaID               string
	DesviacionEsperadaPct  float64
	PromedioProyectadoKwh  float64
}

// --- Helpers de validación ---

func IsValidEstado(v string) bool {
	switch v {
	case "programada", "en_progreso", "completada", "cancelada":
		return true
	}
	return false
}

func IsValidPrioridad(v string) bool {
	switch v {
	case "baja", "media", "alta", "critica":
		return true
	}
	return false
}

func IsValidTipoMantenimiento(v string) bool {
	switch v {
	case "preventivo", "predictivo", "correctivo":
		return true
	}
	return false
}

func IsTerminalEstado(v string) bool {
	return v == "completada" || v == "cancelada"
}

func CanTransition(from, to string) bool {
	if from == to {
		return true
	}
	if IsTerminalEstado(from) {
		return false
	}
	switch from {
	case "programada":
		return to == "en_progreso" || to == "completada" || to == "cancelada"
	case "en_progreso":
		return to == "completada" || to == "cancelada"
	}
	return false
}

func ParseRFC3339WithTZ(raw string) (time.Time, error) {
	return time.Parse(time.RFC3339, raw)
}

