// Package app assembles the pieces into a running system.
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kodelyx/flow-go/flow-go/internal/bridge"
	"github.com/kodelyx/flow-go/flow-go/internal/config"
	"github.com/kodelyx/flow-go/flow-go/internal/cookiejar"
	"github.com/kodelyx/flow-go/flow-go/internal/engine"
	"github.com/kodelyx/flow-go/flow-go/internal/flowapi"
	"github.com/kodelyx/flow-go/flow-go/internal/service"
	"github.com/kodelyx/flow-go/flow-go/internal/store"
)

// DefaultTargets is what the bridge is allowed to attach to, and the first entry
// is the URL it opens when it has no tab.
//
// Both hosts are listed because Labs redirects: `labs.google/fx/tools/flow` is
// the canonical entry point, but it lands on `flow.google.com`. An allowlist
// holding only the first entry means the extension opens a tab, watches it
// redirect out of scope, and then refuses to attach — which is exactly how this
// was found.
var DefaultTargets = []string{
	"https://labs.google/fx/tools/flow",
	"https://flow.google.com",
}

// DefaultCookieDomains is the cookie scope. It is deliberately the minimum that
// produces a working Labs session: the Labs host, the Google identity cookies it
// authenticates against, and the account chooser.
//
// `accounts.google.com` is there for one cookie, `ACCOUNT_CHOOSER`, which is the
// only place the browser records which Google accounts are signed in. Without it
// the engine can address `authuser=0` and nothing else, and every other signed-in
// account is invisible — which reads as "one account" rather than "one readable".
var DefaultCookieDomains = []string{
	"flow.google.com",
	"labs.google",
	"google.com",
	"accounts.google.com",
}

// Config configures an App.
type Config struct {
	ProjectID     string
	ProxyURL      string
	CaptchaMode   string
	Targets       []string
	CookieDomains []string
	DBPath        string
	// CookieFile names one cookie file to run as, instead of the usual search.
	// Empty means the engine looks for a live browser and then a persisted copy.
	CookieFile string
	// Jar is cookies the caller already holds in memory, which the engine uses
	// rather than putting them on disk. See engine.Options.Jar.
	Jar *cookiejar.Jar
	// AtToken and Fsid are the page tokens to use when no browser is attached.
	// A process that has taken a SessionSnapshot from one that has a browser
	// supplies them; a live page always takes precedence.
	AtToken string
	Fsid    string
	// Fingerprint is the browser identity to present when no browser is attached,
	// taken from the same snapshot. A captcha-bearing call is checked against the
	// client its token was minted for, so a process without this presents a
	// generic profile and has its token silently discarded.
	Fingerprint *flowapi.BrowserFingerprint
	// RefreshCreditsOnBoot reads every registered account's balance as the boot
	// finishes. Only the daemon sets it: the figures feed the pool's
	// affordability gate, and a CLI generation run neither routes through the
	// pool nor needs anyone else's balance. See engine.Options.
	RefreshCreditsOnBoot bool
}

// App holds the assembled system.
type App struct {
	Store       *store.Store
	Bridge      *bridge.Bridge
	Engine      *engine.Engine
	captchaMode string
}

// Build opens the database, creates the bridge, and constructs the engine. It
// does not start any listener and does not require cookies yet, so a CLI
// command that only reads statistics works with no browser present.
func Build(cfg Config) (*App, error) {
	if err := config.EnsureDirs(); err != nil {
		return nil, err
	}
	captchaMode := strings.TrimSpace(cfg.CaptchaMode)
	if captchaMode == "" {
		captchaMode = "auto"
	}

	dbPath := cfg.DBPath
	if dbPath == "" {
		dbPath = config.DBPath()
	}

	st, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}

	targets := cfg.Targets
	if len(targets) == 0 {
		targets = DefaultTargets
	}
	domains := cfg.CookieDomains
	if len(domains) == 0 {
		domains = DefaultCookieDomains
	}

	// The bridge now owns where it keeps its files and where it listens; it used
	// to reach into this package's config for both, which is what kept it from
	// being a module of its own.
	br := bridge.NewBridge(targets, domains, config.CookieDir())
	bridgeListenAddr := strings.TrimSpace(os.Getenv("FLOW_BRIDGE_LISTEN_ADDR"))
	if bridgeListenAddr == "" {
		bridgeListenAddr = fmt.Sprintf("127.0.0.1:%d", config.WSPort)
	}
	br.ListenAddr = bridgeListenAddr

	eng, err := engine.New(st, br, engine.Options{
		ProjectID:    cfg.ProjectID,
		ProxyURL:     cfg.ProxyURL,
		CaptchaMode:  captchaMode,
		AccountIndex: config.AccountIndex,
		AtToken:      cfg.AtToken,
		Fsid:         cfg.Fsid,
		Fingerprint:  cfg.Fingerprint,
		CookieFile:   cfg.CookieFile,
		Jar:          cfg.Jar,
		// The env value is a default, not an instruction, so it ranks below the
		// browser. An explicit cfg.ProjectID still outranks both.
		DefaultProjectID:     config.ProjectID,
		RefreshCreditsOnBoot: cfg.RefreshCreditsOnBoot,
	})
	if err != nil {
		_ = st.Close()
		return nil, err
	}

	return &App{Store: st, Bridge: br, Engine: eng, captchaMode: captchaMode}, nil
}

// Close releases resources.
func (a *App) Close() error {
	if a.Store != nil {
		return a.Store.Close()
	}
	return nil
}

// clearPairingMarker removes what the bridge-token era left behind.
//
// The bridge is tokenless: it accepts any extension that connects, and there is
// no pairing step for a human to perform. An install upgrading from the token
// build still has `bridge-token` and `bridge-token.claimed` in the cookie
// directory, and neither is read any more — `Bridge.EnsureToken` is a no-op kept
// for compatibility. Deleting them at start-up is a migration, not a security
// step.
//
// It runs before the listener opens so that a stale marker is not the first thing
// a reader finds while debugging a connection.
func clearPairingMarker() {
	_ = os.Remove(filepath.Join(config.CookieDir(), "bridge-token.claimed"))
	_ = os.Remove(filepath.Join(config.CookieDir(), "bridge-token"))
}

// RunBridge starts the extension bridge and keeps it up until ctx is cancelled.
//
// **This is the whole of the process.** The WebSocket listener is its only
// surface: a connected Chrome profile is asked for its cookies, its page tokens
// and its browser identity, and the bridge writes them into
// `cookies/account_<key>.json` on the way in — `Bridge.SyncCookies` runs on every
// connection, so persistence does not depend on anything below. Every generation
// happens in a separate CLI run that reads one of those files, which is why there
// is no HTTP server here and nothing generates over the network.
//
// The engine is still bootstrapped once a browser shows up, and that is not a
// generation path: it is what registers the connected accounts and reads their
// credit balances, so `flow-go stats` has something current to report. Without it
// the accounts table would only move when an operator ran `flow-go credits`.
//
// One consequence of dropping the HTTP server worth knowing: `POST
// /api/sync-cookies` used to be the way to re-run that registration after a new
// profile connected. There is no replacement trigger, so a profile that attaches
// after this bootstrap writes its bundle (fine — that is the file every
// generation reads) but is not added to the accounts table until the next
// `flow-go credits` or a restart.
func (a *App) RunBridge(ctx context.Context) error {
	// Before the listener opens, because the pairing window is decided by
	// `EnsureToken` inside `Bridge.Listen` — clearing the marker afterwards would
	// leave the window shut for the whole run.
	clearPairingMarker()

	// The bridge runs for the whole process lifetime.
	bridgeErr := make(chan error, 1)
	go func() {
		if err := a.Bridge.Listen(ctx); err != nil {
			bridgeErr <- err
		}
	}()

	// Bootstrap once a browser shows up, in the background, so a slow or absent
	// browser never holds up the listener — which is the part that has to work.
	go func() {
		waitCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()

		if err := a.Bridge.WaitForExtension(waitCtx, 60*time.Second); err != nil {
			log.Printf("app: %v — listening anyway; a bundle is written when one connects", err)
		}

		bootCtx, bootCancel := context.WithTimeout(ctx, 60*time.Second)
		defer bootCancel()
		if err := a.Engine.Bootstrap(bootCtx); err != nil {
			log.Printf("app: engine bootstrap failed: %v", err)
			log.Printf("app: the bridge is still listening and still persisting bundles; " +
				"only the account and credit records are missing")
		}
	}()

	select {
	case err := <-bridgeErr:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		log.Println("app: shutting down")
	}
	return nil
}

// RunService starts one bridge owner, warm account runtimes, a durable job
// queue, and the authenticated internal HTTP API.
func (a *App) RunService(ctx context.Context, listenAddr, token string, workers int) error {
	serviceStart := time.Now()
	if strings.TrimSpace(listenAddr) == "" {
		listenAddr = "127.0.0.1:9223"
	}
	instanceLock, err := service.AcquireInstanceLock(a.Store.DatabasePath() + ".service.lock")
	if err != nil {
		return err
	}
	defer instanceLock.Close()
	queueLimit, err := positiveEnvInt("FLOW_MAX_OUTSTANDING_JOBS", 1000)
	if err != nil {
		return err
	}
	drainSeconds, err := serviceDrainSecondsFromEnv()
	if err != nil {
		return err
	}
	retentionDays, err := positiveEnvInt("FLOW_JOB_RETENTION_DAYS", 30)
	if err != nil {
		return err
	}
	maxAttempts, err := jobMaxAttemptsFromEnv()
	if err != nil {
		return err
	}
	pollLimit, err := mediaConcurrencyFromEnv("FLOW_MEDIA_POLL_CONCURRENCY", workers*2)
	if err != nil {
		return err
	}
	downloadLimit, err := mediaConcurrencyFromEnv("FLOW_MEDIA_DOWNLOAD_CONCURRENCY", workers)
	if err != nil {
		return err
	}
	queueWorkers, err := queueWorkersFromEnv(workers)
	if err != nil {
		return err
	}
	preSubmitAttempts, err := jobRetryBudgetFromEnv("FLOW_JOB_MAX_PRE_SUBMIT_ATTEMPTS", maxAttempts)
	if err != nil {
		return err
	}
	resumeAttempts, err := jobRetryBudgetFromEnv("FLOW_JOB_MAX_RESUME_ATTEMPTS", maxAttempts)
	if err != nil {
		return err
	}
	throttleAttempts, err := jobRetryBudgetFromEnv("FLOW_JOB_MAX_THROTTLE_ATTEMPTS", maxAttempts)
	if err != nil {
		return err
	}
	cookieFreshness := strings.ToLower(strings.TrimSpace(os.Getenv("FLOW_COOKIE_FRESHNESS")))
	if cookieFreshness != "cached" {
		cookieFreshness = "strict"
	}
	defaults := service.RuntimeSettings{
		SubmissionConcurrency: workers, QueueWorkers: queueWorkers, AccountConcurrency: 1,
		MediaPollConcurrency: pollLimit, MediaDownloadConcurrency: downloadLimit,
		MaxOutstandingJobs: queueLimit, MaxJobAttempts: maxAttempts,
		PreSubmitAttempts: preSubmitAttempts, ResumeAttempts: resumeAttempts,
		ThrottleAttempts: throttleAttempts, JobRetentionDays: retentionDays,
		CookieFreshness: cookieFreshness,
	}
	runtimeSettings, settingSources, err := service.LoadRuntimeSettings(a.Store, defaults)
	if err != nil {
		return err
	}
	if err := a.Store.SetMaxOutstandingJobs(runtimeSettings.MaxOutstandingJobs); err != nil {
		return err
	}
	if _, err := a.Store.DeleteTerminalJobsBefore(ctx, time.Now().Add(-time.Duration(runtimeSettings.JobRetentionDays)*24*time.Hour)); err != nil {
		return fmt.Errorf("app: apply configured %d-day job retention: %w", runtimeSettings.JobRetentionDays, err)
	}
	manager, err := service.NewAccountManagerWithMediaLimits(a.Store, a.Bridge, workers, pollLimit, downloadLimit, a.captchaMode)
	if err != nil {
		return err
	}
	manager.ApplyRuntimeSettings(runtimeSettings)
	log.Printf("app: service reCAPTCHA strategy=%q", a.captchaMode)
	jobs, err := service.NewJobs(a.Store, manager, token, runtimeSettings.QueueWorkers)
	if err != nil {
		return err
	}
	jobs.ApplyRuntimeSettings(runtimeSettings)
	a.Bridge.OnCookieSync = manager.Notify
	a.Bridge.OnConnectionChange = manager.NotifyConnectionChange
	clearPairingMarker()
	// Keep service workers alive after the caller requests shutdown so the
	// bounded drain can finish active jobs before stop() cancels their runtime.
	serviceCtx, stop := context.WithCancel(context.Background())
	defer stop()
	if reporter := service.NewNodeReporter(manager, workers); reporter != nil {
		go reporter.Start(serviceCtx)
	}
	var runtimeSettingsMu sync.RWMutex
	activeSettings := runtimeSettings
	activeSources := settingSources
	retentionWake := make(chan struct{}, 1)
	pruneRetention := func(ctx context.Context) {
		runtimeSettingsMu.RLock()
		days := activeSettings.JobRetentionDays
		runtimeSettingsMu.RUnlock()
		cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
		if _, err := a.Store.DeleteTerminalJobsBefore(ctx, cutoff); err != nil && ctx.Err() == nil {
			log.Printf("app: periodic job retention cleanup failed: %v", err)
		}
		if _, err := a.Store.DeleteIntakeBefore(ctx, cutoff); err != nil && ctx.Err() == nil {
			log.Printf("app: periodic intake retention cleanup failed: %v", err)
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/internal/v1/jobs", jobs)
	mux.Handle("/internal/v1/jobs/", jobs)
	mux.Handle("/internal/v1/node", service.RequireToken(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		nodeID := strings.TrimSpace(os.Getenv("FLOW_NODE_ID"))
		if nodeID == "" {
			nodeID, _ = os.Hostname()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"node_id":            nodeID,
			"ready_accounts":     manager.ReadyAccounts(),
			"connected_accounts": manager.ConnectedAccounts(),
			"accounts":           manager.ClusterAccounts(),
			"status":             "ok",
		})
	})))
	mux.Handle("/internal/v1/uploads", service.RequireToken(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseMultipartForm(128 << 20); err != nil {
			http.Error(w, "invalid multipart upload", http.StatusBadRequest)
			return
		}
		part, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "file is required", http.StatusBadRequest)
			return
		}
		defer part.Close()
		ext := filepath.Ext(header.Filename)
		if ext == "" {
			ext = ".mp4"
		}
		temp, err := os.CreateTemp(config.OutputDir(), "gateway-upload-*"+ext)
		if err != nil {
			http.Error(w, "could not create upload staging file", http.StatusInternalServerError)
			return
		}
		tempPath := temp.Name()
		if _, err = io.Copy(temp, part); err != nil {
			temp.Close()
			_ = os.Remove(tempPath)
			http.Error(w, "could not store upload", http.StatusInternalServerError)
			return
		}
		if err := temp.Close(); err != nil {
			_ = os.Remove(tempPath)
			http.Error(w, "could not close upload", http.StatusInternalServerError)
			return
		}
		payload, _ := json.Marshal(map[string]any{"kind": "upload-video", "payload": map[string]any{
			"file_path": tempPath, "account_id": r.FormValue("account_id"),
			"force": strings.EqualFold(r.FormValue("force"), "true"),
		}})
		upstream := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs", bytes.NewReader(payload))
		upstream.Header.Set("Content-Type", "application/json")
		upstream.Header.Set("Authorization", r.Header.Get("Authorization"))
		upstream.Header.Set("Idempotency-Key", r.Header.Get("Idempotency-Key"))
		jobs.ServeHTTP(w, upstream)
	})))
	mux.Handle("/internal/v1/media/", service.RequireToken(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mediaID := strings.TrimPrefix(r.URL.Path, "/internal/v1/media/")
		if mediaID == "" || strings.Contains(mediaID, "/") || strings.Contains(mediaID, "\\") {
			http.Error(w, "invalid media id", http.StatusBadRequest)
			return
		}
		media, err := a.Store.MediaByID(mediaID)
		if err != nil {
			http.Error(w, "media not found", http.StatusNotFound)
			return
		}
		path, err := filepath.Abs(media.FilePath)
		if err != nil {
			http.Error(w, "invalid media path", http.StatusInternalServerError)
			return
		}
		root, err := filepath.Abs(config.OutputDir())
		if err != nil {
			http.Error(w, "invalid output directory", http.StatusInternalServerError)
			return
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			http.Error(w, "media path is outside output directory", http.StatusForbidden)
			return
		}
		file, err := os.Open(path)
		if err != nil {
			http.Error(w, "media file unavailable", http.StatusNotFound)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || info.IsDir() {
			http.Error(w, "media file unavailable", http.StatusNotFound)
			return
		}
		contentType := "application/octet-stream"
		if strings.HasSuffix(strings.ToLower(media.FileName), ".mp4") {
			contentType = "video/mp4"
		} else if strings.HasSuffix(strings.ToLower(media.FileName), ".jpg") || strings.HasSuffix(strings.ToLower(media.FileName), ".jpeg") {
			contentType = "image/jpeg"
		}
		w.Header().Set("Content-Type", contentType)
		http.ServeContent(w, r, media.FileName, info.ModTime(), file)
	})))
	mux.Handle("/internal/v1/settings", service.RequireToken(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSettings := func() {
			runtimeSettingsMu.RLock()
			settingsSnapshot, sourcesSnapshot := activeSettings, make(map[string]string, len(activeSources))
			for key, value := range activeSources {
				sourcesSnapshot[key] = value
			}
			runtimeSettingsMu.RUnlock()
			active := manager.RuntimeLimitStats()
			queueTarget, queueLive := jobs.WorkerCounts()
			active["queue_workers_target"] = queueTarget
			active["queue_workers_live"] = queueLive
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"values": settingsSnapshot, "defaults": defaults, "sources": sourcesSnapshot, "active": active,
			})
		}
		switch r.Method {
		case http.MethodGet:
			writeSettings()
		case http.MethodPatch:
			var request struct {
				Values map[string]json.RawMessage `json:"values"`
				Reset  []string                   `json:"reset"`
			}
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&request); err != nil {
				http.Error(w, "invalid settings request: "+err.Error(), http.StatusBadRequest)
				return
			}
			if len(request.Values) == 0 && len(request.Reset) == 0 {
				http.Error(w, "provide values or reset keys", http.StatusBadRequest)
				return
			}
			runtimeSettingsMu.Lock()
			next, err := service.MergeRuntimeSettings(defaults, activeSettings, request.Values, request.Reset)
			if err != nil {
				runtimeSettingsMu.Unlock()
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := service.PersistRuntimeSettings(a.Store, next, request.Values, request.Reset); err != nil {
				runtimeSettingsMu.Unlock()
				http.Error(w, "could not persist settings", http.StatusInternalServerError)
				return
			}
			_ = a.Store.SetMaxOutstandingJobs(next.MaxOutstandingJobs)
			manager.ApplyRuntimeSettings(next)
			jobs.ApplyRuntimeSettings(next)
			activeSettings = next
			for key := range request.Values {
				activeSources[key] = "admin"
			}
			for _, key := range request.Reset {
				activeSources[key] = "startup default"
			}
			runtimeSettingsMu.Unlock()
			select {
			case retentionWake <- struct{}{}:
			default:
			}
			writeSettings()
		default:
			w.Header().Set("Allow", "GET, PATCH")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	mux.Handle("/internal/v1/balance", service.RequireToken(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result, err := manager.Balance(r.Context(), r.URL.Query().Get("refresh") == "true")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})))
	mux.Handle("/internal/v1/projects", service.RequireToken(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accountID := r.URL.Query().Get("account_id")
		if r.URL.Query().Get("cookies") != "" {
			http.Error(w, "use account_id; cookie paths are not accepted by the service API", http.StatusUnprocessableEntity)
			return
		}
		result, err := manager.ListProjects(r.Context(), accountID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})))
	mux.Handle("/internal/v1/stats", service.RequireToken(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recent, err := a.Store.RecentGenerations(50)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		byStatus, byKind, err := a.Store.GenerationBreakdown()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		latency, latencyByKind, err := a.Store.PhaseLatencyBreakdown(r.Context(), 5000)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		queue, err := a.Store.JobQueueStats(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		summary, err := a.Store.Stats()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		generations := make([]map[string]any, 0, len(recent))
		for _, g := range recent {
			var elapsed any
			if g.ElapsedMS != nil {
				elapsed = float64(*g.ElapsedMS) / 1000
			}
			generations = append(generations, map[string]any{
				"job_id": g.JobID, "account_id": g.AccountID, "kind": g.Kind,
				"model": g.Model, "aspect": g.Aspect, "status": g.Status,
				"credits_spent": g.CreditsSpent, "elapsed_seconds": elapsed,
			})
		}
		var total int64
		for _, count := range byStatus {
			total += count
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"generations": generations,
			"totals": map[string]any{"generations": total, "by_status": byStatus,
				"by_kind": byKind, "media_files": summary.TotalMedia, "credits_spent": summary.CreditsSpent},
			"latency":         latency,
			"latency_by_kind": latencyByKind,
			"service": map[string]any{
				"queue": queue, "ready_accounts": manager.ReadyAccounts(),
				"connected_accounts":      manager.ConnectedAccounts(),
				"strict_cookie_freshness": manager.RequiresLiveCookieSync(),
				"cookie_sync_age_seconds": manager.CookieSyncAges(),
			},
		})
	})))
	mux.Handle("/internal/v1/monitor", service.RequireToken(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 200 {
				http.Error(w, "limit must be between 1 and 200", http.StatusBadRequest)
				return
			}
			limit = parsed
		}
		page := 0
		if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 || parsed > 1_000_000_000 {
				http.Error(w, "page must be between 0 and 1000000000", http.StatusBadRequest)
				return
			}
			page = parsed
		}
		pageSize := limit
		if raw := strings.TrimSpace(r.URL.Query().Get("page_size")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 200 {
				http.Error(w, "page_size must be between 1 and 200", http.StatusBadRequest)
				return
			}
			pageSize = parsed
		}
		jobs, jobTotal, err := a.Store.MonitorJobsPage(r.Context(), pageSize, page*pageSize,
			strings.TrimSpace(r.URL.Query().Get("status")), strings.TrimSpace(r.URL.Query().Get("kind")))
		if err != nil {
			http.Error(w, "could not read monitor jobs", http.StatusInternalServerError)
			return
		}
		intakePage := 0
		if raw := strings.TrimSpace(r.URL.Query().Get("request_page")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 || parsed > 1_000_000_000 {
				http.Error(w, "request_page must be between 0 and 1000000000", http.StatusBadRequest)
				return
			}
			intakePage = parsed
		}
		intakePageSize := 20
		if raw := strings.TrimSpace(r.URL.Query().Get("request_page_size")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 200 {
				http.Error(w, "request_page_size must be between 1 and 200", http.StatusBadRequest)
				return
			}
			intakePageSize = parsed
		}
		intake, intakeTotal, err := a.Store.IntakeRequestsPage(r.Context(), intakePageSize, intakePage*intakePageSize)
		if err != nil {
			http.Error(w, "could not read intake requests", http.StatusInternalServerError)
			return
		}
		counts := make(map[string]map[string]int64, 3)
		outcomes := make(map[string]map[string]int64, 3)
		retries := make(map[string]int64, 3)
		for name, duration := range map[string]time.Duration{
			"5m": 5 * time.Minute, "1h": time.Hour, "24h": 24 * time.Hour,
		} {
			since := time.Now().Add(-duration)
			value, err := a.Store.IntakeCountsSince(r.Context(), since)
			if err != nil {
				http.Error(w, "could not summarize intake requests", http.StatusInternalServerError)
				return
			}
			counts[name] = value
			valueByStatus, err := a.Store.JobOutcomeCountsSince(r.Context(), since)
			if err != nil {
				http.Error(w, "could not summarize job outcomes", http.StatusInternalServerError)
				return
			}
			outcomes[name] = valueByStatus
			retryCount, err := a.Store.RetryCountSince(r.Context(), since)
			if err != nil {
				http.Error(w, "could not summarize job retries", http.StatusInternalServerError)
				return
			}
			retries[name] = retryCount
		}
		timeline, err := a.Store.IntakeTimeline(r.Context(), time.Now())
		if err != nil {
			http.Error(w, "could not read intake timeline", http.StatusInternalServerError)
			return
		}
		queue, err := a.Store.JobQueueStats(r.Context())
		if err != nil {
			http.Error(w, "could not read job queue", http.StatusInternalServerError)
			return
		}
		latency, latencyByKind, err := a.Store.PhaseLatencyBreakdown(r.Context(), 5000)
		if err != nil {
			http.Error(w, "could not read phase latency", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"generated_at": time.Now().UTC(),
			"intake": map[string]any{"counts": counts, "timeline": timeline, "recent": intake,
				"pagination": map[string]any{"page": intakePage, "page_size": intakePageSize, "total": intakeTotal}},
			"jobs":            jobs,
			"jobs_pagination": map[string]any{"page": page, "page_size": pageSize, "total": jobTotal},
			"job_outcomes":    outcomes,
			"retry_attempts":  retries,
			"service": map[string]any{
				"queue": queue, "ready_accounts": manager.ReadyAccounts(),
				"connected_accounts":      manager.ConnectedAccounts(),
				"strict_cookie_freshness": manager.RequiresLiveCookieSync(),
				"cookie_sync_age_seconds": manager.CookieSyncAges(),
			},
			"latency": latency, "latency_by_kind": latencyByKind,
		})
	})))
	mux.Handle("/health", serviceHealthHandler(manager, false))
	mux.Handle("/readyz", serviceHealthHandler(manager, true))
	server := &http.Server{
		Addr: listenAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second,
	}
	bridgeErr := make(chan error, 1)
	serverErr := make(chan error, 1)
	managerDone := make(chan struct{})
	jobsDone := make(chan struct{})
	bridgeDone := make(chan struct{})
	cleanupDone := make(chan struct{})
	go func() { manager.Start(serviceCtx); close(managerDone) }()
	go func() { jobs.Start(serviceCtx); close(jobsDone) }()
	go func() {
		defer close(cleanupDone)
		pruneRetention(serviceCtx)
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-serviceCtx.Done():
				return
			case <-ticker.C:
				pruneRetention(serviceCtx)
			case <-retentionWake:
				pruneRetention(serviceCtx)
			}
		}
	}()
	go func() {
		bridgeErr <- a.Bridge.Listen(serviceCtx)
		close(bridgeDone)
	}()
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	log.Printf("app: generation service listening on http://%s with bridge ws://%s",
		listenAddr, a.Bridge.ListenAddr)
	log.Printf("service_metric phase=service_start_to_listening elapsed_ms=%d", time.Since(serviceStart).Milliseconds())
	var runErr error
	select {
	case <-ctx.Done():
	case err := <-bridgeErr:
		runErr = err
	case err := <-serverErr:
		runErr = err
	}
	// Close the intake and prevent new leases first. Let active handlers finish
	// briefly, then drain already leased jobs while the bridge and account
	// manager remain alive so they can use fresh cookie/session updates.
	jobs.StopClaiming()
	httpShutdownCtx, cancelHTTP := context.WithTimeout(context.Background(), 3*time.Second)
	shutdownErr := server.Shutdown(httpShutdownCtx)
	cancelHTTP()
	if shutdownErr != nil {
		_ = server.Close()
	}
	drainCtx, cancelDrain := context.WithTimeout(context.Background(), time.Duration(drainSeconds)*time.Second)
	drainErr := jobs.Drain(drainCtx)
	cancelDrain()
	if drainErr != nil {
		log.Printf("app: job drain deadline reached; persisting remaining work for recovery: %v", drainErr)
		if shutdownErr == nil {
			shutdownErr = fmt.Errorf("app: job drain: %w", drainErr)
		}
	}
	stop()
	<-jobsDone
	manager.WaitBackground()
	<-managerDone
	<-bridgeDone
	<-cleanupDone
	if runErr != nil {
		return runErr
	}
	return shutdownErr
}

func serviceDrainSecondsFromEnv() (int, error) {
	const name = "FLOW_SERVICE_DRAIN_SECONDS"
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 30, nil
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 1 || seconds > 3600 {
		return 0, fmt.Errorf("app: %s must be an integer from 1 to 3600", name)
	}
	return seconds, nil
}

func serviceHealthHandler(manager *service.AccountManager, readiness bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		readyAccounts := manager.ReadyAccounts()
		initialized := manager.Initialized()
		status := "ok"
		if readiness && (!initialized || readyAccounts == 0) {
			status = "not_ready"
			w.WriteHeader(http.StatusServiceUnavailable)
		} else if readiness {
			status = "ready"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": status, "initialized": initialized,
			"ready_accounts": readyAccounts, "connected_accounts": manager.ConnectedAccounts(),
			"strict_cookie_freshness": manager.RequiresLiveCookieSync(),
		})
	})
}

func positiveEnvInt(name string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("app: %s must be a positive integer", name)
	}
	return value, nil
}

func jobMaxAttemptsFromEnv() (int, error) {
	value, err := positiveEnvInt("FLOW_JOB_MAX_ATTEMPTS", 3)
	if err != nil {
		return 0, err
	}
	if value > 20 {
		return 0, fmt.Errorf("app: FLOW_JOB_MAX_ATTEMPTS must be between 1 and 20")
	}
	return value, nil
}

func jobRetryBudgetFromEnv(name string, fallback int) (int, error) {
	value, err := positiveEnvInt(name, fallback)
	if err != nil {
		return 0, err
	}
	if value > 20 {
		return 0, fmt.Errorf("app: %s must be between 1 and 20", name)
	}
	return value, nil
}

func queueWorkersFromEnv(serviceWorkers int) (int, error) {
	if serviceWorkers < 1 {
		serviceWorkers = 1
	}
	defaultWorkers := serviceWorkers * 4
	if defaultWorkers < 16 {
		defaultWorkers = 16
	}
	if defaultWorkers > 256 {
		defaultWorkers = 256
	}
	value, err := positiveEnvInt("FLOW_QUEUE_WORKERS", defaultWorkers)
	if err != nil {
		return 0, err
	}
	if value > 256 {
		return 0, fmt.Errorf("app: FLOW_QUEUE_WORKERS must be between 1 and 256")
	}
	return value, nil
}

func mediaConcurrencyFromEnv(name string, fallback int) (int, error) {
	if fallback < 1 {
		fallback = 1
	}
	if fallback > 256 {
		fallback = 256
	}
	value, err := positiveEnvInt(name, fallback)
	if err != nil {
		return 0, err
	}
	if value > 256 {
		return 0, fmt.Errorf("app: %s must be between 1 and 256", name)
	}
	return value, nil
}
