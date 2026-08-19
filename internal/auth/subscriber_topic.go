package auth

import "strings"

// AuthorizeSubscriberTopic reports whether a read-only downstream subscriber
// whose configured filters are `allowed` may use `topic` for the given access.
//
// Subscribers are the third account shape on the authenticated broker, after
// observers (publish-only, bound to their own subtree) and ridgelined's ingest
// consumer (superuser). They are third parties pulling the raw packet stream
// for their own site: they authenticate with a plain username/password, never
// hold a node key, and must never write. Write is refused here rather than left
// to the broker, because nothing else in the chain does it -- a subscriber is
// not a superuser, so this function is the only gate between it and publishing
// a forged packet under an observer's identity.
func AuthorizeSubscriberTopic(allowed []string, topic string, write bool) bool {
	if write || topic == "" {
		return false
	}
	// Wildcards never match a $-prefixed topic (MQTT 4.7.2), and a subscriber
	// has no business reading $SYS regardless of how loose its filters are.
	if strings.HasPrefix(topic, "$") {
		return false
	}
	for _, filter := range allowed {
		if TopicFilterCovers(filter, topic) {
			return true
		}
	}
	return false
}

// TopicFilterCovers reports whether every topic matching `requested` also
// matches `filter`.
//
// This is NOT plain MQTT topic matching. At ACL time the topic a client sends
// is a concrete name on a read but the SUBSCRIPTION FILTER on a subscribe, so
// `requested` may itself carry wildcards -- and "meshcore/+/+/packets" must not
// be judged against "meshcore/YVR/#" by pretending the + is a literal. Coverage
// is the correct question in both cases: a wildcard in `requested` is only
// covered by a wildcard at least as wide in `filter`.
func TopicFilterCovers(filter, requested string) bool {
	f := strings.Split(filter, "/")
	r := strings.Split(requested, "/")

	for i, fs := range f {
		if fs == "#" {
			// Multi-level wildcard, which is only legal last and matches the
			// remainder including zero levels. Anything left in `requested` is
			// covered whatever it contains.
			return i == len(f)-1
		}
		if i >= len(r) {
			return false // requested is shorter and filter still demands levels
		}
		switch fs {
		case "+":
			// A single-level wildcard cannot cover a multi-level one.
			if r[i] == "#" {
				return false
			}
		default:
			// A literal covers only itself; a wildcard in `requested` would
			// reach names this filter never granted.
			if r[i] != fs {
				return false
			}
		}
	}
	return len(r) == len(f)
}
