package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kodelyx/flow-go/flow-go/internal/bridge"
	"github.com/kodelyx/flow-go/flow-go/internal/config"
	"github.com/kodelyx/flow-go/flow-go/internal/cookiejar"
	"github.com/kodelyx/flow-go/flow-go/internal/engine"
	"github.com/kodelyx/flow-go/flow-go/internal/pool"
	"github.com/kodelyx/flow-go/flow-go/internal/store"
)

// AccountManager owns one warm engine per account file and routes jobs to those
// engines. Each engine is built from an explicit bundle and has a private bridge
// view with no browser clients, so mutable session state cannot cross accounts.
type AccountManager struct {
	store             *store.Store
	bridge            *bridge.Bridge
	workers           int
	strictFresh       bool
	mu                sync.RWMutex
	backgroundMu      sync.Mutex
	backgroundWG      sync.WaitGroup
	backgroundStop    bool
	accounts          map[string]*accountRuntime
	accountGates      map[string]chan struct{}
	accountRevisions  map[string]uint64
	runtimeFactory    func(context.Context, string, string, *cookiejar.Bundle, bool) (*accountRuntime, *pool.Worker, error)
	scheduler         *pool.Pool
	scheduleWorkers   map[string]*pool.Worker
	reloadLocks       map[string]*sync.Mutex
	updates           chan bridge.CookieUpdate
	global            chan struct{}
	mediaPollGate     chan struct{}
	mediaDownloadGate chan struct{}
	cursor            string
	done              chan struct{}
	doneOnce          sync.Once
	initialized       chan struct{}
	initOnce          sync.Once
}

var errAccountSnapshotSuperseded = errors.New("service: account snapshot superseded by a newer cookie update")

type accountRuntime struct {
	id                string
	path              string
	engine            *engine.Engine
	gate              chan struct{}
	lastUsed          time.Time
	snapshot          string
	schedulerWorker   *pool.Worker
}

type generationPayload struct {
	Prompt      string   `json:"prompt"`
	Aspect      string   `json:"aspect"`
	Model       string   `json:"model"`
	Duration    string   `json:"duration"`
	Quality     string   `json:"quality"`
	Count       int      `json:"count"`
	FilePath    string   `json:"file_path"`
	ForceUpload bool     `json:"force"`
	NoDownload  bool     `json:"no_download"`
	StartImage  string   `json:"start_image"`
	EndImage    string   `json:"end_image"`
	References  []string `json:"references,omitempty"`
	AccountID   string   `json:"account_id"`
	AllAccounts bool     `json:"all_accounts"`
	Cookies     string   `json:"cookies"`
	ProjectID   string   `json:"project_id"`
}

func NewAccountManager(st *store.Store, br *bridge.Bridge, workers int) (*AccountManager, error) {
	if workers < 1 {
		workers = 1
	}
	pollLimit := workers * 2
	if pollLimit > 256 {
		pollLimit = 256
	}
	return NewAccountManagerWithMediaLimits(st, br, workers, pollLimit, workers)
}

// NewAccountManagerWithMediaLimits shares poll and download limits across all
// per-account engines without coupling them to the account submission slots.
func NewAccountManagerWithMediaLimits(st *store.Store, br *bridge.Bridge, workers, pollLimit, downloadLimit int) (*AccountManager, error) {
	if st == nil || br == nil {
		return nil, fmt.Errorf("service: account manager requires a store and browser bridge")
	}
	if workers < 1 {
		workers = 1
	}
	if pollLimit < 1 || pollLimit > 256 || downloadLimit < 1 || downloadLimit > 256 {
		return nil, fmt.Errorf("service: media poll and download limits must be between 1 and 256")
	}
	manager := &AccountManager{
		store: st, bridge: br, workers: workers,
		strictFresh: !strings.EqualFold(strings.TrimSpace(os.Getenv("FLOW_COOKIE_FRESHNESS")), "cached"),
		accounts:    make(map[string]*accountRuntime), accountGates: make(map[string]chan struct{}), accountRevisions: make(map[string]uint64),
		scheduler: pool.New(), scheduleWorkers: make(map[string]*pool.Worker),
		reloadLocks: make(map[string]*sync.Mutex), updates: make(chan bridge.CookieUpdate, 128),
		global: make(chan struct{}, workers), mediaPollGate: make(chan struct{}, pollLimit),
		mediaDownloadGate: make(chan struct{}, downloadLimit), done: make(chan struct{}), initialized: make(chan struct{}),
	}
	manager.runtimeFactory = manager.buildAccountRuntime
	return manager, nil
}

// Notify queues an account bundle change from the browser bridge.
func (m *AccountManager) Notify(update bridge.CookieUpdate) {
	// Invalidate synchronously so no new job can use the old snapshot. Do not
	// wait for reloadLock here: reload holds it while bootstrapping (up to 90s),
	// and blocking the bridge callback would stall cookie delivery itself.
	m.mu.Lock()
	m.accountRevisions[update.AccountID]++
	delete(m.accounts, update.AccountID)
	m.mu.Unlock()
	m.scheduler.Remove(update.AccountID)
	select {
	case m.updates <- update:
	case <-m.done:
	}
}

// Start loads persisted accounts and applies live bridge updates until shutdown.
func (m *AccountManager) Start(ctx context.Context) {
	defer m.doneOnce.Do(func() { close(m.done) })
	bootstrapLimit := m.workers
	if bootstrapLimit > 4 {
		bootstrapLimit = 4
	}
	bootstrapSlots := make(chan struct{}, bootstrapLimit)
	if paths, err := engine.DiscoverAccountJars(config.CookieDir()); err == nil {
		var wg sync.WaitGroup
		for _, path := range paths {
			acquired := false
			select {
			case bootstrapSlots <- struct{}{}:
				acquired = true
			case <-ctx.Done():
			}
			if !acquired {
				break
			}
			if ctx.Err() != nil {
				<-bootstrapSlots
				break
			}
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				defer func() { <-bootstrapSlots }()
				if err := m.reload(ctx, path); err != nil {
					// A profile without a complete bundle is not a schedulable account.
					log.Printf("service: account bootstrap skipped for %s: %v", path, err)
				}
			}(path)
		}
		wg.Wait()
	}
	m.initOnce.Do(func() { close(m.initialized) })
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-m.updates:
			if update.Path != "" {
				select {
				case bootstrapSlots <- struct{}{}:
				case <-ctx.Done():
					return
				}
				go func(update bridge.CookieUpdate) {
					defer func() { <-bootstrapSlots }()
					if err := m.reload(ctx, update.Path); err != nil {
						log.Printf("service: account reload failed for %s: %v", update.AccountID, err)
					}
				}(update)
			}
		}
	}
}

func (m *AccountManager) reload(ctx context.Context, path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "account_") || !strings.HasSuffix(base, ".json") {
		return fmt.Errorf("service: invalid account bundle path %q", path)
	}
	id := strings.TrimSuffix(strings.TrimPrefix(base, "account_"), ".json")
	lock := m.reloadLock(id)
	lock.Lock()
	defer lock.Unlock()
	previousStatus, _ := m.store.AccountStatus(id)
	m.mu.RLock()
	revision := m.accountRevisions[id]
	m.mu.RUnlock()

	// Remove it from selection while the new snapshot is bootstrapping. Existing
	// jobs retain their runtime pointer and finish under the snapshot they began.
	m.scheduler.Remove(id)
	m.mu.Lock()
	delete(m.accounts, id)
	m.mu.Unlock()

	bundle, err := cookiejar.LoadBundleFile(path)
	if err != nil {
		return err
	}
	if bundle.AccountID != "" && bundle.AccountID != id {
		return fmt.Errorf("service: account bundle identity %q does not match file identity %q", bundle.AccountID, id)
	}
	if !bundle.Complete() {
		return fmt.Errorf("service: account %s has an incomplete session bundle", id)
	}
	runtime, worker, err := m.runtimeFactory(ctx, id, path, bundle, previousStatus == "needs_login")
	if err != nil {
		return err
	}
	if runtime == nil || worker == nil || runtime.id != id || worker.ID != id {
		return fmt.Errorf("service: runtime factory returned an invalid account binding for %q", id)
	}
	if !m.publishRuntimeIfCurrent(id, revision, runtime, worker) {
		return errAccountSnapshotSuperseded
	}
	return nil
}

func (m *AccountManager) buildAccountRuntime(ctx context.Context, id, path string, bundle *cookiejar.Bundle, clearAuthCooldown bool) (*accountRuntime, *pool.Worker, error) {
	bootstrapStarted := time.Now()
	defer func() {
		log.Printf("service_metric phase=account_bootstrap account=%q elapsed_ms=%d", id, time.Since(bootstrapStarted).Milliseconds())
	}()
	localBridge := m.bridge.AccountView(id)
	eng, err := engine.New(m.store, localBridge, engine.Options{
		AccountID: id, CookieFile: path, DefaultProjectID: config.ProjectID, RefreshCreditsOnBoot: true,
		MediaPollGate: m.mediaPollGate, MediaDownloadGate: m.mediaDownloadGate,
		SchedulerOwnsFailover: true,
	})
	if err != nil {
		return nil, nil, err
	}
	bootCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if err := eng.Bootstrap(bootCtx); err != nil {
		if reason := eng.SessionFailure(); reason != "" {
			m.markNeedsLogin(id, reason, nil)
		}
		return nil, nil, fmt.Errorf("service: bootstrap account %s: %w", id, err)
	}
	if reason := eng.SessionFailure(); reason != "" {
		m.markNeedsLogin(id, reason, nil)
		return nil, nil, fmt.Errorf("service: account %s needs a fresh login: %s", id, reason)
	}
	if !eng.Ready() {
		return nil, nil, fmt.Errorf("service: account %s did not become ready", id)
	}
	if previousID, reconciled, err := m.store.ReconcileAccountIdentity(ctx, id); err != nil {
		log.Printf("service: account identity history reconciliation skipped for %s: %v", id, err)
	} else if reconciled {
		log.Printf("service: reconciled historical account identity %s into current runtime %s", previousID, id)
	}
	snapshot, _ := json.Marshal(bundle)
	digest := fmt.Sprintf("%x", sha256.Sum256(snapshot))
	runtime := &accountRuntime{
		id: id, path: path, engine: eng, gate: make(chan struct{}, 1), snapshot: digest,
	}
	worker := m.prepareSchedulerWorker(id, eng, clearAuthCooldown)
	runtime.schedulerWorker = worker
	return runtime, worker, nil
}

func (m *AccountManager) prepareSchedulerWorker(id string, eng *engine.Engine, clearAuthCooldown bool) *pool.Worker {
	var observed *pool.Worker
	for _, candidate := range eng.Pool().Workers() {
		if candidate.ID == id {
			observed = candidate
			break
		}
	}
	m.mu.Lock()
	worker := m.scheduleWorkers[id]
	if worker == nil {
		worker = pool.NewWorker(id, nil)
		worker.Concurrency = 1
		m.scheduleWorkers[id] = worker
	}
	if observed != nil {
		if credits, known := observed.Credits(); known {
			worker.SetCredits(credits, observed.SKU())
		}
	}
	worker.SetProjectID(eng.ProjectID())
	m.mu.Unlock()
	if clearAuthCooldown {
		worker.ClearCooldown()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := m.store.ClearAccountCooldown(ctx, id); err != nil {
			log.Printf("service: clear cooldown after account %s recovered: %v", id, err)
		}
		cancel()
	}
	m.restoreAccountCooldown(worker, id)
	m.scheduler.NotifyAvailabilityChanged()
	return worker
}

// publishRuntimeIfCurrent commits the runtime and its scheduler worker
// atomically with respect to Notify's account revision update. A bootstrap for
// an older cookie bundle cannot publish after a newer rotation event.
func (m *AccountManager) publishRuntimeIfCurrent(id string, revision uint64, runtime *accountRuntime, worker *pool.Worker) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.accountRevisions[id] != revision {
		return false
	}
	gate := m.accountGates[id]
	if gate == nil {
		gate = runtime.gate
		if gate == nil {
			gate = make(chan struct{}, 1)
		}
		m.accountGates[id] = gate
	}
	runtime.gate = gate
	m.accounts[id] = runtime
	m.scheduler.Register(worker)
	return true
}

func (m *AccountManager) restoreAccountCooldown(worker *pool.Worker, accountID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	until, reason, exists, err := m.store.AccountCooldown(ctx, accountID)
	if err != nil {
		log.Printf("service: load scheduler cooldown for account %s: %v", accountID, err)
		return
	}
	if !exists {
		return
	}
	if reason == "insufficient_credits" {
		if credits, known := worker.Credits(); known && credits > 0 {
			worker.ClearCooldown()
			if err := m.store.ClearAccountCooldown(ctx, accountID); err != nil {
				log.Printf("service: clear replenished credit cooldown for account %s: %v", accountID, err)
			}
			return
		}
	}
	if until.After(time.Now()) {
		worker.RestoreCooldown(until)
		return
	}
	if err := m.store.ClearAccountCooldown(ctx, accountID); err != nil {
		log.Printf("service: clear expired scheduler cooldown for account %s: %v", accountID, err)
	}
}

func (m *AccountManager) syncSchedulerBalance(runtime *accountRuntime) {
	if runtime == nil || runtime.engine == nil {
		return
	}
	var observed *pool.Worker
	for _, candidate := range runtime.engine.Pool().Workers() {
		if candidate.ID == runtime.id {
			observed = candidate
			break
		}
	}
	if observed == nil {
		return
	}
	credits, known := observed.Credits()
	if !known {
		return
	}
	m.mu.RLock()
	scheduled := m.scheduleWorkers[runtime.id]
	m.mu.RUnlock()
	if scheduled == nil {
		return
	}
	scheduled.SetCredits(credits, observed.SKU())
	m.scheduler.NotifyAvailabilityChanged()
	if credits <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, reason, exists, err := m.store.AccountCooldown(ctx, runtime.id)
	if err != nil {
		log.Printf("service: read cooldown after refreshing account %s balance: %v", runtime.id, err)
		return
	}
	if exists && reason == "insufficient_credits" {
		scheduled.ClearCooldown()
		m.scheduler.NotifyAvailabilityChanged()
		if err := m.store.ClearAccountCooldown(ctx, runtime.id); err != nil {
			log.Printf("service: clear credit cooldown for account %s: %v", runtime.id, err)
		}
	}
}

func (m *AccountManager) reloadLock(id string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	if lock := m.reloadLocks[id]; lock != nil {
		return lock
	}
	lock := &sync.Mutex{}
	m.reloadLocks[id] = lock
	return lock
}

func (m *AccountManager) account(id string) *accountRuntime {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.accounts[id]
}

func (m *AccountManager) accountIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.accounts))
	for id := range m.accounts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Initialized reports whether the initial persisted-account scan is complete.
func (m *AccountManager) Initialized() bool {
	select {
	case <-m.initialized:
		return true
	default:
		return false
	}
}

// ReadyAccounts reports accounts that can accept a new job under the active
// freshness policy. Busy accounts remain ready; open circuits and disconnected
// accounts in strict-fresh mode do not.
func (m *AccountManager) ReadyAccounts() int {
	ready := 0
	for _, id := range m.accountIDs() {
		runtime := m.account(id)
		if runtime == nil || runtime.schedulerWorker == nil || runtime.schedulerWorker.State() == pool.StateCircuitOpen {
			continue
		}
		if m.strictFresh && !m.bridge.AccountConnected(id) {
			continue
		}
		ready++
	}
	return ready
}

// RequiresLiveCookieSync reports whether strict freshness requires an online
// extension connection. Jobs use the persisted account bundle and do not pull a
// cookie snapshot immediately before submission.
func (m *AccountManager) RequiresLiveCookieSync() bool { return m.strictFresh }

// ConnectedAccounts reports runtimes with a currently attached extension peer.
func (m *AccountManager) ConnectedAccounts() int {
	connected := 0
	for _, id := range m.accountIDs() {
		if m.bridge.AccountConnected(id) {
			connected++
		}
	}
	return connected
}

// CookieSyncAges reports durable snapshot age per registered account, in
// seconds. Missing bundles are omitted so callers can distinguish unknown from
// a recently synced session.
func (m *AccountManager) CookieSyncAges() map[string]float64 {
	ages := make(map[string]float64)
	for _, id := range m.accountIDs() {
		if age, ok := m.bridge.CookieSyncAge(id); ok {
			ages[id] = age.Seconds()
		}
	}
	return ages
}

func (m *AccountManager) Balance(ctx context.Context, refresh bool) (map[string]any, error) {
	if refresh {
		var wg sync.WaitGroup
		for _, id := range m.accountIDs() {
			runtime, err := m.acquire(ctx, id)
			if err != nil {
				return nil, err
			}
			wg.Add(1)
			go func(runtime *accountRuntime) {
				defer wg.Done()
				defer m.release(runtime)
				runtime.engine.RefreshCredits(ctx)
				m.syncSchedulerBalance(runtime)
			}(runtime)
		}
		wg.Wait()
	}
	accounts, err := m.store.ListAccounts()
	if err != nil {
		return nil, err
	}
	runtimeIDs := make(map[string]struct{})
	for _, id := range m.accountIDs() {
		if m.account(id) != nil {
			runtimeIDs[id] = struct{}{}
		}
	}
	total, unread := 0, 0
	current := make([]store.Account, 0, len(runtimeIDs))
	for _, account := range accounts {
		if _, ok := runtimeIDs[account.AccountID]; !ok {
			continue
		}
		current = append(current, account)
		if account.Credits == nil {
			unread++
		} else {
			total += *account.Credits
		}
	}
	return map[string]any{"accounts": current, "total_credits": total, "unread": unread}, nil
}

func (m *AccountManager) ListProjects(ctx context.Context, accountID string) (any, error) {
	runtime, err := m.acquireScheduledRuntime(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer m.release(runtime)
	return runtime.engine.ListProjects(ctx)
}

func (m *AccountManager) acquire(ctx context.Context, id string) (*accountRuntime, error) {
	if id != "" {
		runtime := m.account(id)
		if runtime == nil {
			select {
			case <-m.initialized:
				runtime = m.account(id)
				if runtime == nil {
					return nil, fmt.Errorf("service: account %s is not ready", id)
				}
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if m.strictFresh && !m.bridge.AccountConnected(id) {
			return nil, fmt.Errorf("service: account %s extension is offline; strict cookie freshness requires a connected extension", id)
		}
		select {
		case runtime.gate <- struct{}{}:
			return runtime, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		m.mu.Lock()
		var best *accountRuntime
		eligible := 0
		for _, candidate := range m.accounts {
			if m.strictFresh && !m.bridge.AccountConnected(candidate.id) {
				continue
			}
			eligible++
			if len(candidate.gate) == cap(candidate.gate) {
				continue
			}
			if best == nil || len(candidate.gate) < len(best.gate) ||
				(len(candidate.gate) == len(best.gate) && candidate.lastUsed.Before(best.lastUsed)) {
				best = candidate
			}
		}
		if eligible == 0 {
			m.mu.Unlock()
			select {
			case <-m.initialized:
				return nil, fmt.Errorf("service: no account has a connected extension under strict cookie freshness")
			default:
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-m.initialized:
					continue
				case <-ticker.C:
					continue
				}
			}
		}
		if best != nil {
			select {
			case best.gate <- struct{}{}:
				best.lastUsed = time.Now()
				m.cursor = best.id
				m.mu.Unlock()
				return best, nil
			default:
			}
		}
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *AccountManager) release(runtime *accountRuntime) { <-runtime.gate }

// Run implements the job Runner used by the durable queue.
func (m *AccountManager) Run(ctx context.Context, job store.Job) (any, error) {
	return m.RunWithProgress(ctx, job, nil)
}

// ValidateJobPayload performs local validation before a request is persisted
// into the shared queue. Account availability and upstream checks remain in the
// scheduler/runner, where they can be reported as execution outcomes.
func (m *AccountManager) ValidateJobPayload(ctx context.Context, job store.Job) error {
	var payload generationPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("service: invalid %s payload: %w", job.Kind, err)
	}
	switch job.Kind {
	case "image":
		if strings.TrimSpace(payload.Prompt) == "" {
			return fmt.Errorf("service: image prompt is required")
		}
		if payload.Count < 0 || payload.Count > 4 {
			return fmt.Errorf("service: image count must be between 1 and 4")
		}
		if len(payload.References) > 0 {
			return fmt.Errorf("service: image jobs do not accept video reference images")
		}
		if aspect := strings.TrimSpace(payload.Aspect); aspect != "" {
			if _, ok := config.ImageAspectValue(aspect); !ok {
				return fmt.Errorf("service: invalid image aspect %q", aspect)
			}
		}
	case "video":
		if strings.TrimSpace(payload.Prompt) == "" {
			return fmt.Errorf("service: video prompt is required")
		}
		if payload.Count < 0 || payload.Count > config.MaxCount {
			return fmt.Errorf("service: video count must be between 1 and %d", config.MaxCount)
		}
		if len(payload.References) > 0 && (payload.StartImage != "" || payload.EndImage != "") {
			return fmt.Errorf("service: reference images cannot be combined with start_image or end_image")
		}
		duration := 8
		if raw := strings.TrimSpace(payload.Duration); raw != "" {
			value, err := strconv.Atoi(strings.TrimSuffix(raw, "s"))
			if err != nil {
				return fmt.Errorf("service: invalid video duration %q", raw)
			}
			duration = value
		}
		if len(payload.References) > 0 {
			if _, ok := config.ReferenceVideoModelFor(duration); !ok {
				return fmt.Errorf("service: unsupported reference video duration %ds", duration)
			}
		} else if !slices.Contains(config.Durations, duration) {
			return fmt.Errorf("service: unsupported video duration %ds", duration)
		}
		if len(payload.References) == 0 {
			if aspect := strings.TrimSpace(payload.Aspect); aspect != "" {
				if _, ok := config.VideoAspectValue(aspect); !ok {
					return fmt.Errorf("service: invalid video aspect %q", aspect)
				}
			}
		}
		if payload.AllAccounts && (len(payload.References) > 0 || payload.StartImage != "" || payload.EndImage != "") {
			return fmt.Errorf("service: reference media cannot be fanned out across accounts; pin account_id")
		}
		if payload.AccountID == "" && (len(payload.References) > 0 || payload.StartImage != "" || payload.EndImage != "") {
			if err := m.bindReferenceAccount(ctx, &payload); err != nil {
				return err
			}
		}
	case "upload-video":
		path := strings.TrimSpace(payload.FilePath)
		if path == "" {
			return fmt.Errorf("service: file_path is required for upload-video")
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("service: upload file is not accessible: %w", err)
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("service: upload file must be a non-empty regular file")
		}
		if limit := int64(config.MaxUploadMB) * 1024 * 1024; info.Size() > limit {
			return fmt.Errorf("service: upload file exceeds the %d MB limit", config.MaxUploadMB)
		}
	default:
		return fmt.Errorf("service: unsupported job kind %q", job.Kind)
	}
	return nil
}

func (m *AccountManager) RunWithProgress(ctx context.Context, job store.Job, progress AttemptReporter) (any, error) {
	var payload generationPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return nil, fmt.Errorf("service: decode %s payload: %w", job.Kind, err)
	}
	if payload.AllAccounts {
		return nil, fmt.Errorf("service: all_accounts request must be expanded into durable child jobs")
	}
	if err := m.bindReferenceAccount(ctx, &payload); err != nil {
		return nil, err
	}
	accountID := payload.AccountID
	if accountID == "" && payload.Cookies != "" {
		var err error
		accountID, err = accountIDFromCookiePath(payload.Cookies)
		if err != nil {
			return nil, err
		}
	}
	return m.runOne(ctx, job.ID, job.Kind, payload, accountID, progress)
}

// bindReferenceAccount pins known generated media to its owner. An opaque
// upstream ID has no locally verifiable account, so callers must pin it rather
// than let load balancing choose a potentially unrelated project.
func (m *AccountManager) bindReferenceAccount(ctx context.Context, payload *generationPayload) error {
	if payload.AccountID != "" {
		return nil
	}
	refs := append([]string(nil), payload.References...)
	refs = append(refs, payload.StartImage, payload.EndImage)
	ownerAccount := ""
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if info, err := os.Stat(ref); err == nil && info.Mode().IsRegular() {
			continue // local bytes can be uploaded under the selected account
		}
		owner, found, err := m.store.LookupMediaOwnership(ctx, ref)
		if err != nil {
			return fmt.Errorf("service: check reference media ownership: %w", err)
		}
		if !found || owner.AccountID == "" {
			return fmt.Errorf("service: external reference %q requires an explicit account_id", ref)
		}
		if ownerAccount != "" && ownerAccount != owner.AccountID {
			return fmt.Errorf("service: reference media belongs to multiple accounts; pin and split the job")
		}
		ownerAccount = owner.AccountID
	}
	payload.AccountID = ownerAccount
	return nil
}

// ExpandForQueue snapshots eligible accounts for all_accounts and pins image
// count children to one account before queue workers execute them.
func (m *AccountManager) ExpandForQueue(_ context.Context, job store.Job) ([]store.Job, bool, error) {
	var payload generationPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return nil, false, fmt.Errorf("service: decode %s payload for expansion: %w", job.Kind, err)
	}
	count := payload.Count
	if count < 1 {
		count = 1
	}
	all := payload.AllAccounts
	imageFanout := job.Kind == "image" && count > 1
	if !all && !imageFanout {
		return nil, false, nil
	}
	if imageFanout && count > 4 {
		return nil, true, fmt.Errorf("service: image count must be between 1 and 4")
	}

	ids := m.accountIDs()
	if payload.AccountID == "" && payload.Cookies != "" {
		id, err := accountIDFromCookiePath(payload.Cookies)
		if err != nil {
			return nil, true, err
		}
		payload.AccountID = id
	}
	if !all {
		if payload.AccountID != "" {
			ids = []string{payload.AccountID}
		} else if len(ids) > 0 {
			// The count is pinned as a single batch. Choose the least occupied
			// connected runtime once, then let the durable queue serialize its children.
			best := ""
			bestLoad := int(^uint(0) >> 1)
			for _, id := range ids {
				candidate := m.account(id)
				if candidate == nil || (m.strictFresh && !m.bridge.AccountConnected(id)) {
					continue
				}
				load := len(candidate.gate)
				if load < bestLoad {
					best, bestLoad = id, load
				}
			}
			ids = nil
			if best != "" {
				ids = []string{best}
			}
		}
	}
	if m.strictFresh {
		filtered := ids[:0]
		for _, id := range ids {
			if m.bridge.AccountConnected(id) {
				filtered = append(filtered, id)
			}
		}
		ids = filtered
	}
	if len(ids) == 0 {
		return nil, true, fmt.Errorf("service: no eligible account is ready for parent job")
	}
	for _, id := range ids {
		if m.account(id) == nil {
			return nil, true, fmt.Errorf("service: account %s is not ready for parent job", id)
		}
	}

	children := make([]store.Job, 0)
	for _, id := range ids {
		n := 1
		if imageFanout {
			n = count
		}
		for i := 0; i < n; i++ {
			childPayload := payload
			childPayload.AccountID = id
			childPayload.Cookies = ""
			childPayload.AllAccounts = false
			if imageFanout {
				childPayload.Count = 1
			}
			encoded, err := json.Marshal(childPayload)
			if err != nil {
				return nil, true, err
			}
			children = append(children, store.Job{ID: uuid.NewString(), Kind: job.Kind, Payload: string(encoded)})
		}
	}
	return children, true, nil
}

// ResumeWithProgress resumes work whose upstream media ids were persisted before
// the worker lease expired. This path only resolves/polls/downloads those ids;
// it never calls a generation submission method.
func (m *AccountManager) ResumeWithProgress(ctx context.Context, job store.Job, attempt store.JobAttempt, progress AttemptReporter) (any, error) {
	if attempt.AccountID == "" || attempt.ProjectID == "" || len(attempt.UpstreamIDs) == 0 || attempt.AccountID == "all_accounts" {
		return nil, fmt.Errorf("service: attempt lacks a resumable account, project, or upstream media ids")
	}
	var payload generationPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return nil, fmt.Errorf("service: decode %s payload for resume: %w", job.Kind, err)
	}
	if payload.NoDownload {
		return map[string]any{"job_id": job.ID, "account_id": attempt.AccountID, "project_id": attempt.ProjectID, "media_ids": attempt.UpstreamIDs, "status": "submitted"}, nil
	}
	if job.Kind != "image" && job.Kind != "video" {
		return nil, fmt.Errorf("service: job kind %q cannot resume accepted media", job.Kind)
	}
	runtime, err := m.acquire(ctx, attempt.AccountID)
	if err != nil {
		return nil, err
	}
	defer m.release(runtime)
	if progress != nil {
		ctx = engine.WithExecutionProgress(ctx, progress)
	}
	result, err := runtime.engine.ResumeAcceptedMedia(ctx, attempt.ProjectID, attempt.UpstreamIDs, job.Kind, payload.Prompt, true)
	if err != nil {
		if reason := runtime.engine.SessionFailure(); reason != "" {
			m.markNeedsLogin(runtime.id, reason, runtime)
		}
		return nil, err
	}
	result["job_id"] = job.ID
	result["account_id"] = runtime.id
	return result, nil
}

func (m *AccountManager) runOne(ctx context.Context, jobID, kind string, payload generationPayload, accountID string, progress AttemptReporter) (result any, runErr error) {
	if strings.TrimSpace(payload.Prompt) == "" && kind != "upload-video" {
		return nil, fmt.Errorf("service: prompt is required")
	}
	cost := generationCreditCost(kind, payload)
	var scheduled *pool.Worker
	var err error
	if accountID != "" {
		scheduled, err = m.acquireScheduled(ctx, accountID, cost)
	} else {
		scheduled, err = m.acquireScheduled(ctx, "", cost)
	}
	if err != nil {
		return nil, err
	}
	engineStarted := false
	var runtime *accountRuntime
	defer func() {
		if outcome, ok := result.(*engine.BatchVideoOutcome); ok && outcome.CreditsKnown {
			scheduled.SetCredits(outcome.Credits, "")
		}
		// Errors from queue cancellation, cookie refresh, or local setup do not
		// indicate that Flow is unhealthy. Only charge account failure state once
		// an engine operation has actually begun, and do not penalize cancellation.
		releaseErr := runErr
		if !engineStarted || errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			releaseErr = nil
		}
		m.scheduler.Release(scheduled, 0, releaseErr)
		sessionFailure := ""
		if runtime != nil && runtime.engine != nil {
			sessionFailure = runtime.engine.SessionFailure()
		}
		if sessionFailure != "" {
			m.markNeedsLogin(scheduled.ID, sessionFailure, runtime)
			return
		}
		cooldownUntil := scheduled.CooldownUntil()
		persistCtx, cancelPersist := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelPersist()
		if cooldownUntil.After(time.Now()) {
			if err := m.store.SetAccountCooldown(persistCtx, scheduled.ID, cooldownUntil, schedulerCooldownReason(releaseErr)); err != nil {
				log.Printf("service: persist scheduler cooldown for account %s: %v", scheduled.ID, err)
			}
		} else if releaseErr == nil {
			if err := m.store.ClearAccountCooldown(persistCtx, scheduled.ID); err != nil {
				log.Printf("service: clear scheduler cooldown for account %s: %v", scheduled.ID, err)
			}
		}
	}()
	accountID = scheduled.ID
	runtime, err = m.acquire(ctx, accountID)
	if err != nil {
		return nil, err
	}
	// The account runtime is built from its persisted bundle. Cookie rotation
	// events update that bundle asynchronously; generation does not pull cookies
	// from the extension immediately before submitting a job.
	defer m.release(runtime)
	if err := m.validateReferenceOwnership(ctx, runtime.engine, payload); err != nil {
		return nil, err
	}
	select {
	case m.global <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	var submissionGateOnce sync.Once
	releaseSubmissionGate := func() {
		submissionGateOnce.Do(func() { <-m.global })
	}
	defer releaseSubmissionGate()
	if progress != nil {
		if err := progress.BindAccount(runtime.id, runtime.engine.ProjectID(), runtime.snapshot); err != nil {
			return nil, err
		}
		// Image balance refresh and image/video engine preflights happen before
		// reporting "submitting" immediately before the generation RPC. Keep the
		// durable phase at preparing until that boundary so a process restart can
		// safely reclaim a lease lost during preflight. Upload-video has no
		// equivalent phase boundary yet, so it stays conservative.
		if kind == "upload-video" {
			if err := progress.SetPhase("executing", ""); err != nil {
				return nil, err
			}
		}
		ctx = engine.WithExecutionProgress(ctx, &submissionGateReporter{
			next: progress, release: releaseSubmissionGate,
		})
	}
	engineStarted = true
	switch kind {
	case "upload-video":
		if strings.TrimSpace(payload.FilePath) == "" {
			return nil, fmt.Errorf("service: file_path is required")
		}
		return runtime.engine.UploadVideo(ctx, payload.FilePath, engine.UploadOptions{Force: payload.ForceUpload, ProjectID: payload.ProjectID})
	case "image":
		return m.generateImages(ctx, runtime, jobID, payload)
	case "video":
		if len(payload.References) > 0 {
			duration := 8
			if raw := strings.TrimSpace(payload.Duration); raw != "" {
				value, parseErr := strconv.Atoi(strings.TrimSuffix(raw, "s"))
				if parseErr != nil {
					return nil, fmt.Errorf("service: invalid duration %q", raw)
				}
				duration = value
			}
			return runtime.engine.GenerateVideoFromReferencesViaBatch(ctx, engine.BatchReferenceRequest{
				Prompt: payload.Prompt, References: payload.References, ProjectID: payload.ProjectID,
				Duration: duration, Wait: !payload.NoDownload, Download: !payload.NoDownload,
			})
		}
		duration := 8
		if raw := strings.TrimSpace(payload.Duration); raw != "" {
			value, parseErr := strconv.Atoi(strings.TrimSuffix(raw, "s"))
			if parseErr != nil {
				return nil, fmt.Errorf("service: invalid duration %q", raw)
			}
			duration = value
		}
		count := payload.Count
		if count < 1 {
			count = 1
		}
		return runtime.engine.GenerateVideoViaBatch(ctx, engine.BatchVideoRequest{
			Prompt: payload.Prompt, Aspect: payload.Aspect, ProjectID: payload.ProjectID, Duration: duration,
			Quality: payload.Quality, Count: count, StartImage: payload.StartImage,
			Wait: !payload.NoDownload, Download: !payload.NoDownload,
		})
	default:
		return nil, fmt.Errorf("service: unsupported job kind %q", kind)
	}
}

// validateReferenceOwnership prevents a locally known Flow media ID from being
// sent with another account's cookies or project. Unknown IDs remain usable for
// externally created assets whose ownership is not present in this database.
func (m *AccountManager) validateReferenceOwnership(ctx context.Context, e *engine.Engine, payload generationPayload) error {
	refs := append([]string(nil), payload.References...)
	refs = append(refs, payload.StartImage, payload.EndImage)
	for _, id := range refs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		owner, found, err := m.store.LookupMediaOwnership(ctx, id)
		if err != nil {
			return fmt.Errorf("service: check reference media ownership: %w", err)
		}
		if !found {
			continue
		}
		if owner.AccountID != "" && owner.AccountID != e.AccountID() {
			return fmt.Errorf("service: reference media %q belongs to account %q, selected account is %q", id, owner.AccountID, e.AccountID())
		}
		selectedProject := strings.TrimSpace(payload.ProjectID)
		if selectedProject == "" {
			selectedProject = e.ProjectID()
		}
		if owner.ProjectID != "" && selectedProject != "" && owner.ProjectID != selectedProject {
			return fmt.Errorf("service: reference media %q belongs to project %q, selected project is %q", id, owner.ProjectID, selectedProject)
		}
	}
	return nil
}

// refreshImageBalance reads the authoritative balance immediately before an
// image submission and publishes it to the scheduler and account store. A
// failed balance read is best-effort: it must not prevent image generation.
func (m *AccountManager) refreshImageBalance(ctx context.Context, runtime *accountRuntime) {
	if runtime == nil || runtime.engine == nil {
		return
	}
	creditCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	credits, err := runtime.engine.Credits(creditCtx)
	if err != nil {
		log.Printf("service: refresh image balance for account %s: %v", runtime.id, err)
		return
	}
	m.publishObservedBalance(runtime, credits)
}

func (m *AccountManager) startBackground(task func()) bool {
	if task == nil {
		return false
	}
	m.backgroundMu.Lock()
	if m.backgroundStop {
		m.backgroundMu.Unlock()
		return false
	}
	m.backgroundWG.Add(1)
	m.backgroundMu.Unlock()
	go func() {
		defer m.backgroundWG.Done()
		task()
	}()
	return true
}

// WaitBackground prevents balance refreshes from racing database shutdown.
// Call it after the durable job workers have stopped, so no new refresh can be
// launched while the wait group is being drained.
func (m *AccountManager) WaitBackground() {
	m.backgroundMu.Lock()
	m.backgroundStop = true
	m.backgroundMu.Unlock()
	m.backgroundWG.Wait()
}

func (m *AccountManager) publishObservedBalance(runtime *accountRuntime, credits int) bool {
	if runtime == nil {
		return false
	}
	m.mu.RLock()
	current := m.accounts[runtime.id] == runtime
	worker := m.scheduleWorkers[runtime.id]
	m.mu.RUnlock()
	if !current || worker == nil {
		return false
	}
	worker.SetCredits(credits, "")
	m.scheduler.NotifyAvailabilityChanged()
	checkedAt := time.Now().UTC()
	value := credits
	if err := m.store.UpsertAccount(store.Account{
		AccountID: runtime.id, Credits: &value, CreditsCheckedAt: &checkedAt, Status: "active",
	}); err != nil {
		log.Printf("service: persist refreshed image balance for account %s: %v", runtime.id, err)
	}
	if credits > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, reason, exists, err := m.store.AccountCooldown(ctx, runtime.id)
		if err != nil {
			log.Printf("service: read cooldown after image balance refresh for account %s: %v", runtime.id, err)
		} else if exists && reason == "insufficient_credits" {
			worker.ClearCooldown()
			m.scheduler.NotifyAvailabilityChanged()
			if err := m.store.ClearAccountCooldown(ctx, runtime.id); err != nil {
				log.Printf("service: clear credit cooldown after image balance refresh for account %s: %v", runtime.id, err)
			}
		}
	}
	return true
}

func schedulerCooldownReason(err error) string {
	var insufficient interface{ InsufficientCredits() bool }
	if errors.As(err, &insufficient) && insufficient.InsufficientCredits() {
		return "insufficient_credits"
	}
	var throttled interface{ Throttled() bool }
	if errors.As(err, &throttled) && throttled.Throttled() {
		return "throttled"
	}
	return "circuit_breaker"
}

func (m *AccountManager) markNeedsLogin(accountID, reason string, expected *accountRuntime) {
	if strings.TrimSpace(accountID) == "" {
		return
	}
	m.mu.Lock()
	if expected != nil && m.accounts[accountID] != expected {
		m.mu.Unlock()
		return
	}
	delete(m.accounts, accountID)
	m.mu.Unlock()
	m.scheduler.Remove(accountID)
	if err := m.store.UpsertAccount(store.Account{
		AccountID: accountID, Status: "needs_login", LastError: reason,
	}); err != nil {
		log.Printf("service: mark account %s needs_login: %v", accountID, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := m.store.ClearAccountCooldown(ctx, accountID); err != nil {
		log.Printf("service: clear cooldown for account %s requiring login: %v", accountID, err)
	}
	cancel()
}

func (m *AccountManager) acquireScheduled(ctx context.Context, accountID string, cost int) (*pool.Worker, error) {
	// A partially bootstrapped registry may already contain other accounts. A
	// pinned request for an account still being loaded must wait for the initial
	// scan to finish instead of being rejected as an unknown account. Automatic
	// selection can use any account that is already ready.
	if accountID != "" && m.account(accountID) == nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-m.initialized:
		}
	}
	if accountID == "" && m.scheduler.Size() == 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-m.initialized:
		}
	}
	if accountID != "" {
		return m.scheduler.AcquireAccount(ctx, accountID, cost)
	}
	return m.scheduler.Acquire(ctx, cost)
}

// acquireScheduledRuntime reserves the account in the scheduler before taking
// its runtime gate. The pool slot is released once the runtime gate is held;
// callers release the runtime with m.release.
func (m *AccountManager) acquireScheduledRuntime(ctx context.Context, accountID string) (*accountRuntime, error) {
	worker, err := m.acquireScheduled(ctx, accountID, 0)
	if err != nil {
		return nil, err
	}
	runtime, err := m.acquire(ctx, worker.ID)
	m.scheduler.Release(worker, 0, err)
	return runtime, err
}

func generationCreditCost(kind string, payload generationPayload) int {
	if kind != "video" {
		return 0
	}
	if len(payload.References) > 0 {
		return config.CreditsPerReferenceVideo
	}
	duration := 8
	if raw := strings.TrimSpace(payload.Duration); raw != "" {
		if value, err := strconv.Atoi(strings.TrimSuffix(raw, "s")); err == nil {
			duration = value
		} else {
			return 0
		}
	}
	count := payload.Count
	if count < 1 {
		count = 1
	}
	cost, known := engine.EstimateVideoCredits(duration, count, payload.Quality)
	if !known {
		return 0
	}
	return cost
}

// generateImages expands the public image count into sequential submissions on
// the same pinned runtime. The account gate remains held for the whole request,
// so another request cannot interleave mutable session work on that account.
func (m *AccountManager) generateImages(ctx context.Context, runtime *accountRuntime, parentJobID string, payload generationPayload) (any, error) {
	count := payload.Count
	if count < 1 {
		count = 1
	}
	if count > 4 {
		return nil, fmt.Errorf("service: image count must be between 1 and 4")
	}
	if count == 1 {
		m.refreshImageBalance(ctx, runtime)
		return runtime.engine.GenerateImageViaBatch(ctx, engine.BatchImageRequest{
			Prompt: payload.Prompt, Model: payload.Model, ProjectID: payload.ProjectID, Aspect: payload.Aspect, Download: !payload.NoDownload,
		})
	}
	combined := map[string]any{
		"job_id": parentJobID, "account_id": runtime.id, "media": []any{}, "files": []any{},
		"child_job_ids": []string{}, "status": "succeeded", "elapsed_seconds": float64(0),
	}
	var failures []string
	succeeded := 0
	for i := 0; i < count; i++ {
		m.refreshImageBalance(ctx, runtime)
		outcome, err := runtime.engine.GenerateImageViaBatch(ctx, engine.BatchImageRequest{
			Prompt: payload.Prompt, Model: payload.Model, ProjectID: payload.ProjectID, Aspect: payload.Aspect, Download: !payload.NoDownload,
		})
		if err != nil {
			failures = append(failures, err.Error())
			if ctx.Err() != nil {
				break
			}
			continue
		}
		encoded, _ := json.Marshal(outcome)
		var item map[string]any
		if err := json.Unmarshal(encoded, &item); err != nil {
			failures = append(failures, err.Error())
			continue
		}
		appendJSONItems(combined, "media", item["media"])
		appendJSONItems(combined, "files", item["files"])
		appendJSONItems(combined, "child_job_ids", []any{item["job_id"]})
		if elapsed, ok := item["elapsed_seconds"].(float64); ok {
			combined["elapsed_seconds"] = combined["elapsed_seconds"].(float64) + elapsed
		}
		if item["status"] == "succeeded" || item["status"] == "ready" {
			succeeded++
		} else {
			failures = append(failures, fmt.Sprintf("image attempt %d returned status %v", i+1, item["status"]))
		}
	}
	if succeeded == 0 {
		if len(failures) == 0 {
			return nil, fmt.Errorf("service: image generation produced no result")
		}
		return nil, fmt.Errorf("service: all %d image submissions failed: %s", count, strings.Join(failures, "; "))
	}
	if len(failures) > 0 || succeeded < count {
		combined["status"] = "partial"
		combined["errors"] = failures
	}
	return combined, nil
}

func appendJSONItems(target map[string]any, key string, value any) {
	items, _ := target[key].([]any)
	more, _ := value.([]any)
	target[key] = append(items, more...)
}

func accountIDFromCookiePath(path string) (string, error) {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "account_") || !strings.HasSuffix(base, ".json") {
		return "", fmt.Errorf("service: cookies must identify an account_<key>.json bundle")
	}
	return strings.TrimSuffix(strings.TrimPrefix(base, "account_"), ".json"), nil
}
