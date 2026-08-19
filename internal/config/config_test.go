package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The shipped example is the thing people copy; if it drifts out of sync with
// the struct it is worse than no example at all.
func TestLoadExampleConfig(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "config.example.json"))
	if err != nil {
		t.Fatalf("loading config.example.json: %v", err)
	}

	if cfg.MQTT.Broker == "" {
		t.Error("primary broker is empty")
	}
	if len(cfg.ExtraBrokers) != 1 {
		t.Fatalf("ExtraBrokers = %d, want 1", len(cfg.ExtraBrokers))
	}
	if cfg.MQTTAuth.Audience == "" {
		t.Error("mqttAuth.audience is empty, so observer auth would be disabled")
	}
	// The consumer login is checked against mqttAuth by the broker, so the two
	// halves of the example must agree or a copy-paste deploy fails to ingest.
	if cfg.ExtraBrokers[0].Username != cfg.MQTTAuth.ConsumerUsername ||
		cfg.ExtraBrokers[0].Password != cfg.MQTTAuth.ConsumerPassword {
		t.Error("extraBrokers[0] credentials do not match mqttAuth consumer credentials")
	}
}

func TestExtraBrokerDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body := `{
	  "mqtt": {"broker": "tcp://a:1883", "clientID": "ridgelined", "topics": ["meshcore/+/+/packets"]},
	  "extraBrokers": [{"broker": "tcp://b:1883"}, {"broker": "tcp://c:1883"}]
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Distinct client IDs matter: two clients sharing one on a broker evict each
	// other in a reconnect loop.
	seen := map[string]bool{cfg.MQTT.ClientID: true}
	for i, b := range cfg.ExtraBrokers {
		if b.ClientID == "" {
			t.Errorf("extraBrokers[%d]: ClientID not defaulted", i)
		}
		if seen[b.ClientID] {
			t.Errorf("extraBrokers[%d]: duplicate ClientID %q", i, b.ClientID)
		}
		seen[b.ClientID] = true
		if len(b.Topics) == 0 {
			t.Errorf("extraBrokers[%d]: topics not inherited from the primary", i)
		}
	}
}

func TestSubscriberDefaultsAndValidation(t *testing.T) {
	write := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
		return path
	}
	const consumer = `"consumerUsername": "ridgelined", "consumerPassword": "s3cret"`

	t.Run("topics default to the whole feed", func(t *testing.T) {
		cfg, err := Load(write(t, `{"mqttAuth": {"audience": "a", `+consumer+`,
		  "subscribers": [{"username": "site", "password": "pw"}]}}`))
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		got := cfg.MQTTAuth.Subscribers[0].Topics
		if len(got) != 1 || got[0] != "meshcore/#" {
			t.Errorf("Topics = %v, want [meshcore/#]", got)
		}
	})

	// Each of these would produce a broker with an ambiguous or wide-open
	// account, so loading must fail rather than start and serve it.
	rejected := map[string]string{
		"no password":        `{"username": "site"}`,
		"empty username":     `{"password": "pw"}`,
		"observer prefix":    `{"username": "v1_ABCD", "password": "pw"}`,
		"consumer collision": `{"username": "ridgelined", "password": "pw"}`,
	}
	for name, sub := range rejected {
		t.Run(name+" is refused", func(t *testing.T) {
			if _, err := Load(write(t, `{"mqttAuth": {"audience": "a", `+consumer+`,
			  "subscribers": [`+sub+`]}}`)); err == nil {
				t.Fatal("Load succeeded, want an error")
			}
		})
	}

	t.Run("duplicate usernames are refused", func(t *testing.T) {
		if _, err := Load(write(t, `{"mqttAuth": {"audience": "a", `+consumer+`,
		  "subscribers": [{"username": "site", "password": "a"}, {"username": "site", "password": "b"}]}}`)); err == nil {
			t.Fatal("Load succeeded, want an error")
		}
	})
}
