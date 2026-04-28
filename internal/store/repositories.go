package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"runloop/internal/dispatch"
	"runloop/internal/inbox"
	"runloop/internal/runs"
	"runloop/internal/sources"
	"runloop/internal/workflows"
)

func (s *Store) UpsertInboxItem(ctx context.Context, c sources.InboxCandidate) (inbox.InboxItem, inbox.InboxItemVersion, bool, error) {
	hash, err := inbox.HashPayload(c.RawPayload, c.Normalized)
	if err != nil {
		return inbox.InboxItem{}, inbox.InboxItemVersion{}, false, err
	}
	raw, _ := json.Marshal(c.RawPayload)
	normalized, _ := json.Marshal(c.Normalized)
	var item inbox.InboxItem
	var version inbox.InboxItemVersion
	changed := false
	err = withTx(ctx, s.db, func(tx *sql.Tx) error {
		ts := now()
		_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO sources(id, type, enabled, created_at) VALUES(?, ?, 1, ?)`, c.SourceID, c.SourceID, ts)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO inbox_items(source_id, external_id, entity_type, title, created_at, updated_at)
			VALUES(?, ?, ?, ?, ?, ?)
			ON CONFLICT(source_id, external_id) DO UPDATE SET title=excluded.title, entity_type=excluded.entity_type, updated_at=excluded.updated_at`,
			c.SourceID, c.ExternalID, c.EntityType, c.Title, ts, ts)
		if err != nil {
			return err
		}
		item, err = scanInboxItem(tx.QueryRowContext(ctx, `SELECT id, source_id, external_id, entity_type, title, archived_at, ignored_at, created_at, updated_at FROM inbox_items WHERE source_id=? AND external_id=?`, c.SourceID, c.ExternalID))
		if err != nil {
			return err
		}
		var latestHash string
		var latestVersion int
		err = tx.QueryRowContext(ctx, `SELECT payload_hash, version FROM inbox_item_versions WHERE inbox_item_id=? ORDER BY version DESC LIMIT 1`, item.ID).Scan(&latestHash, &latestVersion)
		if errors.Is(err, sql.ErrNoRows) {
			latestVersion = 0
		} else if err != nil {
			return err
		}
		if latestHash == hash {
			version, err = s.latestInboxVersionTx(ctx, tx, item.ID)
			return err
		}
		changed = true
		observed := c.ObservedAt.UTC().Format(time.RFC3339Nano)
		if c.ObservedAt.IsZero() {
			observed = ts
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO inbox_item_versions(inbox_item_id, version, raw_payload, normalized, payload_hash, observed_at, created_at) VALUES(?, ?, ?, ?, ?, ?, ?)`,
			item.ID, latestVersion+1, string(raw), string(normalized), hash, observed, ts)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		version, err = s.getInboxVersionTx(ctx, tx, id)
		return err
	})
	return item, version, changed, err
}

func (s *Store) GetInboxItem(ctx context.Context, id int64) (inbox.InboxItem, error) {
	return scanInboxItem(s.db.QueryRowContext(ctx, `SELECT id, source_id, external_id, entity_type, title, archived_at, ignored_at, created_at, updated_at FROM inbox_items WHERE id=?`, id))
}

func (s *Store) ListInboxItems(ctx context.Context) ([]inbox.InboxItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, source_id, external_id, entity_type, title, archived_at, ignored_at, created_at, updated_at FROM inbox_items ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var items []inbox.InboxItem
	for rows.Next() {
		item, err := scanInboxItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ArchiveInboxItem(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE inbox_items SET archived_at=?, updated_at=? WHERE id=?`, now(), now(), id)
	return err
}

func (s *Store) IgnoreInboxItem(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE inbox_items SET ignored_at=?, updated_at=? WHERE id=?`, now(), now(), id)
	return err
}

func (s *Store) LatestInboxVersion(ctx context.Context, itemID int64) (inbox.InboxItemVersion, error) {
	return s.latestInboxVersionTx(ctx, nil, itemID)
}

func (s *Store) latestInboxVersionTx(ctx context.Context, tx *sql.Tx, itemID int64) (inbox.InboxItemVersion, error) {
	query := `SELECT id, inbox_item_id, version, raw_payload, normalized, payload_hash, observed_at, created_at FROM inbox_item_versions WHERE inbox_item_id=? ORDER BY version DESC LIMIT 1`
	if tx != nil {
		return scanInboxVersion(tx.QueryRowContext(ctx, query, itemID))
	}
	return scanInboxVersion(s.db.QueryRowContext(ctx, query, itemID))
}

func (s *Store) getInboxVersionTx(ctx context.Context, tx *sql.Tx, id int64) (inbox.InboxItemVersion, error) {
	return scanInboxVersion(tx.QueryRowContext(ctx, `SELECT id, inbox_item_id, version, raw_payload, normalized, payload_hash, observed_at, created_at FROM inbox_item_versions WHERE id=?`, id))
}

type rowScanner interface{ Scan(dest ...any) error }

func scanInboxItem(row rowScanner) (inbox.InboxItem, error) {
	var item inbox.InboxItem
	var archived, ignored sql.NullString
	var created, updated string
	if err := row.Scan(&item.ID, &item.SourceID, &item.ExternalID, &item.EntityType, &item.Title, &archived, &ignored, &created, &updated); err != nil {
		return item, err
	}
	item.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	item.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	if archived.Valid {
		t, _ := time.Parse(time.RFC3339Nano, archived.String)
		item.ArchivedAt = &t
	}
	if ignored.Valid {
		t, _ := time.Parse(time.RFC3339Nano, ignored.String)
		item.IgnoredAt = &t
	}
	return item, nil
}

func scanInboxVersion(row rowScanner) (inbox.InboxItemVersion, error) {
	var v inbox.InboxItemVersion
	var raw, normalized, observed, created string
	if err := row.Scan(&v.ID, &v.InboxItemID, &v.Version, &raw, &normalized, &v.PayloadHash, &observed, &created); err != nil {
		return v, err
	}
	_ = json.Unmarshal([]byte(raw), &v.RawPayload)
	_ = json.Unmarshal([]byte(normalized), &v.Normalized)
	v.ObservedAt, _ = time.Parse(time.RFC3339Nano, observed)
	v.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return v, nil
}

func (s *Store) LoadWorkflowFile(ctx context.Context, path string) (workflows.Version, bool, error) {
	wf, data, err := workflows.ParseFile(path)
	if err != nil {
		return workflows.Version{}, false, err
	}
	if err := workflows.Validate(wf); err != nil {
		return workflows.Version{}, false, err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	var out workflows.Version
	created := false
	err = withTx(ctx, s.db, func(tx *sql.Tx) error {
		ts := now()
		_, err := tx.ExecContext(ctx, `INSERT INTO workflow_definitions(workflow_id, name, enabled, created_at, updated_at)
			VALUES(?, ?, ?, ?, ?)
			ON CONFLICT(workflow_id) DO UPDATE SET name=excluded.name, enabled=excluded.enabled, updated_at=excluded.updated_at`,
			wf.ID, wf.Name, boolInt(wf.Enabled), ts, ts)
		if err != nil {
			return err
		}
		var defID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM workflow_definitions WHERE workflow_id=?`, wf.ID).Scan(&defID); err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `SELECT id, version, hash, path, yaml FROM workflow_versions WHERE workflow_definition_id=? AND hash=?`, defID, hash).Scan(&out.ID, &out.Version, &out.Hash, &out.Path, new(string))
		if err == nil {
			out.DefinitionID = defID
			out.Workflow = wf
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var latest int
		_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM workflow_versions WHERE workflow_definition_id=?`, defID).Scan(&latest)
		res, err := tx.ExecContext(ctx, `INSERT INTO workflow_versions(workflow_definition_id, version, hash, path, yaml, created_at) VALUES(?, ?, ?, ?, ?, ?)`, defID, latest+1, hash, path, string(data), ts)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		out = workflows.Version{ID: id, DefinitionID: defID, Version: latest + 1, Hash: hash, Path: path, Workflow: wf}
		created = true
		return nil
	})
	return out, created, err
}

func (s *Store) LoadWorkflowDir(ctx context.Context, dir string) ([]workflows.Version, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var versions []workflows.Version
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		version, _, err := s.LoadWorkflowFile(ctx, filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	return versions, nil
}

func (s *Store) ListWorkflowDefinitions(ctx context.Context) ([]workflows.Definition, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, workflow_id, name, enabled FROM workflow_definitions ORDER BY workflow_id`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var defs []workflows.Definition
	for rows.Next() {
		var def workflows.Definition
		var enabled int
		if err := rows.Scan(&def.ID, &def.WorkflowID, &def.Name, &enabled); err != nil {
			return nil, err
		}
		def.Enabled = enabled == 1
		defs = append(defs, def)
	}
	return defs, rows.Err()
}

func (s *Store) LatestEnabledWorkflowVersions(ctx context.Context) ([]workflows.Version, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT wv.id, wv.workflow_definition_id, wv.version, wv.hash, wv.path, wv.yaml
		FROM workflow_versions wv
		JOIN workflow_definitions wd ON wd.id = wv.workflow_definition_id
		WHERE wd.enabled = 1 AND wv.version = (SELECT MAX(version) FROM workflow_versions WHERE workflow_definition_id = wd.id)`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var versions []workflows.Version
	for rows.Next() {
		version, err := scanWorkflowVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

func (s *Store) GetWorkflowVersion(ctx context.Context, id int64) (workflows.Version, error) {
	return scanWorkflowVersion(s.db.QueryRowContext(ctx, `SELECT id, workflow_definition_id, version, hash, path, yaml FROM workflow_versions WHERE id=?`, id))
}

func scanWorkflowVersion(row rowScanner) (workflows.Version, error) {
	var v workflows.Version
	var data string
	if err := row.Scan(&v.ID, &v.DefinitionID, &v.Version, &v.Hash, &v.Path, &data); err != nil {
		return v, err
	}
	if err := yaml.Unmarshal([]byte(data), &v.Workflow); err != nil {
		return v, err
	}
	return v, nil
}

func (s *Store) RecordTriggerEvaluation(ctx context.Context, itemID, versionID, workflowID, workflowVersionID int64, matched bool, policy, reason string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO trigger_evaluations(inbox_item_id, inbox_item_version_id, workflow_definition_id, workflow_version_id, matched, policy, reason, created_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		itemID, versionID, workflowID, workflowVersionID, boolInt(matched), policy, reason, now())
	return err
}

func (s *Store) HasDispatchForItem(ctx context.Context, itemID, workflowID int64) (bool, error) {
	return exists(ctx, s.db, `SELECT 1 FROM workflow_dispatches WHERE inbox_item_id=? AND workflow_id=? LIMIT 1`, itemID, workflowID)
}

func (s *Store) HasDispatchForVersion(ctx context.Context, versionID, workflowVersionID int64) (bool, error) {
	return exists(ctx, s.db, `SELECT 1 FROM workflow_dispatches WHERE inbox_item_version_id=? AND workflow_version_id=? LIMIT 1`, versionID, workflowVersionID)
}

func (s *Store) CreateDispatch(ctx context.Context, itemID, itemVersionID, workflowID, workflowVersionID int64) (dispatch.WorkflowDispatch, error) {
	ts := now()
	res, err := s.db.ExecContext(ctx, `INSERT INTO workflow_dispatches(inbox_item_id, inbox_item_version_id, workflow_id, workflow_version_id, status, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?)`,
		itemID, itemVersionID, workflowID, workflowVersionID, dispatch.StatusQueued, ts, ts)
	if err != nil {
		return dispatch.WorkflowDispatch{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetDispatch(ctx, id)
}

func (s *Store) GetDispatch(ctx context.Context, id int64) (dispatch.WorkflowDispatch, error) {
	return scanDispatch(s.db.QueryRowContext(ctx, `SELECT id, inbox_item_id, inbox_item_version_id, workflow_id, workflow_version_id, status, created_at, updated_at FROM workflow_dispatches WHERE id=?`, id))
}

func (s *Store) ClaimQueuedDispatch(ctx context.Context) (dispatch.WorkflowDispatch, bool, error) {
	var d dispatch.WorkflowDispatch
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM workflow_dispatches WHERE status=? ORDER BY id LIMIT 1`, dispatch.StatusQueued).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE workflow_dispatches SET status=?, updated_at=? WHERE id=?`, dispatch.StatusRunning, now(), id); err != nil {
			return err
		}
		d, err = scanDispatch(tx.QueryRowContext(ctx, `SELECT id, inbox_item_id, inbox_item_version_id, workflow_id, workflow_version_id, status, created_at, updated_at FROM workflow_dispatches WHERE id=?`, id))
		return err
	})
	return d, d.ID != 0, err
}

func (s *Store) UpdateDispatchStatus(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE workflow_dispatches SET status=?, updated_at=? WHERE id=?`, status, now(), id)
	return err
}

func scanDispatch(row rowScanner) (dispatch.WorkflowDispatch, error) {
	var d dispatch.WorkflowDispatch
	var created, updated string
	if err := row.Scan(&d.ID, &d.InboxItemID, &d.InboxItemVersionID, &d.WorkflowID, &d.WorkflowVersionID, &d.Status, &created, &updated); err != nil {
		return d, err
	}
	d.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	d.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return d, nil
}

func (s *Store) CreateRun(ctx context.Context, dispatchID, workflowVersionID int64) (runs.WorkflowRun, error) {
	ts := now()
	res, err := s.db.ExecContext(ctx, `INSERT INTO workflow_runs(workflow_dispatch_id, workflow_version_id, status, created_at, updated_at) VALUES(?, ?, ?, ?, ?)`, dispatchID, workflowVersionID, runs.RunCreated, ts, ts)
	if err != nil {
		return runs.WorkflowRun{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetRun(ctx, id)
}

func (s *Store) UpdateRunStatus(ctx context.Context, id int64, status string, errMsg string) error {
	ts := now()
	if status == runs.RunRunning {
		_, err := s.db.ExecContext(ctx, `UPDATE workflow_runs SET status=?, started_at=?, updated_at=? WHERE id=?`, status, ts, ts, id)
		return err
	}
	if status == runs.RunCompleted || status == runs.RunFailed || status == runs.RunCancelled {
		_, err := s.db.ExecContext(ctx, `UPDATE workflow_runs SET status=?, finished_at=?, error=?, updated_at=? WHERE id=?`, status, ts, errMsg, ts, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE workflow_runs SET status=?, error=?, updated_at=? WHERE id=?`, status, errMsg, ts, id)
	return err
}

func (s *Store) GetRun(ctx context.Context, id int64) (runs.WorkflowRun, error) {
	return scanRun(s.db.QueryRowContext(ctx, `SELECT id, workflow_dispatch_id, workflow_version_id, status, started_at, finished_at, created_at, updated_at FROM workflow_runs WHERE id=?`, id))
}

func (s *Store) ListRuns(ctx context.Context) ([]runs.WorkflowRun, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, workflow_dispatch_id, workflow_version_id, status, started_at, finished_at, created_at, updated_at FROM workflow_runs ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var out []runs.WorkflowRun
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func scanRun(row rowScanner) (runs.WorkflowRun, error) {
	var r runs.WorkflowRun
	var started, finished sql.NullString
	var created, updated string
	if err := row.Scan(&r.ID, &r.WorkflowDispatchID, &r.WorkflowVersionID, &r.Status, &started, &finished, &created, &updated); err != nil {
		return r, err
	}
	if started.Valid {
		t, _ := time.Parse(time.RFC3339Nano, started.String)
		r.StartedAt = &t
	}
	if finished.Valid {
		t, _ := time.Parse(time.RFC3339Nano, finished.String)
		r.FinishedAt = &t
	}
	r.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	r.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return r, nil
}

func (s *Store) CreateStepRun(ctx context.Context, runID int64, stepID, stepType, status string, input, output map[string]any, errMsg string) (int64, error) {
	in, _ := json.Marshal(input)
	out, _ := json.Marshal(output)
	ts := now()
	res, err := s.db.ExecContext(ctx, `INSERT INTO step_runs(workflow_run_id, step_id, step_type, status, input_json, output_json, error, started_at, finished_at, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, stepID, stepType, status, string(in), string(out), errMsg, ts, ts, ts, ts)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) AddArtifact(ctx context.Context, inboxItemID, runID, stepRunID int64, typ, path string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO artifacts(inbox_item_id, workflow_run_id, step_run_id, type, path, created_at) VALUES(NULLIF(?,0), NULLIF(?,0), NULLIF(?,0), ?, ?, ?)`,
		inboxItemID, runID, stepRunID, typ, path, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) AddSinkOutput(ctx context.Context, runID int64, typ, path string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sink_outputs(workflow_run_id, type, path, created_at) VALUES(?, ?, ?, ?)`, runID, typ, path, now())
	return err
}

func exists(ctx context.Context, db *sql.DB, query string, args ...any) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx, query, args...).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func FormatID(prefix string, id int64) string {
	return fmt.Sprintf("%s_%d", prefix, id)
}
