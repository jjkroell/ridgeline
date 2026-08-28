package meshcore

import "testing"

// Vectors are built byte-for-byte from the encoders in MeshCore
// src/helpers/ota/OtaProtocol.cpp (little-endian; byte 0 = OtaMsgType).

func TestDecodeOTAAdv(t *testing.T) {
	// encode_adv: type + seeder_id(4) + n_motas(1) + set_digest(4)
	payload := []byte{0x01, 0xC9, 0xDC, 0x5D, 0x8C, 0x02, 0xDE, 0xAD, 0xBE, 0xEF}
	o := decodeOTA(payload)
	if o == nil {
		t.Fatal("decodeOTA returned nil")
	}
	if o.SubName != "Adv" || o.SubType != OTAAdv {
		t.Errorf("sub-type = %q/%#x, want Adv/0x01", o.SubName, o.SubType)
	}
	if o.SeederID != "C9DC5D8C" {
		t.Errorf("SeederID = %q, want C9DC5D8C", o.SeederID)
	}
	if o.NumMotas != 2 {
		t.Errorf("NumMotas = %d, want 2", o.NumMotas)
	}
	if o.SetDigest != "DEADBEEF" {
		t.Errorf("SetDigest = %q, want DEADBEEF", o.SetDigest)
	}
}

func TestDecodeOTAReqAndData(t *testing.T) {
	// encode_req: type + manifest_id(4) + block_idx(2) + want_mask(2)
	req := []byte{0x06, 0x50, 0x3F, 0xC5, 0xB3, 0x2A, 0x00, 0x7F, 0x00}
	o := decodeOTA(req)
	if o == nil || o.SubName != "Req" {
		t.Fatalf("req: got %+v", o)
	}
	if o.ManifestID != "503FC5B3" {
		t.Errorf("ManifestID = %q, want 503FC5B3", o.ManifestID)
	}
	if o.BlockIndex != 42 || o.WantMask != 0x007F {
		t.Errorf("block/mask = %d/%#x, want 42/0x7f", o.BlockIndex, o.WantMask)
	}

	// encode_data: type + manifest_id(4) + block_idx(2) + frag_off(2) + data
	data := append([]byte{0x07, 0x50, 0x3F, 0xC5, 0xB3, 0x2A, 0x00, 0xA0, 0x00}, make([]byte, 160)...)
	d := decodeOTA(data)
	if d == nil || d.SubName != "Data" {
		t.Fatalf("data: got %+v", d)
	}
	if d.BlockIndex != 42 || d.FragOffset != 160 || d.DataLen != 160 {
		t.Errorf("block/off/len = %d/%d/%d, want 42/160/160", d.BlockIndex, d.FragOffset, d.DataLen)
	}
}

func TestDecodeOTAHaveCatalog(t *testing.T) {
	// encode_have: type + seeder(4) + digest(4) + frag_idx + frag_total + n_rows + rows(16 each)
	// row: mid(4) + target(4) + fw_version(4) + codec(1) + flags(1) + have_count(2)
	payload := []byte{0x03, 0xC9, 0xDC, 0x5D, 0x8C, 0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x01, 0x01}
	payload = append(payload,
		0x50, 0x3F, 0xC5, 0xB3, // mid
		0xFD, 0x13, 0xD4, 0x04, // target 0x04D413FD
		0x00, 0x01, 0x11, 0x01, // fw_version 0x01110100 -> v1.17.1
		0x02,       // codec 2 = detools in-place
		0x02,       // flags: SIGNED, not FULL (delta)
		0x55, 0x00, // have_count 85
	)
	o := decodeOTA(payload)
	if o == nil || o.SubName != "Have" {
		t.Fatalf("have: got %+v", o)
	}
	if len(o.Rows) != 1 {
		t.Fatalf("Rows = %d, want 1", len(o.Rows))
	}
	r := o.Rows[0]
	if r.ManifestID != "503FC5B3" || r.TargetID != "FD13D404" {
		t.Errorf("ids = %q/%q", r.ManifestID, r.TargetID)
	}
	if r.FWVersion != "v1.17.1" {
		t.Errorf("FWVersion = %q, want v1.17.1", r.FWVersion)
	}
	if r.Full || !r.Signed {
		t.Errorf("flags: Full=%v Signed=%v, want false/true", r.Full, r.Signed)
	}
	if r.HaveCount != 85 {
		t.Errorf("HaveCount = %d, want 85", r.HaveCount)
	}
}

func TestDecodeOTATruncatedAndShort(t *testing.T) {
	if o := decodeOTA([]byte{0x01, 0x02}); o != nil {
		t.Errorf("short payload should decode nil, got %+v", o)
	}
	// A HAVE claiming 3 rows but carrying one: keep what decoded rather than
	// discarding an otherwise-valid observation.
	p := []byte{0x03, 1, 2, 3, 4, 5, 6, 7, 8, 0x00, 0x01, 0x03}
	p = append(p, make([]byte, otaHaveRowBytes)...)
	o := decodeOTA(p)
	if o == nil || len(o.Rows) != 1 {
		t.Errorf("truncated HAVE: want 1 row, got %+v", o)
	}
}

func TestPayloadOTAString(t *testing.T) {
	if got := PayloadOTA.String(); got != "OTA" {
		t.Errorf("PayloadOTA.String() = %q, want OTA", got)
	}
	if PayloadOTA != 0x0C {
		t.Errorf("PayloadOTA = %#x, want 0x0C", uint8(PayloadOTA))
	}
}

// Real OTA_ADV beacons captured off the air by two independent Ridgeline
// observers on 2026-08-28T07:53-07:54Z (SNR 11.5-12.75), emitted by the bench
// RAK4631 and Heltec T114 during their boot announce burst. Zero-hop, so the
// packet is header(1) + path_len(1) + payload.
func TestDecodeOTARealCapture(t *testing.T) {
	cases := []struct {
		hex      string
		seeder   string
		digest   string
		numMotas uint8
	}{
		{"31000175c3786b01783c9e4f", "75C3786B", "783C9E4F", 1},
		{"310001ad1f32be0129320271", "AD1F32BE", "29320271", 1},
	}
	for _, tc := range cases {
		p, err := DecodeHex(tc.hex)
		if err != nil {
			t.Fatalf("%s: decode: %v", tc.hex, err)
		}
		if p.PayloadType != PayloadOTA {
			t.Errorf("%s: PayloadType = %s, want OTA", tc.hex, p.PayloadType)
		}
		if p.OTA == nil {
			t.Fatalf("%s: OTA not decoded", tc.hex)
		}
		if p.OTA.SubName != "Adv" {
			t.Errorf("%s: SubName = %q, want Adv", tc.hex, p.OTA.SubName)
		}
		if p.OTA.SeederID != tc.seeder {
			t.Errorf("%s: SeederID = %q, want %q", tc.hex, p.OTA.SeederID, tc.seeder)
		}
		if p.OTA.SetDigest != tc.digest {
			t.Errorf("%s: SetDigest = %q, want %q", tc.hex, p.OTA.SetDigest, tc.digest)
		}
		if p.OTA.NumMotas != tc.numMotas {
			t.Errorf("%s: NumMotas = %d, want %d", tc.hex, p.OTA.NumMotas, tc.numMotas)
		}
	}
}
