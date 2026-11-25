package workerpool

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/domain"
	"github.com/google/uuid"
)

// SnapshotInsumos agrupa los datos de entrada para la evaluación de activos.
type SnapshotInsumos struct {
	Activos     []domain.ActivoInsumo
	Alertas     map[string]domain.ResumenAlertasActivo
	Reportes    map[string]domain.HistoricoGeneracionActivo
	Pronosticos map[string]domain.PronosticoPlantaInsumo
}

type EvalParams struct {
	Modo         string // "secuencial" | "paralelo"
	Workers      int
	TamanoLote   int
	UmbralRiesgo float64
	Now          time.Time
}

type EvalResult struct {
	Ordenes    []domain.OrdenMantenimiento
	DuracionMs int64
	Workers    int
}

// EvaluarLotes — Esqueleto de Avance 1.
// Ejecuta una evaluación secuencial básica sobre los datos de ejemplo según el contrato OpenAPI.
// TODO(Avance 2): Implementar la descomposición de datos por lotes con Worker Pool en Go
// (goroutines, canales jobs/results y sync.WaitGroup) para comparar modo=secuencial vs modo=paralelo.
func EvaluarLotes(snap SnapshotInsumos, params EvalParams) EvalResult {
	start := time.Now()
	if len(snap.Activos) == 0 {
		return EvalResult{Ordenes: []domain.OrdenMantenimiento{}, DuracionMs: 0, Workers: 1}
	}

	ordenes := evaluarSublote(snap, snap.Activos, params.UmbralRiesgo, params.Now)
	return EvalResult{
		Ordenes:    ordenes,
		DuracionMs: time.Since(start).Milliseconds(),
		Workers:    1,
	}
}

func evaluarSublote(snap SnapshotInsumos, activos []domain.ActivoInsumo, umbral float64, now time.Time) []domain.OrdenMantenimiento {
	out := make([]domain.OrdenMantenimiento, 0)
	bogotaLoc := time.FixedZone("COT", -5*3600)

	for _, activo := range activos {
		if activo.EstadoOperativo == "en_mantenimiento" {
			continue
		}

		sEstado := scoreEstado(activo.EstadoOperativo)

		alerta := snap.Alertas[activo.ID]
		if alerta.AlertasActivas == 0 {
			alerta = snap.Alertas[activo.PlantaID]
		}
		sAlertas := math.Min(100, float64(alerta.AlertasActivas*10+alerta.AlertasCriticas*25))

		rep, hasRep := snap.Reportes[activo.ID]
		if !hasRep {
			rep, hasRep = snap.Reportes[activo.PlantaID]
		}
		eficienciaPct := 90.0
		pendientePenal := 0.0
		if hasRep {
			eficienciaPct = rep.EficienciaPct
			slope := linearSlope(rep.SerieKwh)
			if slope < 0 {
				pendientePenal = math.Min(30, math.Abs(slope)*3.0)
			}
		}
		sEficiencia := clamp((100.0-eficienciaPct)*1.5+pendientePenal, 0, 100)

		pron := snap.Pronosticos[activo.PlantaID]
		sPronostico := 0.0
		if pron.DesviacionEsperadaPct < 0 {
			sPronostico = math.Min(100, math.Abs(pron.DesviacionEsperadaPct)*2.5)
		}

		puntaje := math.Min(100, 0.35*sEstado+0.25*sAlertas+0.25*sEficiencia+0.15*sPronostico)
		puntaje = math.Round(puntaje*100) / 100

		if puntaje < umbral {
			continue
		}

		prioridad, tipoMant, factor, diasPlazo := clasificar(puntaje)
		costo := math.Round(costoBaseCOP(activo.TipoActivo) * factor)
		motivo := construirMotivo(activo.EstadoOperativo, alerta, eficienciaPct, pron.DesviacionEsperadaPct)

		shortUUID := strings.ToUpper(strings.ReplaceAll(uuid.New().String(), "-", "")[:8])
		ordenID := fmt.Sprintf("OM-%s-%s", now.UTC().Format("20060102"), shortUUID)

		out = append(out, domain.OrdenMantenimiento{
			ID:                ordenID,
			PlantaID:          activo.PlantaID,
			ActivoID:          activo.ID,
			TipoActivo:        activo.TipoActivo,
			TipoMantenimiento: tipoMant,
			Prioridad:         prioridad,
			Estado:            "programada",
			PuntajeRiesgo:     puntaje,
			Motivo:            motivo,
			CostoEstimado:     costo,
			Moneda:            "COP",
			FechaProgramada:   now.In(bogotaLoc).AddDate(0, 0, diasPlazo).Format(time.RFC3339),
			GeneradoEn:        now.UTC().Format(time.RFC3339),
			FactoresEvaluados: &domain.FactoresEvaluados{
				EstadoActivo:            activo.EstadoOperativo,
				AlertasActivas:          alerta.AlertasActivas,
				AlertasCriticas:         alerta.AlertasCriticas,
				EficienciaHistoricaPct:  math.Round(eficienciaPct*10) / 10,
				DesviacionPronosticoPct: math.Round(pron.DesviacionEsperadaPct*10) / 10,
			},
		})
	}
	return out
}

func scoreEstado(estado string) float64 {
	switch strings.ToLower(estado) {
	case "falla", "fuera_de_servicio":
		return 100
	case "degradado":
		return 65
	default:
		return 10
	}
}

func clasificar(puntaje float64) (prioridad, tipo string, factor float64, diasPlazo int) {
	switch {
	case puntaje >= 85:
		return "critica", "correctivo", 1.60, 1
	case puntaje >= 75:
		return "alta", "predictivo", 1.25, 3
	case puntaje >= 65:
		return "media", "predictivo", 1.15, 5
	default:
		return "baja", "preventivo", 1.00, 10
	}
}

func costoBaseCOP(tipoActivo string) float64 {
	switch tipoActivo {
	case "turbina_eolica":
		return 650000
	case "banco_baterias":
		return 530000
	default:
		return 360000
	}
}

func linearSlope(series []float64) float64 {
	n := float64(len(series))
	if n < 2 {
		return 0
	}
	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range series {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denom
}

func construirMotivo(estado string, alerta domain.ResumenAlertasActivo, eficienciaPct, devPron float64) string {
	var parts []string
	if estado == "fuera_de_servicio" || estado == "falla" {
		parts = append(parts, "activo en estado "+estado)
	} else if estado == "degradado" {
		parts = append(parts, "estado operativo degradado")
	}
	if alerta.AlertasCriticas > 0 {
		parts = append(parts, fmt.Sprintf("%d alertas críticas activas", alerta.AlertasCriticas))
	} else if alerta.AlertasActivas > 0 {
		parts = append(parts, fmt.Sprintf("%d alertas activas", alerta.AlertasActivas))
	}
	if eficienciaPct < 85 {
		parts = append(parts, fmt.Sprintf("caída de eficiencia histórica (%.1f%%)", eficienciaPct))
	}
	if devPron < -10 {
		parts = append(parts, fmt.Sprintf("desviación proyectada de %.1f%%", devPron))
	}
	if len(parts) == 0 {
		return "Umbral de riesgo superado en evaluación predictiva programada."
	}
	msg := strings.Join(parts, ", ")
	return strings.ToUpper(msg[:1]) + msg[1:] + "."
}

func clamp(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

