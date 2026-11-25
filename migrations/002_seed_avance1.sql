-- Datos de ejemplo idempotentes para Avance 1, alineados con openapi.yaml
-- y con los contratos de Equipo 1 (PL-001, PL-002) y Equipo 7 (órdenes completadas en 2026-11).

INSERT INTO ordenes_mantenimiento (
    id, lote_id, planta_id, activo_id, tipo_activo, tipo_mantenimiento,
    prioridad, estado, puntaje_riesgo, motivo, costo_estimado, moneda,
    fecha_programada, generado_en, actualizado_en, observaciones, factores_evaluados
) VALUES
(
    'OM-20261119-001',
    '7b4e1c9d-2f5a-4d8e-a1b3-6c9d0e2f4a8b',
    'PL-001',
    'PL-001-PS-07',
    'panel_solar',
    'predictivo',
    'alta',
    'programada',
    78.50,
    'Caída sostenida de generación (-24.3%) y 2 alertas críticas activas.',
    450000.00,
    'COP',
    '2026-11-22T09:00:00-05:00',
    '2026-11-19T14:00:00Z',
    NULL,
    NULL,
    '{"estadoActivo":"degradado","alertasActivas":3,"alertasCriticas":2,"eficienciaHistoricaPct":75.7,"desviacionPronosticoPct":-18.2}'::jsonb
),
(
    'OM-20261115-002',
    '7b4e1c9d-2f5a-4d8e-a1b3-6c9d0e2f4a8b',
    'PL-001',
    'PL-001-PS-03',
    'panel_solar',
    'preventivo',
    'baja',
    'completada',
    56.00,
    'Limpieza preventiva de arreglo fotovoltaico por pérdida leve de eficiencia (-12.0%).',
    360000.00,
    'COP',
    '2026-11-16T08:00:00-05:00',
    '2026-11-15T10:30:00Z',
    '2026-11-16T15:10:00Z',
    'Limpieza completada y verificación de inversor dentro de parámetros nominales.',
    '{"estadoActivo":"operativo","alertasActivas":1,"alertasCriticas":0,"eficienciaHistoricaPct":88.0,"desviacionPronosticoPct":-6.5}'::jsonb
),
(
    'OM-20261119-003',
    '7b4e1c9d-2f5a-4d8e-a1b3-6c9d0e2f4a8b',
    'PL-002',
    'PL-002-BB-01',
    'banco_baterias',
    'correctivo',
    'critica',
    'en_progreso',
    92.00,
    'Sobrecalentamiento recurrente en banco de baterías y estado fuera de servicio.',
    848000.00,
    'COP',
    '2026-11-20T08:00:00-05:00',
    '2026-11-19T14:05:00Z',
    '2026-11-20T09:15:00Z',
    'Cuadrilla técnica en sitio revisando sistema de refrigeración.',
    '{"estadoActivo":"fuera_de_servicio","alertasActivas":5,"alertasCriticas":4,"eficienciaHistoricaPct":58.0,"desviacionPronosticoPct":-31.5}'::jsonb
),
(
    'OM-20261112-004',
    '7b4e1c9d-2f5a-4d8e-a1b3-6c9d0e2f4a8b',
    'PL-002',
    'PL-002-TB-02',
    'turbina_eolica',
    'preventivo',
    'baja',
    'completada',
    58.40,
    'Inspección preventiva de rodamientos y calibración de anemómetro.',
    650000.00,
    'COP',
    '2026-11-13T07:30:00-05:00',
    '2026-11-12T16:00:00Z',
    '2026-11-13T17:00:00Z',
    'Calibración finalizada sin novedades.',
    '{"estadoActivo":"operativo","alertasActivas":1,"alertasCriticas":0,"eficienciaHistoricaPct":84.5,"desviacionPronosticoPct":-8.0}'::jsonb
)
ON CONFLICT (id) DO NOTHING;

