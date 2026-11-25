package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	voltrix "github.com/JonMeister/Voltrix_Mantenimiento_Predictivo"
	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/client"
	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/domain"
	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/repository"
	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/service"
)

type APIHandler struct {
	svc       *service.MantenimientoService
	logger    *slog.Logger
	version   string
	startedAt time.Time
	semaphore *AdmissionSemaphore
}

func NewAPIHandler(svc *service.MantenimientoService, logger *slog.Logger, version string, maxConcurrent, retryAfter int) *APIHandler {
	return &APIHandler{
		svc:       svc,
		logger:    logger,
		version:   version,
		startedAt: time.Now(),
		semaphore: NewAdmissionSemaphore(maxConcurrent, retryAfter),
	}
}

func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	// Endpoints comunes obligatorios
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /openapi.yaml", h.OpenAPISpec)
	mux.HandleFunc("GET /docs", h.RedocDocs)
	mux.HandleFunc("GET /", h.RedocDocs)

	// Endpoints del servicio Mantenimiento Predictivo
	mux.HandleFunc("GET /mantenimiento/ordenes", h.ListarOrdenes)
	mux.HandleFunc("GET /mantenimiento/ordenes/{id}", h.ConsultarOrden)
	mux.HandleFunc("PATCH /mantenimiento/ordenes/{id}", h.ActualizarOrden)
	mux.HandleFunc("POST /mantenimiento/generar", h.semaphore.Wrap(h.GenerarOrdenes))
}

func (h *APIHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
	defer cancel()

	uptime := int64(time.Since(h.startedAt).Seconds())
	if err := h.svc.PingDB(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, domain.Salud{
			Estado:          "no_disponible",
			BaseDatos:       "error",
			Version:         h.version,
			TiempoActivoSeg: uptime,
		})
		return
	}
	writeJSON(w, http.StatusOK, domain.Salud{
		Estado:          "disponible",
		BaseDatos:       "ok",
		Version:         h.version,
		TiempoActivoSeg: uptime,
	})
}

func (h *APIHandler) OpenAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(voltrix.OpenAPISpec)
}

func (h *APIHandler) RedocDocs(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/docs" {
		writeError(w, r, http.StatusNotFound, "NO_ENCONTRADO", "Ruta no encontrada.", nil)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(voltrix.RedocHTML)
}

func (h *APIHandler) ListarOrdenes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var detalles []domain.ErrorDetalle

	filtro := domain.FiltroOrdenes{
		PlantaID:          strings.TrimSpace(q.Get("plantaId")),
		ActivoID:          strings.TrimSpace(q.Get("activoId")),
		Estado:            strings.TrimSpace(q.Get("estado")),
		Prioridad:         strings.TrimSpace(q.Get("prioridad")),
		TipoMantenimiento: strings.TrimSpace(q.Get("tipoMantenimiento")),
		Limit:             50,
		Offset:            0,
	}

	if filtro.PlantaID != "" && !domain.IdentificadorRegex.MatchString(filtro.PlantaID) {
		detalles = append(detalles, domain.ErrorDetalle{Campo: "plantaId", Problema: "identificador inválido"})
	}
	if filtro.ActivoID != "" && !domain.IdentificadorRegex.MatchString(filtro.ActivoID) {
		detalles = append(detalles, domain.ErrorDetalle{Campo: "activoId", Problema: "identificador inválido"})
	}
	if filtro.Estado != "" && !domain.IsValidEstado(filtro.Estado) {
		detalles = append(detalles, domain.ErrorDetalle{Campo: "estado", Problema: "debe ser programada, en_progreso, completada o cancelada"})
	}
	if filtro.Prioridad != "" && !domain.IsValidPrioridad(filtro.Prioridad) {
		detalles = append(detalles, domain.ErrorDetalle{Campo: "prioridad", Problema: "debe ser baja, media, alta o critica"})
	}
	if filtro.TipoMantenimiento != "" && !domain.IsValidTipoMantenimiento(filtro.TipoMantenimiento) {
		detalles = append(detalles, domain.ErrorDetalle{Campo: "tipoMantenimiento", Problema: "debe ser preventivo, predictivo o correctivo"})
	}

	if raw := strings.TrimSpace(q.Get("desde")); raw != "" {
		t, err := domain.ParseRFC3339WithTZ(raw)
		if err != nil {
			detalles = append(detalles, domain.ErrorDetalle{Campo: "desde", Problema: "debe ser una fecha RFC 3339 con zona horaria"})
		} else {
			filtro.Desde = &t
		}
	}
	if raw := strings.TrimSpace(q.Get("hasta")); raw != "" {
		t, err := domain.ParseRFC3339WithTZ(raw)
		if err != nil {
			detalles = append(detalles, domain.ErrorDetalle{Campo: "hasta", Problema: "debe ser una fecha RFC 3339 con zona horaria"})
		} else {
			filtro.Hasta = &t
		}
	}
	if filtro.Desde != nil && filtro.Hasta != nil && filtro.Hasta.Before(*filtro.Desde) {
		writeError(w, r, http.StatusBadRequest, "RANGO_FECHAS_INVALIDO", "La fecha 'hasta' debe ser posterior o igual a 'desde'.", []domain.ErrorDetalle{
			{Campo: "hasta", Problema: "debe ser posterior a desde"},
		})
		return
	}

	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 {
			detalles = append(detalles, domain.ErrorDetalle{Campo: "limit", Problema: "debe ser un entero entre 1 y 200"})
		} else {
			filtro.Limit = n
		}
	}
	if raw := strings.TrimSpace(q.Get("offset")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			detalles = append(detalles, domain.ErrorDetalle{Campo: "offset", Problema: "debe ser un entero mayor o igual a 0"})
		} else {
			filtro.Offset = n
		}
	}

	if len(detalles) > 0 {
		writeError(w, r, http.StatusBadRequest, "SOLICITUD_INVALIDA", "La solicitud no cumple el contrato.", detalles)
		return
	}

	page, err := h.svc.ListarOrdenes(r.Context(), filtro)
	if err != nil {
		h.logger.Error("error listando ordenes", slog.String("error", err.Error()))
		writeError(w, r, http.StatusServiceUnavailable, "NO_DISPONIBLE", "No hay conexión con la base de datos.", nil)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *APIHandler) ConsultarOrden(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !domain.IdentificadorRegex.MatchString(id) {
		writeError(w, r, http.StatusBadRequest, "SOLICITUD_INVALIDA", "Identificador de orden inválido.", []domain.ErrorDetalle{
			{Campo: "id", Problema: "debe cumplir ^[A-Za-z0-9_-]{1,64}$"},
		})
		return
	}

	orden, err := h.svc.ConsultarOrden(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, "NO_ENCONTRADO", fmt.Sprintf("La orden de mantenimiento %s no existe.", id), nil)
			return
		}
		h.logger.Error("error consultando orden", slog.String("error", err.Error()))
		writeError(w, r, http.StatusServiceUnavailable, "NO_DISPONIBLE", "No hay conexión con la base de datos.", nil)
		return
	}
	writeJSON(w, http.StatusOK, orden)
}

func (h *APIHandler) ActualizarOrden(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !domain.IdentificadorRegex.MatchString(id) {
		writeError(w, r, http.StatusBadRequest, "SOLICITUD_INVALIDA", "Identificador de orden inválido.", []domain.ErrorDetalle{
			{Campo: "id", Problema: "debe cumplir ^[A-Za-z0-9_-]{1,64}$"},
		})
		return
	}

	var req domain.SolicitudActualizacionOrden
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "SOLICITUD_INVALIDA", "El cuerpo JSON no cumple el contrato.", []domain.ErrorDetalle{
			{Campo: "body", Problema: err.Error()},
		})
		return
	}

	if req.Estado == nil && req.FechaProgramada == nil && req.CostoEstimado == nil && req.Observaciones == nil {
		writeError(w, r, http.StatusBadRequest, "SOLICITUD_INVALIDA", "Debe enviar al menos un campo a actualizar.", nil)
		return
	}

	var detalles []domain.ErrorDetalle
	if req.Estado != nil && !domain.IsValidEstado(*req.Estado) {
		detalles = append(detalles, domain.ErrorDetalle{Campo: "estado", Problema: "debe ser programada, en_progreso, completada o cancelada"})
	}
	if req.FechaProgramada != nil {
		if _, err := domain.ParseRFC3339WithTZ(*req.FechaProgramada); err != nil {
			detalles = append(detalles, domain.ErrorDetalle{Campo: "fechaProgramada", Problema: "debe ser una fecha RFC 3339 con zona horaria"})
		}
	}
	if req.CostoEstimado != nil && *req.CostoEstimado < 0 {
		detalles = append(detalles, domain.ErrorDetalle{Campo: "costoEstimado", Problema: "debe ser mayor o igual a 0"})
	}
	if req.Observaciones != nil && len(*req.Observaciones) > 500 {
		detalles = append(detalles, domain.ErrorDetalle{Campo: "observaciones", Problema: "no puede superar 500 caracteres"})
	}
	if len(detalles) > 0 {
		writeError(w, r, http.StatusBadRequest, "SOLICITUD_INVALIDA", "La solicitud no cumple el contrato.", detalles)
		return
	}

	updated, err := h.svc.ActualizarOrden(r.Context(), id, req)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			writeError(w, r, http.StatusNotFound, "NO_ENCONTRADO", fmt.Sprintf("La orden de mantenimiento %s no existe.", id), nil)
		case errors.Is(err, repository.ErrInvalidTransition):
			writeError(w, r, http.StatusConflict, "TRANSICION_INVALIDA", err.Error(), nil)
		default:
			h.logger.Error("error actualizando orden", slog.String("error", err.Error()))
			writeError(w, r, http.StatusServiceUnavailable, "NO_DISPONIBLE", "No hay conexión con la base de datos.", nil)
		}
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *APIHandler) GenerarOrdenes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var detalles []domain.ErrorDetalle

	modo := strings.TrimSpace(q.Get("modo"))
	if modo == "" {
		modo = "paralelo"
	}
	if modo != "secuencial" && modo != "paralelo" {
		detalles = append(detalles, domain.ErrorDetalle{Campo: "modo", Problema: "debe ser secuencial o paralelo"})
	}

	workers := 0
	if raw := strings.TrimSpace(q.Get("workers")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 64 {
			detalles = append(detalles, domain.ErrorDetalle{Campo: "workers", Problema: "debe ser un entero entre 1 y 64"})
		} else {
			workers = n
		}
	}

	var req domain.SolicitudGeneracionOrdenes
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "SOLICITUD_INVALIDA", "El cuerpo JSON no cumple el contrato.", []domain.ErrorDetalle{
			{Campo: "body", Problema: err.Error()},
		})
		return
	}

	desde, errDesde := domain.ParseRFC3339WithTZ(req.Desde)
	if errDesde != nil {
		detalles = append(detalles, domain.ErrorDetalle{Campo: "desde", Problema: "debe ser una fecha RFC 3339 con zona horaria"})
	}
	hasta, errHasta := domain.ParseRFC3339WithTZ(req.Hasta)
	if errHasta != nil {
		detalles = append(detalles, domain.ErrorDetalle{Campo: "hasta", Problema: "debe ser una fecha RFC 3339 con zona horaria"})
	}
	if errDesde == nil && errHasta == nil && !hasta.After(desde) {
		writeError(w, r, http.StatusBadRequest, "RANGO_FECHAS_INVALIDO", "La fecha 'hasta' debe ser posterior a 'desde'.", []domain.ErrorDetalle{
			{Campo: "hasta", Problema: "debe ser posterior a desde"},
		})
		return
	}

	for _, pid := range req.PlantaIDs {
		if !domain.IdentificadorRegex.MatchString(pid) {
			detalles = append(detalles, domain.ErrorDetalle{Campo: "plantaIds", Problema: "identificador inválido: " + pid})
		}
	}
	for _, aid := range req.ActivoIDs {
		if !domain.IdentificadorRegex.MatchString(aid) {
			detalles = append(detalles, domain.ErrorDetalle{Campo: "activoIds", Problema: "identificador inválido: " + aid})
		}
	}
	if req.UmbralRiesgo != nil && (*req.UmbralRiesgo < 0 || *req.UmbralRiesgo > 100) {
		detalles = append(detalles, domain.ErrorDetalle{Campo: "umbralRiesgo", Problema: "debe estar entre 0 y 100"})
	}
	if req.TamanoLote != nil && (*req.TamanoLote < 1 || *req.TamanoLote > 500) {
		detalles = append(detalles, domain.ErrorDetalle{Campo: "tamanoLote", Problema: "debe estar entre 1 y 500"})
	}

	if len(detalles) > 0 {
		writeError(w, r, http.StatusBadRequest, "SOLICITUD_INVALIDA", "La solicitud no cumple el contrato.", detalles)
		return
	}

	res, err := h.svc.GenerarOrdenes(r.Context(), req, desde, hasta, modo, workers)
	if err != nil {
		var depErr *client.DependencyError
		if errors.As(err, &depErr) {
			h.logger.Warn("dependencia_fallida",
				slog.String("equipo", depErr.Equipo),
				slog.String("endpoint", depErr.Endpoint),
				slog.String("error", depErr.Error()),
			)
			writeError(w, r, http.StatusBadGateway, "DEPENDENCIA_FALLIDA",
				fmt.Sprintf("Dependencia %s falló: %s", depErr.Equipo, depErr.Mensaje),
				[]domain.ErrorDetalle{{Campo: depErr.Equipo, Problema: depErr.Endpoint + " " + depErr.Mensaje}},
			)
			return
		}
		h.logger.Error("error generando ordenes", slog.String("error", err.Error()))
		writeError(w, r, http.StatusServiceUnavailable, "NO_DISPONIBLE", "Error accediendo a la base de datos.", nil)
		return
	}

	writeJSON(w, http.StatusOK, res)
}

