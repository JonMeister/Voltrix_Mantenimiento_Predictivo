package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestObtenerAlertas_ContratoRealEquipo3(t *testing.T) {
	// Ejemplo exacto tomado de Voltrix_Otros/AlertasNotificaciones.yaml (GET /alertas/estado)
	const payload = `{
		"generadoEn": "2026-11-25T13:06:00Z",
		"totalActivas": 2,
		"porSeveridad": { "advertencia": 1, "critica": 1 },
		"resumenPorActivo": [
			{ "plantaId": "PL-002", "activoId": "PL-002-BB-01", "activas": 1, "criticas": 1 },
			{ "plantaId": "PL-001", "activoId": "PL-001-PS-07", "activas": 1, "criticas": 0 }
		],
		"datos": [
			{
				"id": "ALR-20261125-001",
				"plantaId": "PL-002",
				"activoId": "PL-002-BB-01",
				"tipoActivo": "banco_baterias",
				"metrica": "temperatura",
				"valorObservado": 46.3,
				"umbralAplicado": 45,
				"severidad": "critica",
				"estado": "activa",
				"ocurrencias": 3,
				"primeraDeteccion": "2026-11-25T13:05:00Z",
				"ultimaDeteccion": "2026-11-25T13:05:20Z"
			},
			{
				"id": "ALR-20261125-002",
				"plantaId": "PL-001",
				"activoId": "PL-001-PS-07",
				"tipoActivo": "panel_solar",
				"metrica": "caida_potencia",
				"valorObservado": 27.5,
				"umbralAplicado": 20,
				"severidad": "advertencia",
				"estado": "activa",
				"ocurrencias": 1,
				"primeraDeteccion": "2026-11-25T13:05:10Z",
				"ultimaDeteccion": "2026-11-25T13:05:10Z"
			}
		],
		"total": 2,
		"limit": 50,
		"offset": 0
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/alertas/estado" {
			t.Errorf("ruta inesperada: %s", r.URL.Path)
		}
		if r.Header.Get("X-Equipo-Consumidor") != "equipo-6" {
			t.Errorf("header X-Equipo-Consumidor esperado equipo-6, recibido %q", r.Header.Get("X-Equipo-Consumidor"))
		}
		if r.URL.Query().Get("estado") != "activa" {
			t.Errorf("query estado esperado activa, recibido %q", r.URL.Query().Get("estado"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := NewHTTPUpstreamClient("", srv.URL, "", "", 2*time.Second)
	got, err := c.ObtenerAlertas(context.Background(), nil)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	bb := got["PL-002-BB-01"]
	if bb.AlertasActivas != 1 || bb.AlertasCriticas != 1 || bb.UltimaCondicion != "temperatura" {
		t.Errorf("PL-002-BB-01 incorrecto (posible doble conteo o campo mal mapeado): %+v", bb)
	}

	ps := got["PL-001-PS-07"]
	if ps.AlertasActivas != 1 || ps.AlertasCriticas != 0 || ps.UltimaCondicion != "caida_potencia" {
		t.Errorf("PL-001-PS-07 incorrecto (posible doble conteo o campo mal mapeado): %+v", ps)
	}
}

