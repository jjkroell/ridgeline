// Package meshcore decodes MeshCore LoRa mesh packets from their raw wire
// form into structured Go values.
//
// The wire format and decode logic are ported from the MIT-licensed
// meshcore-decoder by Michael Hart (https://github.com/michaelhart/meshcore-decoder),
// which itself references the MeshCore firmware packet documentation
// (https://github.com/meshcore-dev/MeshCore/blob/main/docs/packet_structure.md).
//
// Original work Copyright (c) 2025 Michael Hart, MIT License.
package meshcore

// RouteType is the 2-bit routing mode in the packet header (bits 0-1).
type RouteType uint8

const (
	RouteTransportFlood  RouteType = 0x00
	RouteFlood           RouteType = 0x01
	RouteDirect          RouteType = 0x02
	RouteTransportDirect RouteType = 0x03
)

func (r RouteType) String() string {
	switch r {
	case RouteTransportFlood:
		return "TransportFlood"
	case RouteFlood:
		return "Flood"
	case RouteDirect:
		return "Direct"
	case RouteTransportDirect:
		return "TransportDirect"
	default:
		return "Unknown"
	}
}

// PayloadType is the 4-bit payload classifier in the packet header (bits 2-5).
type PayloadType uint8

const (
	PayloadRequest     PayloadType = 0x00
	PayloadResponse    PayloadType = 0x01
	PayloadTextMessage PayloadType = 0x02
	PayloadAck         PayloadType = 0x03
	PayloadAdvert      PayloadType = 0x04
	PayloadGroupText   PayloadType = 0x05
	PayloadGroupData   PayloadType = 0x06
	PayloadAnonRequest PayloadType = 0x07
	PayloadPath        PayloadType = 0x08
	PayloadTrace       PayloadType = 0x09
	PayloadMultipart   PayloadType = 0x0A
	PayloadControl     PayloadType = 0x0B
	PayloadRawCustom   PayloadType = 0x0F
)

func (p PayloadType) String() string {
	switch p {
	case PayloadRequest:
		return "Request"
	case PayloadResponse:
		return "Response"
	case PayloadTextMessage:
		return "TextMessage"
	case PayloadAck:
		return "Ack"
	case PayloadAdvert:
		return "Advert"
	case PayloadGroupText:
		return "GroupText"
	case PayloadGroupData:
		return "GroupData"
	case PayloadAnonRequest:
		return "AnonRequest"
	case PayloadPath:
		return "Path"
	case PayloadTrace:
		return "Trace"
	case PayloadMultipart:
		return "Multipart"
	case PayloadControl:
		return "Control"
	case PayloadRawCustom:
		return "RawCustom"
	default:
		return "Unknown"
	}
}

// DeviceRole is the node role advertised in the lower 4 bits of an Advert's
// app-flags byte.
type DeviceRole uint8

const (
	RoleUnknown    DeviceRole = 0x00
	RoleChatNode   DeviceRole = 0x01
	RoleRepeater   DeviceRole = 0x02
	RoleRoomServer DeviceRole = 0x03
	RoleSensor     DeviceRole = 0x04
)

func (d DeviceRole) String() string {
	switch d {
	case RoleChatNode:
		return "ChatNode"
	case RoleRepeater:
		return "Repeater"
	case RoleRoomServer:
		return "RoomServer"
	case RoleSensor:
		return "Sensor"
	default:
		return "Unknown"
	}
}

// Advert app-flag bits.
const (
	advertFlagHasLocation = 0x10
	advertFlagHasFeature1 = 0x20
	advertFlagHasFeature2 = 0x40
	advertFlagHasName     = 0x80
)

// Packet is a decoded MeshCore packet.
type Packet struct {
	// MessageHash is an 8-hex-digit identifier derived from the packet's
	// route-invariant content, used to deduplicate observations of the same
	// transmission seen by multiple observers.
	MessageHash string

	RouteType      RouteType
	PayloadType    PayloadType
	PayloadVersion uint8

	// TransportCodes holds the two 16-bit region transport codes present only
	// on TransportFlood/TransportDirect packets. Nil otherwise.
	TransportCodes *[2]uint16

	// PathHopCount is the number of hops recorded in Path.
	PathHopCount int
	// PathHashSize is the number of bytes per hop entry (1, 2, or 3).
	PathHashSize int
	// Path holds one uppercase-hex string per hop, or nil when empty.
	Path []string

	// PayloadRaw is the uppercase-hex of the undecoded payload bytes.
	PayloadRaw string

	// Advert is populated when PayloadType == PayloadAdvert.
	Advert *Advert

	TotalBytes int
	Valid      bool
	Errors     []string
}

// Advert is the decoded body of an Advert (0x04) payload: a node announcing
// its identity, optional location, and optional name.
type Advert struct {
	PublicKey  string // 32-byte Ed25519 key, uppercase hex
	Timestamp  uint32 // unix seconds
	Signature  string // 64-byte Ed25519 signature, uppercase hex
	Flags      uint8
	DeviceRole DeviceRole

	HasLocation bool
	Latitude    float64
	Longitude   float64

	HasName bool
	Name    string
}
