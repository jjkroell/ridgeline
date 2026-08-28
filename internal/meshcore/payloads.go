// Payload decode logic ported from the MIT-licensed meshcore-decoder by
// Michael Hart (https://github.com/michaelhart/meshcore-decoder).
// Original work Copyright (c) 2025 Michael Hart, MIT License.

package meshcore

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

// upperHex renders bytes as an uppercase hex string.
func upperHex(b []byte) string {
	return strings.ToUpper(hex.EncodeToString(b))
}

// decodeDirectMessage parses the cleartext envelope shared by TextMessage,
// Request, and Response payloads: dest hash(1) + source hash(1) + MAC(2) +
// ciphertext(rest). Returns nil when too short to hold the fixed prefix.
func decodeDirectMessage(payload []byte) *DirectMessage {
	if len(payload) < 4 {
		return nil
	}
	return &DirectMessage{
		DestinationHash: upperHex(payload[0:1]),
		SourceHash:      upperHex(payload[1:2]),
		CipherMAC:       upperHex(payload[2:4]),
		Ciphertext:      upperHex(payload[4:]),
	}
}

// decodeAnonRequest parses an AnonRequest payload: dest hash(1) + sender public
// key(32) + MAC(2) + ciphertext(rest). Returns nil when too short.
func decodeAnonRequest(payload []byte) *AnonRequest {
	if len(payload) < 35 {
		return nil
	}
	return &AnonRequest{
		DestinationHash: upperHex(payload[0:1]),
		SenderPublicKey: upperHex(payload[1:33]),
		CipherMAC:       upperHex(payload[33:35]),
		Ciphertext:      upperHex(payload[35:]),
	}
}

// decodeAck parses an Ack payload: a 4-byte CRC checksum. Returns nil when too
// short.
func decodeAck(payload []byte) *Ack {
	if len(payload) < 4 {
		return nil
	}
	return &Ack{Checksum: upperHex(payload[0:4])}
}

// decodeTrace parses a Trace payload: tag(4) + auth code(4) + flags(1) +
// path hashes. The flags' low two bits select the path-hash size (1<<n). Returns
// nil when too short or the path bytes don't align to the hash size.
func decodeTrace(payload []byte) *Trace {
	if len(payload) < 9 {
		return nil
	}
	t := &Trace{
		Tag:      upperHex(payload[0:4]),
		AuthCode: binary.LittleEndian.Uint32(payload[4:8]),
		Flags:    payload[8],
	}
	t.HashSize = 1 << (t.Flags & 0x03)

	rest := payload[9:]
	if t.HashSize == 0 || len(rest)%t.HashSize != 0 {
		return nil
	}
	for off := 0; off+t.HashSize <= len(rest); off += t.HashSize {
		t.Path = append(t.Path, upperHex(rest[off:off+t.HashSize]))
	}
	return t
}

// decodeControl parses a Control payload. Only the node discovery
// request/response sub-types are interpreted; returns nil for an unknown
// sub-type or one too short for its fields.
func decodeControl(payload []byte) *Control {
	if len(payload) < 1 {
		return nil
	}
	rawFlags := payload[0]
	switch rawFlags & 0xF0 {
	case ControlNodeDiscoverReq:
		// flags(1) + type_filter(1) + tag(4), with an optional since(4).
		if len(payload) < 6 {
			return nil
		}
		c := &Control{
			SubType:    ControlNodeDiscoverReq,
			SubName:    "DiscoverReq",
			RawFlags:   rawFlags,
			PrefixOnly: rawFlags&0x01 != 0,
			TypeFilter: payload[1],
			Tag:        binary.LittleEndian.Uint32(payload[2:6]),
		}
		if len(payload) >= 10 {
			c.Since = binary.LittleEndian.Uint32(payload[6:10])
		}
		return c
	case ControlNodeDiscoverResp:
		// flags(1) + snr(1) + tag(4) + public key (8-byte prefix or 32-byte full).
		if len(payload) < 14 {
			return nil
		}
		return &Control{
			SubType:   ControlNodeDiscoverResp,
			SubName:   "DiscoverResp",
			RawFlags:  rawFlags,
			NodeRole:  DeviceRole(rawFlags & 0x0F),
			SNR:       float64(int8(payload[1])) / 4.0,
			Tag:       binary.LittleEndian.Uint32(payload[2:6]),
			PublicKey: upperHex(payload[6:]),
		}
	default:
		return nil
	}
}

// otaSubName maps an OTA sub-message type to its protocol name.
func otaSubName(t uint8) string {
	switch t {
	case OTAAdv:
		return "Adv"
	case OTAQuery:
		return "Query"
	case OTAHave:
		return "Have"
	case OTAGetManifest:
		return "GetManifest"
	case OTAManifest:
		return "Manifest"
	case OTAReq:
		return "Req"
	case OTAData:
		return "Data"
	case OTAReqProof:
		return "ReqProof"
	case OTAProof:
		return "Proof"
	case OTAGetLeaves:
		return "GetLeaves"
	case OTALeaves:
		return "Leaves"
	default:
		return "Unknown"
	}
}

// decodeOTA decodes an OTA-over-LoRa (0x0C) payload. Layout mirrors the
// encoders in MeshCore src/helpers/ota/OtaProtocol.cpp: byte 0 is the
// sub-type, followed by a 4-byte seeder id (ADV/QUERY/HAVE) or manifest id
// (everything else). All multi-byte fields are little-endian.
//
// The payload is plaintext by design — an OTA transfer's integrity rests on
// the signed manifest inside the .mota, not on link encryption — so this needs
// no keys and every field is observable.
func decodeOTA(payload []byte) *OTA {
	if len(payload) < 5 {
		return nil
	}
	o := &OTA{SubType: payload[0], SubName: otaSubName(payload[0])}
	id := upperHex(payload[1:5])
	rest := payload[5:]

	switch o.SubType {
	case OTAAdv: // seeder(4) + n_motas(1) + set_digest(4)
		o.SeederID = id
		if len(rest) < 5 {
			return nil
		}
		o.NumMotas = rest[0]
		o.SetDigest = upperHex(rest[1:5])

	case OTAQuery: // seeder(4) + set_digest(4) + filter_target(4)
		o.SeederID = id
		if len(rest) < 8 {
			return nil
		}
		o.SetDigest = upperHex(rest[0:4])
		if len(rest) >= 8 {
			o.FilterTarget = binary.LittleEndian.Uint32(rest[4:8])
		}

	case OTAHave: // seeder(4) + set_digest(4) + frag_idx(1) + frag_total(1) + n_rows(1) + rows
		o.SeederID = id
		if len(rest) < 7 {
			return nil
		}
		o.SetDigest = upperHex(rest[0:4])
		o.FragIndex, o.FragTotal = rest[4], rest[5]
		nRows := int(rest[6])
		rows := rest[7:]
		for i := 0; i < nRows; i++ {
			off := i * otaHaveRowBytes
			if off+otaHaveRowBytes > len(rows) {
				break // truncated row: keep what decoded rather than discarding the packet
			}
			r := rows[off : off+otaHaveRowBytes]
			flags := r[13]
			o.Rows = append(o.Rows, OTAHaveRow{
				ManifestID: upperHex(r[0:4]),
				TargetID:   upperHex(r[4:8]),
				FWVersion:  otaVersionString(binary.LittleEndian.Uint32(r[8:12])),
				Codec:      r[12],
				Flags:      flags,
				Full:       flags&0x01 != 0,
				Signed:     flags&0x02 != 0,
				HaveCount:  binary.LittleEndian.Uint16(r[14:16]),
			})
		}

	case OTAGetManifest, OTAGetLeaves: // mid(4) + want_mask(2)
		o.ManifestID = id
		if len(rest) < 2 {
			return nil
		}
		o.WantMask = binary.LittleEndian.Uint16(rest[0:2])

	case OTAManifest, OTALeaves: // mid(4) + frag_idx(1) + frag_total(1) + bytes
		o.ManifestID = id
		if len(rest) < 2 {
			return nil
		}
		o.FragIndex, o.FragTotal = rest[0], rest[1]
		o.DataLen = len(rest) - 2

	case OTAReq: // mid(4) + block_idx(2) + want_mask(2)
		o.ManifestID = id
		if len(rest) < 4 {
			return nil
		}
		o.BlockIndex = binary.LittleEndian.Uint16(rest[0:2])
		o.WantMask = binary.LittleEndian.Uint16(rest[2:4])

	case OTAData: // mid(4) + block_idx(2) + frag_off(2) + data
		o.ManifestID = id
		if len(rest) < 4 {
			return nil
		}
		o.BlockIndex = binary.LittleEndian.Uint16(rest[0:2])
		o.FragOffset = binary.LittleEndian.Uint16(rest[2:4])
		o.DataLen = len(rest) - 4

	case OTAReqProof: // mid(4) + block_idx(2)
		o.ManifestID = id
		if len(rest) < 2 {
			return nil
		}
		o.BlockIndex = binary.LittleEndian.Uint16(rest[0:2])

	case OTAProof: // mid(4) + block_idx(2) + n_proof(1) + proof(n*4)
		o.ManifestID = id
		if len(rest) < 3 {
			return nil
		}
		o.BlockIndex = binary.LittleEndian.Uint16(rest[0:2])
		o.ProofNodes = rest[2]

	default:
		// Unknown sub-type: keep the id so the packet is still attributable.
		o.ManifestID = id
	}
	return o
}

// otaVersionString renders the packed fw_version word used in an OTA_HAVE row
// (major<<24 | minor<<16 | patch<<8 | prerelease) as "v1.17.1".
func otaVersionString(v uint32) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("v%d.%d.%d", (v>>24)&0xFF, (v>>16)&0xFF, (v>>8)&0xFF)
}
