package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func authuser(index int) *int { return &index }

func TestReconcileAccountIdentityTransfersOwnershipAndRetiresLegacyRow(t *testing.T) {
	st := openTemp(t)
	legacyCredits, liveCredits := 800, 1200
	for _, account := range []Account{
		{AccountID: "legacy-core", CookieHash: "old-jar", Credits: &legacyCredits, Status: "active", IdentityKey: "core:old", IdentitySource: "cookie-core", SapisidFingerprint: "same-session", LastAuthuser: authuser(0), TotalRequests: 7, TotalFailures: 2},
		{AccountID: "profile-current", CookieHash: "rotated-jar", Credits: &liveCredits, Status: "active", IdentityKey: "profile-current", IdentitySource: "override", SapisidFingerprint: "same-session", LastAuthuser: authuser(0), TotalRequests: 1},
	} {
		if err := st.UpsertAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.db.Exec(`UPDATE accounts SET total_requests=7,total_failures=2 WHERE account_id='legacy-core'`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE accounts SET total_requests=1 WHERE account_id='profile-current'`); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordAnchor("legacy-core", "core:old", "old-jar", "cookie-core"); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordAnchor("profile-current", "profile-current", "rotated-jar", "override"); err != nil {
		t.Fatal(err)
	}
	generationID, err := st.RecordGeneration(Generation{JobID: "generation-legacy", AccountID: "legacy-core", Kind: "image", Status: "succeeded"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`INSERT INTO media(generation_id,media_id,kind) VALUES(?,?,?)`, generationID, "known-media", "image"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`INSERT INTO request_logs(account_id,endpoint,status_code) VALUES(?,?,?)`, "legacy-core", "/generate", 200); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`INSERT INTO uploads(file_path,file_hash,media_id,account_id,project_id,bytes) VALUES(?,?,?,?,?,?)`, "clip.mp4", "file-hash", "uploaded-media", "legacy-core", "project-a", 12); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`INSERT INTO jobs(job_id,kind,payload,status) VALUES(?,?,?,'queued')`, "pinned-job", "image", `{"prompt":"keep","account_id":"legacy-core"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`INSERT INTO job_attempts(job_id,attempt_no,account_id,phase) VALUES(?,1,?,'preparing')`, "pinned-job", "legacy-core"); err != nil {
		t.Fatal(err)
	}
	legacyCooldown := time.Now().UTC().Add(10 * time.Minute).Round(0)
	currentCooldown := time.Now().UTC().Add(2 * time.Minute).Round(0)
	if err := st.SetAccountCooldown(context.Background(), "legacy-core", legacyCooldown, "old-throttle"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetAccountCooldown(context.Background(), "profile-current", currentCooldown, "current-throttle"); err != nil {
		t.Fatal(err)
	}

	legacyID, reconciled, err := st.ReconcileAccountIdentity(context.Background(), "profile-current")
	if err != nil || !reconciled || legacyID != "legacy-core" {
		t.Fatalf("reconcile=(%q,%v,%v), want legacy-core,true,nil", legacyID, reconciled, err)
	}
	if _, reconciled, err := st.ReconcileAccountIdentity(context.Background(), "profile-current"); err != nil || reconciled {
		t.Fatalf("second reconcile=(%v,%v), want idempotent no-op", reconciled, err)
	}

	accounts, err := st.ListAccounts()
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]Account, len(accounts))
	for _, account := range accounts {
		byID[account.AccountID] = account
	}
	legacy := byID["legacy-core"]
	if legacy.SupersededBy != "profile-current" || legacy.Status != "parked" || legacy.Credits != nil || legacy.TotalRequests != 0 || legacy.TotalFailures != 0 {
		t.Fatalf("legacy row after reconcile=%+v", legacy)
	}
	current := byID["profile-current"]
	if current.Credits == nil || *current.Credits != liveCredits || current.TotalRequests != 8 || current.TotalFailures != 2 {
		t.Fatalf("canonical row after reconcile=%+v", current)
	}
	var generationOwner, requestOwner, uploadOwner, attemptOwner string
	var mediaCount, anchorCount, legacyAnchorCount int
	if err := st.db.QueryRow(`SELECT account_id FROM generations WHERE job_id='generation-legacy'`).Scan(&generationOwner); err != nil {
		t.Fatal(err)
	}
	if err := st.db.QueryRow(`SELECT count(*) FROM media WHERE media_id='known-media'`).Scan(&mediaCount); err != nil {
		t.Fatal(err)
	}
	if err := st.db.QueryRow(`SELECT account_id FROM request_logs WHERE endpoint='/generate'`).Scan(&requestOwner); err != nil {
		t.Fatal(err)
	}
	if err := st.db.QueryRow(`SELECT account_id FROM uploads WHERE media_id='uploaded-media'`).Scan(&uploadOwner); err != nil {
		t.Fatal(err)
	}
	if err := st.db.QueryRow(`SELECT account_id FROM job_attempts WHERE job_id='pinned-job'`).Scan(&attemptOwner); err != nil {
		t.Fatal(err)
	}
	if err := st.db.QueryRow(`SELECT count(*) FROM account_anchors WHERE account_id='profile-current'`).Scan(&anchorCount); err != nil {
		t.Fatal(err)
	}
	if err := st.db.QueryRow(`SELECT count(*) FROM account_anchors WHERE account_id='legacy-core'`).Scan(&legacyAnchorCount); err != nil {
		t.Fatal(err)
	}
	if generationOwner != "profile-current" || mediaCount != 1 || requestOwner != "profile-current" || uploadOwner != "profile-current" || attemptOwner != "profile-current" || anchorCount != 2 || legacyAnchorCount != 0 {
		t.Fatalf("ownership after reconcile: generation=%s media=%d request=%s upload=%s attempt=%s anchors=%d legacyAnchors=%d", generationOwner, mediaCount, requestOwner, uploadOwner, attemptOwner, anchorCount, legacyAnchorCount)
	}
	var payload string
	if err := st.db.QueryRow(`SELECT payload FROM jobs WHERE job_id='pinned-job'`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var jobPayload map[string]any
	if err := json.Unmarshal([]byte(payload), &jobPayload); err != nil {
		t.Fatal(err)
	}
	if jobPayload["account_id"] != "profile-current" || jobPayload["prompt"] != "keep" {
		t.Fatalf("rewritten pinned job payload=%s", payload)
	}
	deadline, reason, exists, err := st.AccountCooldown(context.Background(), "profile-current")
	if err != nil || !exists || reason != "old-throttle" || !deadline.Equal(legacyCooldown) {
		t.Fatalf("merged cooldown=(%v,%q,%v,%v), want longest cooldown", deadline, reason, exists, err)
	}
	if _, _, exists, err := st.AccountCooldown(context.Background(), "legacy-core"); err != nil || exists {
		t.Fatalf("legacy cooldown remains (exists=%v,err=%v)", exists, err)
	}
}

func TestReconcileAccountIdentityRefusesAmbiguousHistoricalRows(t *testing.T) {
	st := openTemp(t)
	for _, account := range []Account{
		{AccountID: "legacy-a", Status: "active", IdentitySource: "cookie-core", SapisidFingerprint: "same-session", LastAuthuser: authuser(0)},
		{AccountID: "legacy-b", Status: "active", IdentitySource: "session", SapisidFingerprint: "same-session", LastAuthuser: authuser(0)},
		{AccountID: "current", Status: "active", IdentitySource: "override", SapisidFingerprint: "same-session", LastAuthuser: authuser(0)},
	} {
		if err := st.UpsertAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.RecordGeneration(Generation{JobID: "ambiguous-history", AccountID: "legacy-a", Kind: "image"}); err != nil {
		t.Fatal(err)
	}
	if _, reconciled, err := st.ReconcileAccountIdentity(context.Background(), "current"); !errors.Is(err, ErrAmbiguousAccountIdentity) || reconciled {
		t.Fatalf("ambiguous reconcile=(%v,%v), want sentinel error and no merge", reconciled, err)
	}
	var owner string
	if err := st.db.QueryRow(`SELECT account_id FROM generations WHERE job_id='ambiguous-history'`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if owner != "legacy-a" {
		t.Fatalf("ambiguous history moved to %q", owner)
	}
}

func TestReconcileAccountIdentityRequiresMatchingAuthuser(t *testing.T) {
	st := openTemp(t)
	for _, account := range []Account{
		{AccountID: "legacy", Status: "active", IdentitySource: "cookie-core", SapisidFingerprint: "same-session", LastAuthuser: authuser(1)},
		{AccountID: "current", Status: "active", IdentitySource: "override", SapisidFingerprint: "same-session", LastAuthuser: authuser(0)},
	} {
		if err := st.UpsertAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	if _, reconciled, err := st.ReconcileAccountIdentity(context.Background(), "current"); err != nil || reconciled {
		t.Fatalf("authuser-mismatch reconcile=(%v,%v), want no-op", reconciled, err)
	}
	accounts, err := st.ListAccounts()
	if err != nil || len(accounts) != 2 {
		t.Fatalf("accounts after authuser mismatch=%d, err=%v", len(accounts), err)
	}
}

func TestReconcileAccountIdentityRequiresSessionProof(t *testing.T) {
	st := openTemp(t)
	for _, account := range []Account{
		{AccountID: "legacy", Status: "active", IdentitySource: "cookie-core", SapisidFingerprint: "", LastAuthuser: authuser(0)},
		{AccountID: "current", Status: "active", IdentitySource: "override", SapisidFingerprint: "", LastAuthuser: authuser(0)},
	} {
		if err := st.UpsertAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	if _, reconciled, err := st.ReconcileAccountIdentity(context.Background(), "current"); err != nil || reconciled {
		t.Fatalf("missing-proof reconcile=(%v,%v), want no-op", reconciled, err)
	}
	accounts, err := st.ListAccounts()
	if err != nil || len(accounts) != 2 {
		t.Fatalf("accounts after unproven reconcile=%d, err=%v", len(accounts), err)
	}
}
