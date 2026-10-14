package cdp

import (
	"testing"
	"time"
)

func TestDoneClosesWhenClientCloses(t *testing.T) {
	client := New(nil)
	select {
	case <-client.Done():
		t.Fatal("Done was closed before the client closed")
	default:
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	select {
	case <-client.Done():
	case <-time.After(time.Second):
		t.Fatal("Done was not closed when the client closed")
	}
}
