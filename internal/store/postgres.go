package store

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/AGlElectronics/suppervisor/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type Postgres struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, databaseURL string) (*Postgres, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	config.MaxConns = 12
	config.MinConns = 1
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute
	var pool *pgxpool.Pool
	for attempt := 0; attempt < 8; attempt++ {
		pool, err = pgxpool.NewWithConfig(ctx, config)
		if err == nil {
			err = pool.Ping(ctx)
		}
		if err == nil {
			return &Postgres{Pool: pool}, nil
		}
		if pool != nil {
			pool.Close()
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
		}
	}
	return nil, fmt.Errorf("connect to postgres: %w", err)
}

func (p *Postgres) Close()                          { p.Pool.Close() }
func (p *Postgres) Ready(ctx context.Context) error { return p.Pool.Ping(ctx) }

func (p *Postgres) Migrate(ctx context.Context) error {
	if _, err := p.Pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	entries, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(entries)
	for _, path := range entries {
		version := strings.TrimSuffix(strings.TrimPrefix(path, "migrations/"), ".sql")
		var exists bool
		if err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		raw, err := migrationFS.ReadFile(path)
		if err != nil {
			return err
		}
		up, _, err := SplitMigration(string(raw))
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		tx, err := p.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, up); err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply %s: %w", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func SplitMigration(sql string) (up, down string, err error) {
	const upMark, downMark = "-- +suppervisor Up", "-- +suppervisor Down"
	upAt, downAt := strings.Index(sql, upMark), strings.Index(sql, downMark)
	if upAt < 0 || downAt < 0 || downAt <= upAt {
		return "", "", errors.New("migration must contain ordered Up and Down sections")
	}
	up = strings.TrimSpace(sql[upAt+len(upMark) : downAt])
	down = strings.TrimSpace(sql[downAt+len(downMark):])
	if up == "" || down == "" {
		return "", "", errors.New("migration sections may not be empty")
	}
	return up, down, nil
}

func (p *Postgres) Profiles(ctx context.Context) ([]domain.Profile, error) {
	rows, err := p.Pool.Query(ctx, `SELECT id,name,dislikes,likes,preferences FROM profiles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Profile
	for rows.Next() {
		var profile domain.Profile
		if err := rows.Scan(&profile.ID, &profile.Name, &profile.Dislikes, &profile.Likes, &profile.Preferences); err != nil {
			return nil, err
		}
		out = append(out, profile)
	}
	return out, rows.Err()
}

func (p *Postgres) Recipes(ctx context.Context) ([]domain.Candidate, error) {
	rows, err := p.Pool.Query(ctx, `SELECT body FROM recipes ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Candidate
	for rows.Next() {
		var raw []byte
		var candidate domain.Candidate
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &candidate); err != nil {
			return nil, err
		}
		out = append(out, candidate)
	}
	return out, rows.Err()
}

func (p *Postgres) SaveDecision(ctx context.Context, decision domain.Decision) (domain.Decision, error) {
	if decision.ID == "" {
		decision.ID = newID()
	}
	decision.CreatedAt = time.Now().UTC()
	if decision.Candidate.ID == "" {
		decision.Candidate.ID = "generated-" + strings.ToLower(strings.ReplaceAll(decision.ID, "-", ""))
	}
	body, _ := json.Marshal(decision.Candidate)
	request, _ := json.Marshal(decision.Request)
	score, _ := json.Marshal(decision.Score)
	violations, _ := json.Marshal(decision.Violations)
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return decision, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO recipes(id,title,body,source) VALUES($1,$2,$3,'generated') ON CONFLICT(id) DO UPDATE SET title=EXCLUDED.title, body=EXCLUDED.body, updated_at=now()`, decision.Candidate.ID, decision.Candidate.Title, body); err != nil {
		return decision, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO decisions(id,request,recipe_id,score,violations,status) VALUES($1,$2,$3,$4,$5,$6)`, decision.ID, request, decision.Candidate.ID, score, violations, decision.Status)
	if err != nil {
		return decision, err
	}
	return decision, tx.Commit(ctx)
}

func (p *Postgres) Decisions(ctx context.Context, limit int) ([]domain.Decision, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	rows, err := p.Pool.Query(ctx, `SELECT d.id,d.request,r.body,d.score,d.violations,d.status,d.correction,d.created_at FROM decisions d JOIN recipes r ON r.id=d.recipe_id ORDER BY d.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Decision
	for rows.Next() {
		var d domain.Decision
		var request, candidate, score, violations []byte
		if err := rows.Scan(&d.ID, &request, &candidate, &score, &violations, &d.Status, &d.Correction, &d.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(request, &d.Request); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(candidate, &d.Candidate); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(score, &d.Score); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(violations, &d.Violations); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (p *Postgres) SetDecisionStatus(ctx context.Context, id, status, correction string) error {
	command, err := p.Pool.Exec(ctx, `UPDATE decisions SET status=$2,correction=$3,updated_at=now() WHERE id=$1`, id, status, correction)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return nil
}

func (p *Postgres) SaveRating(ctx context.Context, rating domain.Rating, candidate domain.Candidate) ([]domain.PreferenceEvent, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	ratingID := newID()
	tags, _ := json.Marshal(rating.Tags)
	_, err = tx.Exec(ctx, `INSERT INTO ratings(id,decision_id,profile_id,overall,taste,ease,reheat,tags,notes) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT(decision_id,profile_id) DO UPDATE SET overall=EXCLUDED.overall,taste=EXCLUDED.taste,ease=EXCLUDED.ease,reheat=EXCLUDED.reheat,tags=EXCLUDED.tags,notes=EXCLUDED.notes`,
		ratingID, rating.DecisionID, rating.ProfileID, rating.Overall, rating.Taste, rating.Ease, rating.Reheat, tags, rating.Notes)
	if err != nil {
		return nil, err
	}
	delta := float64(rating.Overall-3) * .25
	var events []domain.PreferenceEvent
	for _, tag := range candidate.Tags {
		event := domain.PreferenceEvent{ID: newID(), ProfileID: rating.ProfileID, Key: strings.ToLower(tag), Delta: delta, Reason: fmt.Sprintf("rating %d/5 for %s", rating.Overall, candidate.Title), CreatedAt: time.Now().UTC()}
		_, err = tx.Exec(ctx, `INSERT INTO preference_events(id,profile_id,key,delta,reason,rating_id) VALUES($1,$2,$3,$4,$5,$6)`, event.ID, event.ProfileID, event.Key, event.Delta, event.Reason, ratingID)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err = refreshPreferences(ctx, tx, rating.ProfileID); err != nil {
		return nil, err
	}
	return events, tx.Commit(ctx)
}

func (p *Postgres) RevertPreferenceEvent(ctx context.Context, id string) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var profileID string
	if err := tx.QueryRow(ctx, `UPDATE preference_events SET reverted_at=now() WHERE id=$1 AND reverted_at IS NULL RETURNING profile_id`, id).Scan(&profileID); err != nil {
		return err
	}
	err = refreshPreferences(ctx, tx, profileID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func refreshPreferences(ctx context.Context, tx pgx.Tx, profileID string) error {
	_, err := tx.Exec(ctx, `UPDATE profiles SET preferences=(
		SELECT COALESCE(jsonb_object_agg(key,value),'{}') FROM (
			SELECT key, SUM(value)::numeric AS value FROM (
				SELECT key, value::numeric FROM profiles p, jsonb_each_text(p.base_preferences) WHERE p.id=$1
				UNION ALL
				SELECT key, delta FROM preference_events WHERE profile_id=$1 AND reverted_at IS NULL
			) values_by_key GROUP BY key
		) combined
	),updated_at=now() WHERE id=$1`, profileID)
	return err
}

func (p *Postgres) PreferenceEvents(ctx context.Context) ([]domain.PreferenceEvent, error) {
	rows, err := p.Pool.Query(ctx, `SELECT id,profile_id,key,delta,reason,reverted_at,created_at FROM preference_events ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PreferenceEvent
	for rows.Next() {
		var event domain.PreferenceEvent
		if err := rows.Scan(&event.ID, &event.ProfileID, &event.Key, &event.Delta, &event.Reason, &event.RevertedAt, &event.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

func newID() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	dst := make([]byte, 36)
	hex.Encode(dst[0:8], value[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], value[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], value[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], value[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], value[10:16])
	return string(dst)
}
