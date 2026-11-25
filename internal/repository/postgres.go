package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/internal/domain"
	"github.com/JonMeister/Voltrix_Mantenimiento_Predictivo/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound          = errors.New("orden de mantenimiento no encontrada")
	ErrInvalidTransition = errors.New("transicion de estado invalida")
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("error parseando DATABASE_URL: %w", err)
	}
	// Pool conservador pensado para EC2 de 1 GiB RAM y max_connections=30
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("error creando pool PostgreSQL: %w", err)
	}
	return &PostgresRepository{pool: pool}, nil
}

func (r *PostgresRepository) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}

func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *PostgresRepository) RunMigrations(ctx context.Context, seedDemo bool) error {
	initSQL, err := migrations.Files.ReadFile("001_init.sql")
	if err != nil {
		return fmt.Errorf("error leyendo 001_init.sql: %w", err)
	}
	if _, err := r.pool.Exec(ctx, string(initSQL)); err != nil {
		return fmt.Errorf("error ejecutando 001_init.sql: %w", err)
	}
	if seedDemo {
		seedSQL, err := migrations.Files.ReadFile("002_seed_avance1.sql")
		if err != nil {
			return fmt.Errorf("error leyendo 002_seed_avance1.sql: %w", err)
		}
		if _, err := r.pool.Exec(ctx, string(seedSQL)); err != nil {
			return fmt.Errorf("error ejecutando 002_seed_avance1.sql: %w", err)
		}
	}
	return nil
}

func (r *PostgresRepository) List(ctx context.Context, f domain.FiltroOrdenes) (domain.PaginaOrdenesMantenimiento, error) {
	where := make([]string, 0, 7)
	args := make([]any, 0, 9)
	idx := 1

	if f.PlantaID != "" {
		where = append(where, fmt.Sprintf("planta_id = $%d", idx))
		args = append(args, f.PlantaID)
		idx++
	}
	if f.ActivoID != "" {
		where = append(where, fmt.Sprintf("activo_id = $%d", idx))
		args = append(args, f.ActivoID)
		idx++
	}
	if f.Estado != "" {
		where = append(where, fmt.Sprintf("estado = $%d", idx))
		args = append(args, f.Estado)
		idx++
	}
	if f.Prioridad != "" {
		where = append(where, fmt.Sprintf("prioridad = $%d", idx))
		args = append(args, f.Prioridad)
		idx++
	}
	if f.TipoMantenimiento != "" {
		where = append(where, fmt.Sprintf("tipo_mantenimiento = $%d", idx))
		args = append(args, f.TipoMantenimiento)
		idx++
	}
	if f.Desde != nil {
		where = append(where, fmt.Sprintf("generado_en >= $%d", idx))
		args = append(args, *f.Desde)
		idx++
	}
	if f.Hasta != nil {
		where = append(where, fmt.Sprintf("generado_en <= $%d", idx))
		args = append(args, *f.Hasta)
		idx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	var costoTotal float64
	aggQuery := fmt.Sprintf("SELECT COUNT(*), COALESCE(SUM(costo_estimado), 0) FROM ordenes_mantenimiento %s", whereClause)
	if err := r.pool.QueryRow(ctx, aggQuery, args...).Scan(&total, &costoTotal); err != nil {
		return domain.PaginaOrdenesMantenimiento{}, err
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, planta_id, activo_id, tipo_activo, tipo_mantenimiento,
		       prioridad, estado, puntaje_riesgo, motivo, costo_estimado, moneda,
		       fecha_programada, generado_en, actualizado_en, observaciones, factores_evaluados
		FROM ordenes_mantenimiento
		%s
		ORDER BY generado_en DESC, id ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, idx, idx+1)

	dataArgs := append(append([]any{}, args...), f.Limit, f.Offset)
	rows, err := r.pool.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return domain.PaginaOrdenesMantenimiento{}, err
	}
	defer rows.Close()

	datos := make([]domain.OrdenMantenimiento, 0)
	for rows.Next() {
		o, err := scanOrden(rows)
		if err != nil {
			return domain.PaginaOrdenesMantenimiento{}, err
		}
		datos = append(datos, o)
	}
	if err := rows.Err(); err != nil {
		return domain.PaginaOrdenesMantenimiento{}, err
	}

	return domain.PaginaOrdenesMantenimiento{
		Datos:              datos,
		Total:              total,
		CostoTotalEstimado: costoTotal,
		Limit:              f.Limit,
		Offset:             f.Offset,
	}, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (domain.OrdenMantenimiento, error) {
	query := `
		SELECT id, planta_id, activo_id, tipo_activo, tipo_mantenimiento,
		       prioridad, estado, puntaje_riesgo, motivo, costo_estimado, moneda,
		       fecha_programada, generado_en, actualizado_en, observaciones, factores_evaluados
		FROM ordenes_mantenimiento
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)
	o, err := scanOrden(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OrdenMantenimiento{}, ErrNotFound
		}
		return domain.OrdenMantenimiento{}, err
	}
	return o, nil
}

func (r *PostgresRepository) Patch(ctx context.Context, id string, req domain.SolicitudActualizacionOrden) (domain.OrdenMantenimiento, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.OrdenMantenimiento{}, err
	}
	defer tx.Rollback(ctx)

	var currentEstado string
	var currentFecha time.Time
	var currentCosto float64
	var currentObs *string

	err = tx.QueryRow(ctx, `
		SELECT estado, fecha_programada, costo_estimado, observaciones
		FROM ordenes_mantenimiento
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(&currentEstado, &currentFecha, &currentCosto, &currentObs)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OrdenMantenimiento{}, ErrNotFound
		}
		return domain.OrdenMantenimiento{}, err
	}

	if domain.IsTerminalEstado(currentEstado) {
		return domain.OrdenMantenimiento{}, fmt.Errorf("%w: la orden %s ya está %s y no admite cambios", ErrInvalidTransition, id, currentEstado)
	}

	nextEstado := currentEstado
	if req.Estado != nil {
		if !domain.CanTransition(currentEstado, *req.Estado) {
			return domain.OrdenMantenimiento{}, fmt.Errorf("%w: no se permite transicionar de %s a %s", ErrInvalidTransition, currentEstado, *req.Estado)
		}
		nextEstado = *req.Estado
	}

	nextFecha := currentFecha
	if req.FechaProgramada != nil {
		parsed, err := domain.ParseRFC3339WithTZ(*req.FechaProgramada)
		if err != nil {
			return domain.OrdenMantenimiento{}, err
		}
		nextFecha = parsed
	}

	nextCosto := currentCosto
	if req.CostoEstimado != nil {
		nextCosto = *req.CostoEstimado
	}

	nextObs := currentObs
	if req.Observaciones != nil {
		nextObs = req.Observaciones
	}

	now := time.Now().UTC()
	row := tx.QueryRow(ctx, `
		UPDATE ordenes_mantenimiento
		SET estado = $1,
		    fecha_programada = $2,
		    costo_estimado = $3,
		    observaciones = $4,
		    actualizado_en = $5
		WHERE id = $6
		RETURNING id, planta_id, activo_id, tipo_activo, tipo_mantenimiento,
		          prioridad, estado, puntaje_riesgo, motivo, costo_estimado, moneda,
		          fecha_programada, generado_en, actualizado_en, observaciones, factores_evaluados
	`, nextEstado, nextFecha, nextCosto, nextObs, now, id)

	updated, err := scanOrden(row)
	if err != nil {
		return domain.OrdenMantenimiento{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.OrdenMantenimiento{}, err
	}
	return updated, nil
}

func (r *PostgresRepository) BatchInsert(ctx context.Context, loteID string, ordenes []domain.OrdenMantenimiento) error {
	if len(ordenes) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	batch := &pgx.Batch{}
	const insertSQL = `
		INSERT INTO ordenes_mantenimiento (
			id, lote_id, planta_id, activo_id, tipo_activo, tipo_mantenimiento,
			prioridad, estado, puntaje_riesgo, motivo, costo_estimado, moneda,
			fecha_programada, generado_en, actualizado_en, observaciones, factores_evaluados
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, NULL, NULL, $15
		)
	`

	for _, o := range ordenes {
		fechaProg, err := domain.ParseRFC3339WithTZ(o.FechaProgramada)
		if err != nil {
			return err
		}
		genEn, err := domain.ParseRFC3339WithTZ(o.GeneradoEn)
		if err != nil {
			return err
		}
		var factoresJSON []byte
		if o.FactoresEvaluados != nil {
			factoresJSON, err = json.Marshal(o.FactoresEvaluados)
			if err != nil {
				return err
			}
		}
		batch.Queue(insertSQL,
			o.ID, loteID, o.PlantaID, o.ActivoID, o.TipoActivo, o.TipoMantenimiento,
			o.Prioridad, o.Estado, o.PuntajeRiesgo, o.Motivo, o.CostoEstimado, o.Moneda,
			fechaProg, genEn, factoresJSON,
		)
	}

	br := tx.SendBatch(ctx, batch)
	for range ordenes {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return err
		}
	}
	if err := br.Close(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanOrden(s rowScanner) (domain.OrdenMantenimiento, error) {
	var o domain.OrdenMantenimiento
	var fechaProg, genEn time.Time
	var actEn *time.Time
	var obs *string
	var factoresRaw []byte

	err := s.Scan(
		&o.ID, &o.PlantaID, &o.ActivoID, &o.TipoActivo, &o.TipoMantenimiento,
		&o.Prioridad, &o.Estado, &o.PuntajeRiesgo, &o.Motivo, &o.CostoEstimado, &o.Moneda,
		&fechaProg, &genEn, &actEn, &obs, &factoresRaw,
	)
	if err != nil {
		return domain.OrdenMantenimiento{}, err
	}

	bogotaLoc := time.FixedZone("COT", -5*3600)
	o.FechaProgramada = fechaProg.In(bogotaLoc).Format(time.RFC3339)
	o.GeneradoEn = genEn.UTC().Format(time.RFC3339)
	if actEn != nil {
		formatted := actEn.UTC().Format(time.RFC3339)
		o.ActualizadoEn = &formatted
	}
	o.Observaciones = obs
	if len(factoresRaw) > 0 {
		var f domain.FactoresEvaluados
		if err := json.Unmarshal(factoresRaw, &f); err == nil {
			o.FactoresEvaluados = &f
		}
	}
	return o, nil
}

