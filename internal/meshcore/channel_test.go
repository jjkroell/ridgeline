package meshcore

import "testing"

// Test vector from the meshcore-decoder reference suite: a public-channel
// GroupText from "🌲 Tree".
func TestDecodeGroupTextPublicChannel(t *testing.T) {
	const raw = "150011C3C1354D619BAE9590E4D177DB7EEAF982F5BDCF78005D75157D9535FA90178F785D"
	pkt, err := DecodeHex(raw)
	if err != nil {
		t.Fatal(err)
	}
	if pkt.PayloadType != PayloadGroupText {
		t.Fatalf("payload type = %v, want GroupText", pkt.PayloadType)
	}
	gt := pkt.GroupText
	if gt == nil {
		t.Fatal("GroupText not decoded")
	}
	if gt.ChannelHash != "11" {
		t.Errorf("channel hash = %q, want 11", gt.ChannelHash)
	}
	if gt.MAC != "C3C1" {
		t.Errorf("MAC = %q, want C3C1", gt.MAC)
	}
	if !gt.Decrypted {
		t.Fatal("message not decrypted")
	}
	if gt.Channel != "Public" {
		t.Errorf("channel = %q, want Public", gt.Channel)
	}
	if gt.Sender != "🌲 Tree" {
		t.Errorf("sender = %q, want 🌲 Tree", gt.Sender)
	}
	if gt.Message != "☁️" {
		t.Errorf("message = %q, want ☁️", gt.Message)
	}
	if gt.Timestamp != 1758484279 {
		t.Errorf("timestamp = %d, want 1758484279", gt.Timestamp)
	}
}
