package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrAmbiguousAccountIdentity = errors.New("store: multiple historical accounts share the current session fingerprint")

// ReconcileAccountIdentity moves historical ownership to an explicit account
// ID only when the live row and exactly one legacy row share the SAPISID
// fingerprint. Cookie hashes are deliberately not used here because rotating
// cookies change the full-jar hash while the session fingerprint stays stable.
func (s *Store) ReconcileAccountIdentity(ctx context.Context, canonicalID string) (string, bool, error) {
	canonicalID = strings.TrimSpace(canonicalID)
	if canonicalID == "" {
		return "", false, errors.New("store: canonical account ID is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("store: begin account identity reconciliation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var targetSource, fingerprint string
	var targetAuthuser sql.NullInt64
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(identity_source,''), COALESCE(sapisid_fingerprint,''), last_authuser
		  FROM accounts WHERE account_id=?`, canonicalID,
	).Scan(&targetSource, &fingerprint, &targetAuthuser); err != nil {
		return "", false, fmt.Errorf("store: read canonical account identity: %w", err)
	}
	if targetSource != "override" || fingerprint == "" || !targetAuthuser.Valid {
		if err := tx.Commit(); err != nil {
			return "", false, fmt.Errorf("store: finish empty account reconciliation: %w", err)
		}
		return "", false, nil
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT account_id FROM accounts
		 WHERE account_id<>? AND status='active' AND COALESCE(superseded_by,'')=''
		   AND COALESCE(sapisid_fingerprint,'')=?
		   AND last_authuser=?
		   AND identity_source IN ('cookie-core','session','re-anchored','adopted-legacy','legacy-jar-hash')
		 ORDER BY created_at, account_id`, canonicalID, fingerprint, targetAuthuser.Int64)
	if err != nil {
		return "", false, fmt.Errorf("store: find historical identity rows: %w", err)
	}
	var candidates []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return "", false, fmt.Errorf("store: read historical identity row: %w", err)
		}
		candidates = append(candidates, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return "", false, fmt.Errorf("store: iterate historical identity rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return "", false, fmt.Errorf("store: close historical identity rows: %w", err)
	}
	if len(candidates) > 1 {
		return "", false, ErrAmbiguousAccountIdentity
	}
	if len(candidates) == 0 {
		if err := tx.Commit(); err != nil {
			return "", false, fmt.Errorf("store: finish empty account reconciliation: %w", err)
		}
		return "", false, nil
	}
	legacyID := candidates[0]

	if err := rewritePinnedJobs(ctx, tx, legacyID, canonicalID); err != nil {
		return "", false, err
	}
	for _, table := range []string{"generations", "request_logs", "uploads", "job_attempts"} {
		if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET account_id=? WHERE account_id=?`, canonicalID, legacyID); err != nil {
			return "", false, fmt.Errorf("store: transfer %s account ownership: %w", table, err)
		}
	}
	if err := moveAccountCooldown(ctx, tx, legacyID, canonicalID); err != nil {
		return "", false, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO account_anchors(account_id,identity_key,cookie_hash,source,first_seen,last_seen)
		SELECT ?,identity_key,cookie_hash,source,first_seen,last_seen
		  FROM account_anchors WHERE account_id=?
		ON CONFLICT(account_id,identity_key) DO UPDATE SET
			cookie_hash=COALESCE(NULLIF(excluded.cookie_hash,''),account_anchors.cookie_hash),
			source=COALESCE(NULLIF(excluded.source,''),account_anchors.source),
			first_seen=MIN(account_anchors.first_seen,excluded.first_seen),
			last_seen=MAX(account_anchors.last_seen,excluded.last_seen)
	`, canonicalID, legacyID); err != nil {
		return "", false, fmt.Errorf("store: transfer identity anchors: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM account_anchors WHERE account_id=?`, legacyID); err != nil {
		return "", false, fmt.Errorf("store: clear legacy identity anchors: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE accounts SET
			total_requests=(SELECT total_requests FROM accounts WHERE account_id=?)+total_requests,
			total_failures=(SELECT total_failures FROM accounts WHERE account_id=?)+total_failures,
			created_at=MIN(created_at,(SELECT created_at FROM accounts WHERE account_id=?)),
			last_used_at=CASE
				WHEN last_used_at IS NULL THEN (SELECT last_used_at FROM accounts WHERE account_id=?)
				WHEN (SELECT last_used_at FROM accounts WHERE account_id=?) IS NULL THEN last_used_at
				ELSE MAX(last_used_at,(SELECT last_used_at FROM accounts WHERE account_id=?)) END
		 WHERE account_id=?`, legacyID, legacyID, legacyID, legacyID, legacyID, legacyID, canonicalID); err != nil {
		return "", false, fmt.Errorf("store: transfer historical account totals: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE accounts SET status='parked', superseded_by=?, credits=NULL,
			credits_checked_at=NULL, total_requests=0, total_failures=0,
			last_error='identity history moved to canonical account'
		 WHERE account_id=?`, canonicalID, legacyID); err != nil {
		return "", false, fmt.Errorf("store: mark historical identity row superseded: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", false, fmt.Errorf("store: commit account identity reconciliation: %w", err)
	}
	return legacyID, true, nil
}

func rewritePinnedJobs(ctx context.Context, tx *sql.Tx, legacyID, canonicalID string) error {
	rows, err := tx.QueryContext(ctx, `SELECT job_id,payload FROM jobs WHERE instr(payload,?)>0`, legacyID)
	if err != nil {
		return fmt.Errorf("store: find jobs pinned to historical account: %w", err)
	}
	type rewrite struct{ id, payload string }
	var updates []rewrite
	for rows.Next() {
		var id, payload string
		if err := rows.Scan(&id, &payload); err != nil {
			_ = rows.Close()
			return fmt.Errorf("store: read pinned job payload: %w", err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(payload), &fields); err != nil {
			_ = rows.Close()
			return fmt.Errorf("store: decode pinned job payload %q: %w", id, err)
		}
		var accountID string
		if raw := fields["account_id"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &accountID); err != nil {
				_ = rows.Close()
				return fmt.Errorf("store: decode pinned account on job %q: %w", id, err)
			}
		}
		if accountID != legacyID {
			continue
		}
		encodedID, _ := json.Marshal(canonicalID)
		fields["account_id"] = encodedID
		encoded, err := json.Marshal(fields)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("store: encode reassigned job %q: %w", id, err)
		}
		updates = append(updates, rewrite{id: id, payload: string(encoded)})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("store: iterate pinned jobs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("store: close pinned jobs: %w", err)
	}
	for _, update := range updates {
		if _, err := tx.ExecContext(ctx, `UPDATE jobs SET payload=?,updated_at=CURRENT_TIMESTAMP WHERE job_id=?`, update.payload, update.id); err != nil {
			return fmt.Errorf("store: reassign pinned job %q: %w", update.id, err)
		}
	}
	return nil
}

func moveAccountCooldown(ctx context.Context, tx *sql.Tx, legacyID, canonicalID string) error {
	var until, reason, updated string
	err := tx.QueryRowContext(ctx, `SELECT until_at,reason,updated_at FROM account_cooldowns WHERE account_id=?`, legacyID).Scan(&until, &reason, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: read historical account cooldown: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO account_cooldowns(account_id,until_at,reason,updated_at) VALUES(?,?,?,?)
		ON CONFLICT(account_id) DO UPDATE SET
			until_at=CASE WHEN excluded.until_at>account_cooldowns.until_at THEN excluded.until_at ELSE account_cooldowns.until_at END,
			reason=CASE WHEN excluded.until_at>account_cooldowns.until_at THEN excluded.reason ELSE account_cooldowns.reason END,
			updated_at=MAX(excluded.updated_at,account_cooldowns.updated_at)
	`, canonicalID, until, reason, updated); err != nil {
		return fmt.Errorf("store: merge account cooldown: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM account_cooldowns WHERE account_id=?`, legacyID); err != nil {
		return fmt.Errorf("store: clear historical account cooldown: %w", err)
	}
	return nil
}
