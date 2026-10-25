package ingest

import "testing"

// The topic key is ACL-bound to the publisher's token; origin_id is whatever
// the publisher typed, so it may only stand in when the topic has no key.
func TestObserverPubkeyPrefersTopicKey(t *testing.T) {
	if got := observerPubkey("TOPICKEY", "SOMEONEELSE"); got != "TOPICKEY" {
		t.Errorf("topic key must win over origin_id, got %q", got)
	}
	if got := observerPubkey("", "ORIGIN"); got != "ORIGIN" {
		t.Errorf("origin_id should fill in when the topic has no key, got %q", got)
	}
}
