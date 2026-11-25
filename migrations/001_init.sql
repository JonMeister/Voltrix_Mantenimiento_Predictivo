CREATE TABLE IF NOT EXISTS ordenes_mantenimiento (
    id                  VARCHAR(64)    PRIMARY KEY,
    lote_id             UUID           NULL,
    planta_id           VARCHAR(64)    NOT NULL,
    activo_id           VARCHAR(64)    NOT NULL,
    tipo_activo         VARCHAR(32)    NOT NULL,
    tipo_mantenimiento  VARCHAR(32)    NOT NULL,
    prioridad           VARCHAR(16)    NOT NULL,
    estado              VARCHAR(16)    NOT NULL,
    puntaje_riesgo      NUMERIC(5,2)   NOT NULL CHECK (puntaje_riesgo >= 0 AND puntaje_riesgo <= 100),
    motivo              VARCHAR(500)   NOT NULL,
    costo_estimado      NUMERIC(14,2)  NOT NULL CHECK (costo_estimado >= 0),
    moneda              VARCHAR(8)     NOT NULL DEFAULT 'COP',
    fecha_programada    TIMESTAMPTZ    NOT NULL,
    generado_en         TIMESTAMPTZ    NOT NULL,
    actualizado_en      TIMESTAMPTZ    NULL,
    observaciones       VARCHAR(500)   NULL,
    factores_evaluados  JSONB          NULL
);

CREATE INDEX IF NOT EXISTS idx_ordenes_planta_generado
    ON ordenes_mantenimiento (planta_id, generado_en DESC);

CREATE INDEX IF NOT EXISTS idx_ordenes_estado_prioridad
    ON ordenes_mantenimiento (estado, prioridad);

CREATE INDEX IF NOT EXISTS idx_ordenes_activo
    ON ordenes_mantenimiento (activo_id);

