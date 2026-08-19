package auth

import "testing"

func TestTopicFilterCovers(t *testing.T) {
	cases := []struct {
		filter, requested string
		want              bool
	}{
		// Concrete topics, as seen on a read check.
		{"meshcore/#", "meshcore/YVR/AABB/packets", true},
		{"meshcore/#", "meshcore", true}, // # matches zero levels
		{"meshcore/YCD/#", "meshcore/YCD/AABB/packets", true},
		{"meshcore/YCD/#", "meshcore/YVR/AABB/packets", false},
		{"meshcore/+/+/packets", "meshcore/YVR/AABB/packets", true},
		{"meshcore/+/+/packets", "meshcore/YVR/AABB/status", false},
		{"meshcore/+/+/packets", "meshcore/YVR/AABB", false},

		// Subscription filters, as seen on a subscribe check. A requested
		// wildcard must be no wider than the granted one.
		{"meshcore/#", "meshcore/+/+/packets", true},
		{"meshcore/#", "meshcore/#", true},
		{"meshcore/YCD/#", "meshcore/+/+/packets", false},
		{"meshcore/+/+/packets", "meshcore/+/+/#", false},
		{"meshcore/#", "#", false},
		{"meshcore/#", "$SYS/#", false},
	}
	for _, tc := range cases {
		if got := TopicFilterCovers(tc.filter, tc.requested); got != tc.want {
			t.Errorf("TopicFilterCovers(%q, %q) = %v, want %v", tc.filter, tc.requested, got, tc.want)
		}
	}
}

func TestAuthorizeSubscriberTopic(t *testing.T) {
	allowed := []string{"meshcore/YCD/#", "meshcore/YVR/#"}

	if !AuthorizeSubscriberTopic(allowed, "meshcore/YVR/AABB/packets", false) {
		t.Error("a topic inside the second filter should be readable")
	}
	// The whole point of the account: read-only, whatever the topic.
	if AuthorizeSubscriberTopic(allowed, "meshcore/YVR/AABB/packets", true) {
		t.Error("a subscriber must never be allowed to publish, even in scope")
	}
	if AuthorizeSubscriberTopic(allowed, "meshcore/YQQ/AABB/packets", false) {
		t.Error("a region outside every filter should be denied")
	}
	if AuthorizeSubscriberTopic(nil, "meshcore/YVR/AABB/packets", false) {
		t.Error("no configured filters should grant nothing")
	}
	// Broker internals stay off-limits even when the filters are wide open.
	if AuthorizeSubscriberTopic([]string{"#"}, "$SYS/broker/clients/total", false) {
		t.Error("$SYS must not be readable")
	}
	if AuthorizeSubscriberTopic(allowed, "", false) {
		t.Error("an empty topic should be denied")
	}
}
