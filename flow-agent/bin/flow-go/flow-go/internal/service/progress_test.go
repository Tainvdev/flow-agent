package service

import "testing"

type progressRecorder struct{ phases []string }

func (r *progressRecorder) SetPhase(phase, _ string) error {
	r.phases = append(r.phases, phase)
	return nil
}
func (*progressRecorder) SetUpstreamIDs([]string) error            { return nil }
func (*progressRecorder) BindAccount(string, string, string) error { return nil }

func TestSubmissionGateReleasesAtMediaAvailabilityOnlyOnce(t *testing.T) {
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	released := 0
	reporter := &submissionGateReporter{
		next: &progressRecorder{},
		release: func() {
			<-gate
			released++
		},
	}
	if err := reporter.SetPhase("submitting", ""); err != nil {
		t.Fatal(err)
	}
	if len(gate) != 1 {
		t.Fatal("submission permit released before media became available")
	}
	if err := reporter.SetPhase("media_available", ""); err != nil {
		t.Fatal(err)
	}
	if len(gate) != 0 || released != 1 {
		t.Fatalf("permit after media availability: held=%d releases=%d, want held=0 releases=1", len(gate), released)
	}
	if err := reporter.SetPhase("rendering", ""); err != nil {
		t.Fatal(err)
	}
	if err := reporter.SetPhase("media_available", "duplicate"); err != nil {
		t.Fatal(err)
	}
	if released != 1 {
		t.Fatalf("permit release count=%d, want exactly one", released)
	}
}
