// Advert decode logic ported from the MIT-licensed meshcore-decoder by
// Michael Hart (https://github.com/michaelhart/meshcore-decoder).
// Original work Copyright (c) 2025 Michael Hart, MIT License.

package meshcore

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

// advertMinLen is the fixed prefix: public_key(32) + timestamp(4) +
// signature(64) + flags(1).
const advertMinLen = 32 + 4 + 64 + 1

// decodeAdvert parses an Advert payload. It returns nil only when the payload
// is too short to contain the fixed prefix.
func decodeAdvert(payload []byte) *Advert {
	if len(payload) < advertMinLen {
		return nil
	}

	a := &Advert{
		PublicKey: strings.ToUpper(hex.EncodeToString(payload[0:32])),
		Timestamp: binary.LittleEndian.Uint32(payload[32:36]),
		Signature: strings.ToUpper(hex.EncodeToString(payload[36:100])),
		Flags:     payload[100],
	}
	a.DeviceRole = parseDeviceRole(a.Flags)
	a.HasLocation = a.Flags&advertFlagHasLocation != 0
	a.HasName = a.Flags&advertFlagHasName != 0

	offset := advertMinLen

	if a.HasLocation && len(payload) >= offset+8 {
		a.Latitude = float64(int32(binary.LittleEndian.Uint32(payload[offset:offset+4]))) / 1e6
		a.Longitude = float64(int32(binary.LittleEndian.Uint32(payload[offset+4:offset+8]))) / 1e6
		offset += 8
	}

	// Feature fields are not yet interpreted, but their presence shifts the
	// name offset.
	if a.Flags&advertFlagHasFeature1 != 0 {
		offset += 2
	}
	if a.Flags&advertFlagHasFeature2 != 0 {
		offset += 2
	}

	if a.HasName && len(payload) > offset {
		a.Name = decodeNodeName(payload[offset:])
	}

	return a
}

func parseDeviceRole(flags uint8) DeviceRole {
	switch flags & 0x0F {
	case 0x01:
		return RoleChatNode
	case 0x02:
		return RoleRepeater
	case 0x03:
		return RoleRoomServer
	case 0x04:
		return RoleSensor
	default:
		return RoleChatNode
	}
}

// decodeNodeName interprets the trailing name bytes as UTF-8 truncated at the
// first NUL, with control characters stripped and surrounding space trimmed.
func decodeNodeName(b []byte) string {
	if i := indexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	s := string(b)
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7F {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

func indexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}
