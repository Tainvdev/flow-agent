package batchexecute

import (
	"errors"
	"testing"
)

func TestHTTP429ClassificationIsScopedToGenerationRetries(t *testing.T) {
	tests := []struct {
		name         string
		rpcID        string
		wantSafe     bool
		wantThrottle bool
	}{
		{name: "image generation", rpcID: RPCIDGenerate, wantSafe: true, wantThrottle: true},
		{name: "reference video generation", rpcID: RPCIDGenerateVideoReferences, wantSafe: true, wantThrottle: true},
		{name: "media upload", rpcID: RPCIDUploadMedia, wantThrottle: true},
		{name: "read operation", rpcID: "project-list", wantThrottle: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := responseStatusError(tc.rpcID, 429, "rate limited")
			var throttle interface{ Throttled() bool }
			if !errors.As(err, &throttle) || throttle.Throttled() != tc.wantThrottle {
				t.Fatalf("throttle classification=(%v,%v), want %v", throttle, err, tc.wantThrottle)
			}
			var safe interface{ SafeToRetry() bool }
			gotSafe := errors.As(err, &safe) && safe.SafeToRetry()
			if gotSafe != tc.wantSafe {
				t.Fatalf("safe retry=%v, want %v", gotSafe, tc.wantSafe)
			}
			if tc.wantSafe {
				var classifier interface{ RetryClass() string }
				if !errors.As(err, &classifier) || classifier.RetryClass() != "account_throttle" {
					t.Fatalf("retry class=(%v,%v), want account_throttle", classifier, err)
				}
			}
		})
	}
}

func TestHTTP5xxGenerationOutcomeIsUnknownNotRetryable(t *testing.T) {
	for _, rpcID := range []string{RPCIDGenerate, RPCIDUploadMedia} {
		err := responseStatusError(rpcID, 503, "upstream unavailable")
		var unknown interface{ SubmissionUnknown() bool }
		if !errors.As(err, &unknown) || !unknown.SubmissionUnknown() {
			t.Fatalf("rpc=%s error=%v, want submission-unknown classification", rpcID, err)
		}
		var safe interface{ SafeToRetry() bool }
		if errors.As(err, &safe) && safe.SafeToRetry() {
			t.Fatalf("rpc=%s error=%v must not be blindly retried", rpcID, err)
		}
	}
}
