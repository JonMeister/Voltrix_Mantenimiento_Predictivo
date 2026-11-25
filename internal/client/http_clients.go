package client

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/domain"
)

const consumerHeaderValue = "equipo-6"

// HTTPUpstreamClient implementa los adaptadores alineados a los contratos reales
// de Equipo 1 (Inventario), Equipo 3 (Alertas y Notificaciones), Equipo 4 (Reportes)
// y Equipo 5 (Pronóstico).
type HTTPUpstreamClient struct {
	httpClient     *http.Client
	urlInventario  string
	urlAlertas     string
	urlReportes    string
	urlPronosticos string
}

func NewHTTPUpstreamClient(urlInv, urlAlertas, urlReportes, urlPronosticos string, timeout time.Duration) *HTTPUpstreamClient {
	return &HTTPUpstreamClient{
		httpClient:     &http.Client{Timeout: timeout},
		urlInventario:  strings.TrimRight(urlInv, "/"),
		urlAlertas:     strings.TrimRight(urlAlertas, "/"),
		urlReportes:    strings.TrimRight(urlReportes, "/"),
		urlPronosticos: strings.TrimRight(urlPronosticos, "/"),
	}
}

func (c *HTTPUpstreamClient) doGet(ctx context.Context, equipo, fullURL string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return &DependencyError{Equipo: equipo, Endpoint: fullURL, Mensaje: "URL inválida", Err: err}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Equipo-Consumidor", consumerHeaderValue)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &DependencyError{Equipo: equipo, Endpoint: fullURL, Mensaje: "sin respuesta o timeout", Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 400 {
		return &DependencyError{
			Equipo:   equipo,
			Endpoint: fullURL,
			Mensaje:  fmt.Sprintf("respondió %d", resp.StatusCode),
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return &DependencyError{Equipo: equipo, Endpoint: fullURL, Mensaje: "JSON inesperado", Err: err}
	}
	return nil
}

// --- Equipo 1: Inventario de Activos ---
// Contrato real: GET /activos?plantaId=...&limit=200&offset=0 -> PaginaActivos
// TipoActivo real: panel_solar | turbina | banco_baterias
// EstadoActivo real: operativo | en_mantenimiento | fuera_de_servicio

type invActivoDTO struct {
	ID                 string  `json:"id"`
	PlantaID           string  `json:"plantaId"`
	Tipo               string  `json:"tipo"`
	Estado             string  `json:"estado"`
	CapacidadNominalKw float64 `json:"capacidadNominalKw"`
}

type invPaginaDTO struct {
	Datos  []invActivoDTO `json:"datos"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

func normalizeTipoActivo(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "turbina", "turbina_eolica":
		return "turbina_eolica"
	case "banco_baterias", "bateria", "baterias":
		return "banco_baterias"
	default:
		return "panel_solar"
	}
}

func (c *HTTPUpstreamClient) ObtenerActivos(ctx context.Context, plantaIDs, activoIDs []string) ([]domain.ActivoInsumo, error) {
	if c.urlInventario == "" {
		return nil, &DependencyError{Equipo: "equipo-1", Endpoint: "GET /activos", Mensaje: "URL_INVENTARIO no configurada"}
	}

	var result []domain.ActivoInsumo
	if len(activoIDs) > 0 {
		for _, id := range activoIDs {
			var dto invActivoDTO
			endpoint := fmt.Sprintf("%s/activos/%s", c.urlInventario, url.PathEscape(id))
			if err := c.doGet(ctx, "equipo-1", endpoint, &dto); err != nil {
				return nil, err
			}
			if dto.ID != "" {
				result = append(result, domain.ActivoInsumo{
					ID:                 dto.ID,
					PlantaID:           dto.PlantaID,
					TipoActivo:         normalizeTipoActivo(dto.Tipo),
					EstadoOperativo:    dto.Estado,
					CapacidadNominalKw: dto.CapacidadNominalKw,
				})
			}
		}
		return result, nil
	}

	targets := plantaIDs
	if len(targets) == 0 {
		targets = []string{""}
	}
	for _, pid := range targets {
		q := url.Values{}
		q.Set("limit", "200")
		q.Set("offset", "0")
		if pid != "" {
			q.Set("plantaId", pid)
		}
		endpoint := fmt.Sprintf("%s/activos?%s", c.urlInventario, q.Encode())
		var page invPaginaDTO
		if err := c.doGet(ctx, "equipo-1", endpoint, &page); err != nil {
			return nil, err
		}
		for _, dto := range page.Datos {
			result = append(result, domain.ActivoInsumo{
				ID:                 dto.ID,
				PlantaID:           dto.PlantaID,
				TipoActivo:         normalizeTipoActivo(dto.Tipo),
				EstadoOperativo:    dto.Estado,
				CapacidadNominalKw: dto.CapacidadNominalKw,
			})
		}
	}
	return result, nil
}

// --- Equipo 3: Alertas y Notificaciones ---
// Contrato real: GET /alertas/estado?plantaId=...&estado=activa&limit=200&offset=0 -> EstadoAlertas
// resumenPorActivo (sin paginar, diseñado para Equipo 6): [{ plantaId, activoId, activas, criticas }]
// datos (paginado): [{ id, plantaId, activoId, tipoActivo, metrica, valorObservado, umbralAplicado, severidad, estado, ocurrencias }]
// Metrica real: temperatura | caida_potencia | velocidad_viento; Severidad real: advertencia | critica.

type alertaItemDTO struct {
	ID             string  `json:"id"`
	PlantaID       string  `json:"plantaId"`
	ActivoID       string  `json:"activoId"`
	TipoActivo     string  `json:"tipoActivo"`
	Metrica        string  `json:"metrica"`
	ValorObservado float64 `json:"valorObservado"`
	UmbralAplicado float64 `json:"umbralAplicado"`
	Severidad      string  `json:"severidad"`
	Estado         string  `json:"estado"`
	Ocurrencias    int     `json:"ocurrencias"`
}

type alertaResumenDTO struct {
	PlantaID string `json:"plantaId"`
	ActivoID string `json:"activoId"`
	Activas  int    `json:"activas"`
	Criticas int    `json:"criticas"`
}

type alertasRespDTO struct {
	Datos            []alertaItemDTO    `json:"datos"`
	ResumenPorActivo []alertaResumenDTO `json:"resumenPorActivo"`
}

func (c *HTTPUpstreamClient) ObtenerAlertas(ctx context.Context, plantaIDs []string) (map[string]domain.ResumenAlertasActivo, error) {
	out := make(map[string]domain.ResumenAlertasActivo)
	if c.urlAlertas == "" {
		return out, nil
	}

	targets := plantaIDs
	if len(targets) == 0 {
		targets = []string{""}
	}
	fromSummary := make(map[string]bool)
	for _, pid := range targets {
		q := url.Values{}
		q.Set("estado", "activa")
		q.Set("limit", "200")
		q.Set("offset", "0")
		if pid != "" {
			q.Set("plantaId", pid)
		}
		endpoint := fmt.Sprintf("%s/alertas/estado?%s", c.urlAlertas, q.Encode())
		var resp alertasRespDTO
		if err := c.doGet(ctx, "equipo-3", endpoint, &resp); err != nil {
			return nil, err
		}
		// 1) Fuente autoritativa de conteos: resumenPorActivo (sin paginar en Equipo 3).
		for _, r := range resp.ResumenPorActivo {
			if r.ActivoID == "" {
				continue
			}
			cur := out[r.ActivoID]
			cur.AlertasActivas = r.Activas
			cur.AlertasCriticas = r.Criticas
			out[r.ActivoID] = cur
			fromSummary[r.ActivoID] = true
		}
		// 2) Enriquecer UltimaCondicion con datos[].metrica sin duplicar conteos ya provistos por resumenPorActivo.
		for _, a := range resp.Datos {
			if a.Estado != "" && strings.ToLower(a.Estado) != "activa" {
				continue
			}
			key := a.ActivoID
			if key == "" {
				key = a.PlantaID
			}
			if key == "" {
				continue
			}
			cur := out[key]
			if !fromSummary[key] {
				cur.AlertasActivas++
				if strings.ToLower(a.Severidad) == "critica" {
					cur.AlertasCriticas++
				}
			}
			if a.Metrica != "" && (cur.UltimaCondicion == "" || strings.ToLower(a.Severidad) == "critica") {
				cur.UltimaCondicion = a.Metrica
			}
			out[key] = cur
		}
	}
	return out, nil
}

// --- Equipo 4: Reportes de Generación ---
// Contrato real: GET /reportes/generacion?plantaId=...&desde=...&hasta=...&granularidad=dia&limit=200
// PaginaAgregados -> datos[].{ plantaId, activoId, energiaKwh, factorPlanta, capacidadNominalKw }

type repAgregadoDTO struct {
	PlantaID           string   `json:"plantaId"`
	ActivoID           *string  `json:"activoId"`
	EnergiaKwh         float64  `json:"energiaKwh"`
	CapacidadNominalKw *float64 `json:"capacidadNominalKw"`
	FactorPlanta       *float64 `json:"factorPlanta"`
}

type repPaginaDTO struct {
	Datos []repAgregadoDTO `json:"datos"`
}

func (c *HTTPUpstreamClient) ObtenerReportes(ctx context.Context, plantaIDs []string, desde, hasta time.Time) (map[string]domain.HistoricoGeneracionActivo, error) {
	out := make(map[string]domain.HistoricoGeneracionActivo)
	if c.urlReportes == "" {
		return out, nil
	}

	targets := plantaIDs
	if len(targets) == 0 {
		targets = []string{""}
	}
	factorSum := make(map[string]float64)
	factorCnt := make(map[string]int)

	for _, pid := range targets {
		q := url.Values{}
		q.Set("granularidad", "dia")
		q.Set("limit", "200")
		if pid != "" {
			q.Set("plantaId", pid)
		}
		if !desde.IsZero() {
			q.Set("desde", desde.Format(time.RFC3339))
		}
		if !hasta.IsZero() {
			q.Set("hasta", hasta.Format(time.RFC3339))
		}
		endpoint := fmt.Sprintf("%s/reportes/generacion?%s", c.urlReportes, q.Encode())
		var page repPaginaDTO
		if err := c.doGet(ctx, "equipo-4", endpoint, &page); err != nil {
			return nil, err
		}
		for _, item := range page.Datos {
			key := item.PlantaID
			if item.ActivoID != nil && *item.ActivoID != "" {
				key = *item.ActivoID
			}
			cur := out[key]
			cur.SerieKwh = append(cur.SerieKwh, item.EnergiaKwh)
			if item.FactorPlanta != nil {
				// Factor de planta solar/eólico típico nominal ~0.35 -> escalamos a eficiencia %
				eff := math.Min(100, (*item.FactorPlanta/0.35)*100)
				factorSum[key] += eff
				factorCnt[key]++
			}
			out[key] = cur
		}
	}

	for key, cur := range out {
		if factorCnt[key] > 0 {
			cur.EficienciaPct = factorSum[key] / float64(factorCnt[key])
		} else {
			cur.EficienciaPct = 85.0
		}
		out[key] = cur
	}
	return out, nil
}

// --- Equipo 5: Pronóstico de Generación ---
// Contrato real (snake_case): GET /pronosticos/{plantaId} -> PronosticoPlanta
// { planta_id, metodo, frecuencia, generado_en, valores: [ { periodo, generacion_kwh } ] }

type pronValorDTO struct {
	Periodo       string  `json:"periodo"`
	GeneracionKwh float64 `json:"generacion_kwh"`
}

type pronPlantaDTO struct {
	PlantaID string         `json:"planta_id"`
	Valores  []pronValorDTO `json:"valores"`
}

func (c *HTTPUpstreamClient) ObtenerPronosticos(ctx context.Context, plantaIDs []string) (map[string]domain.PronosticoPlantaInsumo, error) {
	out := make(map[string]domain.PronosticoPlantaInsumo)
	if c.urlPronosticos == "" || len(plantaIDs) == 0 {
		return out, nil
	}

	for _, pid := range plantaIDs {
		endpoint := fmt.Sprintf("%s/pronosticos/%s", c.urlPronosticos, url.PathEscape(pid))
		var dto pronPlantaDTO
		if err := c.doGet(ctx, "equipo-5", endpoint, &dto); err != nil {
			return nil, err
		}
		if len(dto.Valores) == 0 {
			continue
		}
		var sum float64
		for _, v := range dto.Valores {
			sum += v.GeneracionKwh
		}
		avg := sum / float64(len(dto.Valores))
		first := dto.Valores[0].GeneracionKwh
		last := dto.Valores[len(dto.Valores)-1].GeneracionKwh
		var devPct float64
		if first > 0 {
			devPct = ((last - first) / first) * 100.0
		}
		out[pid] = domain.PronosticoPlantaInsumo{
			PlantaID:              pid,
			DesviacionEsperadaPct: devPct,
			PromedioProyectadoKwh: avg,
		}
	}
	return out, nil
}

