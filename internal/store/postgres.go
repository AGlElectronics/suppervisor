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

func (p *Postgres) ClaimTelegramUpdate(ctx context.Context, updateID int64) (bool, error) {
	command, err := p.Pool.Exec(ctx, `WITH pruned AS (
		DELETE FROM telegram_updates WHERE received_at < now() - interval '30 days'
	) INSERT INTO telegram_updates(update_id) VALUES($1) ON CONFLICT(update_id) DO NOTHING`, updateID)
	if err != nil {
		return false, err
	}
	return command.RowsAffected() == 1, nil
}

func (p *Postgres) Migrate(ctx context.Context) error {
	connection, err := p.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer connection.Release()
	const migrationLockID int64 = 7_219_743_338_774_111
	if _, err := connection.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockID); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = connection.Exec(unlockCtx, `SELECT pg_advisory_unlock($1)`, migrationLockID)
	}()
	if _, err := connection.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
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
		if err := connection.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&exists); err != nil {
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
		tx, err := connection.Begin(ctx)
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

func (p *Postgres) SeedDemo(ctx context.Context) error {
	_, err := p.Pool.Exec(ctx, `INSERT INTO decisions(id,request,recipe_id,score,violations,status,correction,created_at)
		VALUES(
			'00000000-0000-4000-8000-000000000001',
			'{"schemaVersion":1,"text":"Something cosy under 35 minutes, with two lunches","diners":["alan","partner"],"servings":2,"lunchPortions":2,"maxMinutes":35,"maxCostPence":1200,"equipment":["hob","oven"],"requireReheating":true,"assumptions":["standard hob and oven available"]}',
			'known-winner-tomato-orzo',
			'{"preference":32,"time":17,"cost":13,"leftovers":10,"novelty":9,"ingredients":11,"total":92}',
			'[]',
			'accepted',
			'',
			now() - interval '1 day'
		) ON CONFLICT(id) DO NOTHING;
		INSERT INTO decision_events(id,decision_id,event_type,actor,detail,created_at)
		VALUES(
			'00000000-0000-4000-8000-000000000002',
			'00000000-0000-4000-8000-000000000001',
			'accepted',
			'demo',
			'{"note":"development-only accepted meal"}',
			now() - interval '1 day'
		) ON CONFLICT(id) DO NOTHING;
		INSERT INTO workflow_states(decision_id)
		VALUES('00000000-0000-4000-8000-000000000001')
		ON CONFLICT(decision_id) DO NOTHING`)
	return err
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
	recipeSnapshotID := decision.Candidate.ID + ":" + decision.ID
	body, _ := json.Marshal(decision.Candidate)
	request, _ := json.Marshal(decision.Request)
	score, _ := json.Marshal(decision.Score)
	violations, _ := json.Marshal(decision.Violations)
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return decision, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO recipes(id,title,body,source) VALUES($1,$2,$3,'generated')`, recipeSnapshotID, decision.Candidate.Title, body); err != nil {
		return decision, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO decisions(id,request,recipe_id,score,violations,status) VALUES($1,$2,$3,$4,$5,$6)`, decision.ID, request, recipeSnapshotID, score, violations, decision.Status)
	if err != nil {
		return decision, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO decision_events(id,decision_id,event_type,actor,detail) VALUES($1,$2,'proposed','system',$3)`, newID(), decision.ID, score); err != nil {
		return decision, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO workflow_states(decision_id) VALUES($1)`, decision.ID); err != nil {
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

func (p *Postgres) DecisionByID(ctx context.Context, id string) (domain.Decision, error) {
	var decision domain.Decision
	var request, candidate, score, violations []byte
	err := p.Pool.QueryRow(ctx, `SELECT d.id,d.request,r.body,d.score,d.violations,d.status,d.correction,d.created_at
		FROM decisions d JOIN recipes r ON r.id=d.recipe_id WHERE d.id=$1`, id).
		Scan(&decision.ID, &request, &candidate, &score, &violations, &decision.Status, &decision.Correction, &decision.CreatedAt)
	if err != nil {
		return decision, err
	}
	if err := json.Unmarshal(request, &decision.Request); err != nil {
		return decision, err
	}
	if err := json.Unmarshal(candidate, &decision.Candidate); err != nil {
		return decision, err
	}
	if err := json.Unmarshal(score, &decision.Score); err != nil {
		return decision, err
	}
	if err := json.Unmarshal(violations, &decision.Violations); err != nil {
		return decision, err
	}
	return decision, nil
}

func (p *Postgres) RecentMeals(ctx context.Context, limit int) ([]domain.Candidate, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := p.Pool.Query(ctx, `SELECT r.body FROM decisions d JOIN recipes r ON r.id=d.recipe_id
		WHERE d.status IN ('accepted','cooked') ORDER BY d.updated_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Candidate
	for rows.Next() {
		var body []byte
		var candidate domain.Candidate
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &candidate); err != nil {
			return nil, err
		}
		out = append(out, candidate)
	}
	return out, rows.Err()
}

func (p *Postgres) DecisionEvents(ctx context.Context) ([]domain.DecisionEvent, error) {
	rows, err := p.Pool.Query(ctx, `SELECT id,decision_id,event_type,actor,detail,created_at FROM decision_events ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.DecisionEvent
	for rows.Next() {
		var event domain.DecisionEvent
		var detail []byte
		if err := rows.Scan(&event.ID, &event.DecisionID, &event.Type, &event.Actor, &detail, &event.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(detail, &event.Detail); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

func (p *Postgres) WorkflowStates(ctx context.Context) ([]domain.WorkflowState, error) {
	rows, err := p.Pool.Query(ctx, `SELECT decision_id,shopping_checked,cooking_step,updated_at FROM workflow_states ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.WorkflowState
	for rows.Next() {
		var state domain.WorkflowState
		if err := rows.Scan(&state.DecisionID, &state.ShoppingChecked, &state.CookingStep, &state.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, state)
	}
	return out, rows.Err()
}

func (p *Postgres) SaveWorkflowState(ctx context.Context, state domain.WorkflowState) (domain.WorkflowState, error) {
	checked, err := json.Marshal(state.ShoppingChecked)
	if err != nil {
		return state, err
	}
	err = p.Pool.QueryRow(ctx, `INSERT INTO workflow_states(decision_id,shopping_checked,cooking_step)
		VALUES($1,$2,$3)
		ON CONFLICT(decision_id) DO UPDATE SET shopping_checked=EXCLUDED.shopping_checked,cooking_step=EXCLUDED.cooking_step,updated_at=now()
		RETURNING updated_at`, state.DecisionID, checked, state.CookingStep).Scan(&state.UpdatedAt)
	return state, err
}

func (p *Postgres) SetDecisionStatus(ctx context.Context, id, status, correction string) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var foundID string
	err = tx.QueryRow(ctx, `UPDATE decisions SET status=$2,correction=$3,updated_at=now()
		WHERE id=$1 AND (status IS DISTINCT FROM $2 OR correction IS DISTINCT FROM $3) RETURNING id`, id, status, correction).Scan(&foundID)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if queryErr := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM decisions WHERE id=$1)`, id).Scan(&exists); queryErr != nil {
			return queryErr
		}
		if exists {
			return tx.Commit(ctx)
		}
		return pgx.ErrNoRows
	}
	if err != nil {
		return err
	}
	detail, _ := json.Marshal(map[string]string{"correction": correction})
	if _, err = tx.Exec(ctx, `INSERT INTO decision_events(id,decision_id,event_type,actor,detail) VALUES($1,$2,$3,'user',$4)`, newID(), id, status, detail); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) SaveRating(ctx context.Context, rating domain.Rating, candidate domain.Candidate) ([]domain.PreferenceEvent, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	ratingID := newID()
	tags, _ := json.Marshal(rating.Tags)
	err = tx.QueryRow(ctx, `INSERT INTO ratings(id,decision_id,profile_id,overall,taste,ease,reheat,tags,notes) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT(decision_id,profile_id) DO UPDATE SET overall=EXCLUDED.overall,taste=EXCLUDED.taste,ease=EXCLUDED.ease,reheat=EXCLUDED.reheat,tags=EXCLUDED.tags,notes=EXCLUDED.notes
		RETURNING id`,
		ratingID, rating.DecisionID, rating.ProfileID, rating.Overall, rating.Taste, rating.Ease, rating.Reheat, tags, rating.Notes).Scan(&ratingID)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT original.id FROM preference_events original
		LEFT JOIN preference_events reversal ON reversal.reversal_of=original.id
		WHERE original.rating_id=$1 AND original.reversal_of IS NULL AND reversal.id IS NULL`, ratingID)
	if err != nil {
		return nil, err
	}
	var superseded []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		superseded = append(superseded, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range superseded {
		if _, err := compensatePreferenceEvent(ctx, tx, id, "superseded by an updated rating"); err != nil {
			return nil, err
		}
	}
	var events []domain.PreferenceEvent
	record := func(key string, delta float64, reason string) error {
		event := domain.PreferenceEvent{ID: newID(), ProfileID: rating.ProfileID, Key: strings.ToLower(key), Delta: delta, Reason: reason, CreatedAt: time.Now().UTC()}
		_, err = tx.Exec(ctx, `INSERT INTO preference_events(id,profile_id,key,delta,reason,rating_id) VALUES($1,$2,$3,$4,$5,$6)`, event.ID, event.ProfileID, event.Key, event.Delta, event.Reason, ratingID)
		if err != nil {
			return err
		}
		events = append(events, event)
		return nil
	}
	for _, tag := range candidate.Tags {
		if err := record(tag, float64(rating.Taste-3)*.25, fmt.Sprintf("taste rating %d/5 for %s", rating.Taste, candidate.Title)); err != nil {
			return nil, err
		}
	}
	if err := record("ease", float64(rating.Ease-3)*.2, fmt.Sprintf("ease rating %d/5 for %s", rating.Ease, candidate.Title)); err != nil {
		return nil, err
	}
	if err := record("reheat", float64(rating.Reheat-3)*.2, fmt.Sprintf("reheat rating %d/5 for %s", rating.Reheat, candidate.Title)); err != nil {
		return nil, err
	}
	for _, tag := range rating.Tags {
		if err := record("feedback:"+tag, float64(rating.Overall-3)*.15, fmt.Sprintf("feedback tag for %s", candidate.Title)); err != nil {
			return nil, err
		}
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
	reversal, err := compensatePreferenceEvent(ctx, tx, id, "reverted by user")
	if err != nil {
		return err
	}
	err = refreshPreferences(ctx, tx, reversal.ProfileID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func compensatePreferenceEvent(ctx context.Context, tx pgx.Tx, id, reason string) (domain.PreferenceEvent, error) {
	reversalID := newID()
	var event domain.PreferenceEvent
	err := tx.QueryRow(ctx, `INSERT INTO preference_events(id,profile_id,key,delta,reason,rating_id,reversal_of)
		SELECT $1,original.profile_id,original.key,-original.delta,$3,original.rating_id,original.id
		FROM preference_events original
		WHERE original.id=$2 AND original.reversal_of IS NULL
		  AND NOT EXISTS(SELECT 1 FROM preference_events existing WHERE existing.reversal_of=original.id)
		RETURNING id,profile_id,key,delta,reason,reversal_of,created_at`, reversalID, id, reason).
		Scan(&event.ID, &event.ProfileID, &event.Key, &event.Delta, &event.Reason, &event.ReversalOf, &event.CreatedAt)
	return event, err
}

func refreshPreferences(ctx context.Context, tx pgx.Tx, profileID string) error {
	_, err := tx.Exec(ctx, `UPDATE profiles SET preferences=(
		SELECT COALESCE(jsonb_object_agg(key,value),'{}') FROM (
			SELECT key, SUM(value)::numeric AS value FROM (
				SELECT key, value::numeric FROM profiles p, jsonb_each_text(p.base_preferences) WHERE p.id=$1
				UNION ALL
				SELECT key, delta FROM preference_events WHERE profile_id=$1
			) values_by_key GROUP BY key
		) combined
	),updated_at=now() WHERE id=$1`, profileID)
	return err
}

func (p *Postgres) PreferenceEvents(ctx context.Context) ([]domain.PreferenceEvent, error) {
	rows, err := p.Pool.Query(ctx, `SELECT event.id,event.profile_id,event.key,event.delta,event.reason,event.reversal_of,reversal.created_at,event.created_at
		FROM preference_events event
		LEFT JOIN preference_events reversal ON reversal.reversal_of=event.id
		ORDER BY event.created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PreferenceEvent
	for rows.Next() {
		var event domain.PreferenceEvent
		if err := rows.Scan(&event.ID, &event.ProfileID, &event.Key, &event.Delta, &event.Reason, &event.ReversalOf, &event.RevertedAt, &event.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

func (p *Postgres) Ratings(ctx context.Context) ([]domain.Rating, error) {
	rows, err := p.Pool.Query(ctx, `SELECT decision_id,profile_id,overall,taste,ease,reheat,tags,notes FROM ratings ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Rating
	for rows.Next() {
		var rating domain.Rating
		if err := rows.Scan(&rating.DecisionID, &rating.ProfileID, &rating.Overall, &rating.Taste, &rating.Ease, &rating.Reheat, &rating.Tags, &rating.Notes); err != nil {
			return nil, err
		}
		out = append(out, rating)
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
