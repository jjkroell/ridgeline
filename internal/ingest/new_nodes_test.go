package ingest

import (
	"testing"
	"time"
)

// One observer exhausts its ration without touching another's, and the ration
// refills over time.
func TestNewNodeRation(t *testing.T) {
	var r newNodeRation
	now := time.Now()
	for i := 0; i < newNodeBurst; i++ {
		if !r.allow("a", now) {
			t.Fatalf("denied within burst at %d", i)
		}
	}
	if r.allow("a", now) {
		t.Fatal("allowed past the burst")
	}
	if !r.allow("b", now) {
		t.Fatal("another observer was charged for the first one's nodes")
	}
	if !r.allow("a", now.Add(time.Duration(1/newNodeRate)*time.Second)) {
		t.Fatal("ration did not refill")
	}
}
