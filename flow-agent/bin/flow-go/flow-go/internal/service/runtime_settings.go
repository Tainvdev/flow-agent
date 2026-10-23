package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/kodelyx/flow-go/flow-go/internal/store"
)

const runtimeSettingPrefix = "runtime."

// RuntimeSettings are values that can be changed without restarting service.
type RuntimeSettings struct {
	SubmissionConcurrency    int    `json:"submission_concurrency"`
	QueueWorkers             int    `json:"queue_workers"`
	AccountConcurrency       int    `json:"account_concurrency"`
	MediaPollConcurrency     int    `json:"media_poll_concurrency"`
	MediaDownloadConcurrency int    `json:"media_download_concurrency"`
	MaxOutstandingJobs       int    `json:"max_outstanding_jobs"`
	MaxJobAttempts           int    `json:"max_job_attempts"`
	PreSubmitAttempts        int    `json:"pre_submit_attempts"`
	ResumeAttempts           int    `json:"resume_attempts"`
	ThrottleAttempts         int    `json:"throttle_attempts"`
	JobRetentionDays         int    `json:"job_retention_days"`
	CookieFreshness          string `json:"cookie_freshness"`
}

func (s RuntimeSettings) Validate() error {
	for key, value := range map[string]int{
		"submission_concurrency":     s.SubmissionConcurrency,
		"queue_workers":              s.QueueWorkers,
		"media_poll_concurrency":     s.MediaPollConcurrency,
		"media_download_concurrency": s.MediaDownloadConcurrency,
		"max_outstanding_jobs":       s.MaxOutstandingJobs,
	} {
		max := 256
		if key == "max_outstanding_jobs" {
			max = 100000
		}
		if value < 1 || value > max {
			return fmt.Errorf("%s must be between 1 and %d", key, max)
		}
	}
	if s.AccountConcurrency < 1 || s.AccountConcurrency > 16 {
		return fmt.Errorf("account_concurrency must be between 1 and 16")
	}
	for key, value := range map[string]int{
		"max_job_attempts":    s.MaxJobAttempts,
		"pre_submit_attempts": s.PreSubmitAttempts,
		"resume_attempts":     s.ResumeAttempts,
		"throttle_attempts":   s.ThrottleAttempts,
	} {
		if value < 1 || value > maxJobMaxAttempts {
			return fmt.Errorf("%s must be between 1 and %d", key, maxJobMaxAttempts)
		}
	}
	if s.JobRetentionDays < 1 || s.JobRetentionDays > 3650 {
		return fmt.Errorf("job_retention_days must be between 1 and 3650")
	}
	if !strings.EqualFold(s.CookieFreshness, "strict") && !strings.EqualFold(s.CookieFreshness, "cached") {
		return fmt.Errorf("cookie_freshness must be strict or cached")
	}
	return nil
}

func runtimeSettingValues(s RuntimeSettings) map[string]string {
	return map[string]string{
		"runtime.submission_concurrency":     strconv.Itoa(s.SubmissionConcurrency),
		"runtime.queue_workers":              strconv.Itoa(s.QueueWorkers),
		"runtime.account_concurrency":        strconv.Itoa(s.AccountConcurrency),
		"runtime.media_poll_concurrency":     strconv.Itoa(s.MediaPollConcurrency),
		"runtime.media_download_concurrency": strconv.Itoa(s.MediaDownloadConcurrency),
		"runtime.max_outstanding_jobs":       strconv.Itoa(s.MaxOutstandingJobs),
		"runtime.max_job_attempts":           strconv.Itoa(s.MaxJobAttempts),
		"runtime.pre_submit_attempts":        strconv.Itoa(s.PreSubmitAttempts),
		"runtime.resume_attempts":            strconv.Itoa(s.ResumeAttempts),
		"runtime.throttle_attempts":          strconv.Itoa(s.ThrottleAttempts),
		"runtime.job_retention_days":         strconv.Itoa(s.JobRetentionDays),
		"runtime.cookie_freshness":           strings.ToLower(strings.TrimSpace(s.CookieFreshness)),
	}
}

func LoadRuntimeSettings(st *store.Store, defaults RuntimeSettings) (RuntimeSettings, map[string]string, error) {
	if err := defaults.Validate(); err != nil {
		return RuntimeSettings{}, nil, fmt.Errorf("invalid runtime defaults: %w", err)
	}
	stored, err := st.Settings(runtimeSettingPrefix)
	if err != nil {
		return RuntimeSettings{}, nil, err
	}
	settings := defaults
	sources := make(map[string]string, len(runtimeSettingValues(defaults)))
	for key := range runtimeSettingValues(defaults) {
		name := strings.TrimPrefix(key, runtimeSettingPrefix)
		sources[name] = "startup default"
		value, found := stored[key]
		if !found {
			continue
		}
		if err := setRuntimeSetting(&settings, name, value); err != nil {
			return RuntimeSettings{}, nil, fmt.Errorf("invalid saved setting %s: %w", name, err)
		}
		sources[name] = "admin"
	}
	if err := settings.Validate(); err != nil {
		return RuntimeSettings{}, nil, fmt.Errorf("invalid saved runtime settings: %w", err)
	}
	return settings, sources, nil
}

func MergeRuntimeSettings(defaults, current RuntimeSettings, values map[string]json.RawMessage, reset []string) (RuntimeSettings, error) {
	currentJSON, _ := json.Marshal(current)
	defaultsJSON, _ := json.Marshal(defaults)
	var next map[string]json.RawMessage
	var fallback map[string]json.RawMessage
	_ = json.Unmarshal(currentJSON, &next)
	_ = json.Unmarshal(defaultsJSON, &fallback)
	known := runtimeSettingValues(defaults)
	for key, raw := range values {
		if _, ok := known[runtimeSettingPrefix+key]; !ok {
			return RuntimeSettings{}, fmt.Errorf("unknown runtime setting %q", key)
		}
		if len(raw) == 0 {
			return RuntimeSettings{}, fmt.Errorf("%s has an empty value", key)
		}
		next[key] = raw
	}
	for _, key := range reset {
		if _, ok := known[runtimeSettingPrefix+key]; !ok {
			return RuntimeSettings{}, fmt.Errorf("unknown runtime setting %q", key)
		}
		next[key] = fallback[key]
	}
	encoded, err := json.Marshal(next)
	if err != nil {
		return RuntimeSettings{}, err
	}
	var settings RuntimeSettings
	if err := json.Unmarshal(encoded, &settings); err != nil {
		return RuntimeSettings{}, fmt.Errorf("invalid runtime settings: %w", err)
	}
	if err := settings.Validate(); err != nil {
		return RuntimeSettings{}, err
	}
	return settings, nil
}

func PersistRuntimeSettings(st *store.Store, settings RuntimeSettings, changed map[string]json.RawMessage, reset []string) error {
	all := runtimeSettingValues(settings)
	values := make(map[string]string, len(changed))
	for key := range changed {
		value, ok := all[runtimeSettingPrefix+key]
		if !ok {
			return fmt.Errorf("unknown runtime setting %q", key)
		}
		values[runtimeSettingPrefix+key] = value
	}
	deletes := make([]string, 0, len(reset))
	for _, key := range reset {
		if _, ok := all[runtimeSettingPrefix+key]; !ok {
			return fmt.Errorf("unknown runtime setting %q", key)
		}
		deletes = append(deletes, runtimeSettingPrefix+key)
	}
	return st.ApplySettings(values, deletes)
}

func setRuntimeSetting(settings *RuntimeSettings, key, raw string) error {
	if key == "cookie_freshness" {
		settings.CookieFreshness = strings.ToLower(strings.TrimSpace(raw))
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("must be an integer")
	}
	switch key {
	case "submission_concurrency":
		settings.SubmissionConcurrency = value
	case "queue_workers":
		settings.QueueWorkers = value
	case "account_concurrency":
		settings.AccountConcurrency = value
	case "media_poll_concurrency":
		settings.MediaPollConcurrency = value
	case "media_download_concurrency":
		settings.MediaDownloadConcurrency = value
	case "max_outstanding_jobs":
		settings.MaxOutstandingJobs = value
	case "max_job_attempts":
		settings.MaxJobAttempts = value
	case "pre_submit_attempts":
		settings.PreSubmitAttempts = value
	case "resume_attempts":
		settings.ResumeAttempts = value
	case "throttle_attempts":
		settings.ThrottleAttempts = value
	case "job_retention_days":
		settings.JobRetentionDays = value
	default:
		return fmt.Errorf("unknown runtime setting %q", key)
	}
	return nil
}

