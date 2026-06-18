// Client-side byte-level breakdown of a raw MeshCore packet, for the packet
// detail view. Computes field offsets/values purely from the raw hex (+ a few
// friendly values from the decoded node), mirroring CoreScope's field table and
// the breakdown ranges in internal/meshcore. No backend round-trip.
import type { LiveEvent } from './api';

export interface PacketField {
	off: number | null; // byte offset into the packet (null = derived, non-positional)
	bytes: number; // field length in bytes
	label: string;
	value: string; // display value
	desc?: string; // muted explanatory note
	key: string; // color key (see FIELD_KEYS)
}

export interface ByteRange {
	start: number;
	end: number;
	key: string;
	label: string;
}

const up = (s: string) => s.toUpperCase();
const trunc = (s: string, n: number) => (s.length > n ? s.slice(0, n) + '…' : s);

/** Little-endian uint32 from 8 hex chars. */
function le32(h: string): number {
	if (h.length < 8) return 0;
	return (
		parseInt(h.slice(0, 2), 16) +
		parseInt(h.slice(2, 4), 16) * 0x100 +
		parseInt(h.slice(4, 6), 16) * 0x10000 +
		parseInt(h.slice(6, 8), 16) * 0x1000000
	);
}

/** Signed little-endian int32 from 8 hex chars. */
function s32le(h: string): number {
	const v = le32(h);
	return v >= 0x80000000 ? v - 0x100000000 : v;
}

function advTime(h: string): string {
	const t = le32(h);
	return t ? new Date(t * 1000).toISOString().replace('T', ' ').slice(0, 19) + 'Z' : '';
}

function latLon(known: number | undefined, h: string): string {
	const v = known != null ? known : s32le(h) / 1e6;
	return v.toFixed(5);
}

function asciiName(h: string): string {
	let s = '';
	for (let i = 0; i + 1 < h.length; i += 2) {
		const c = parseInt(h.slice(i, i + 2), 16);
		if (c === 0) break;
		if (c >= 32 && c < 127) s += String.fromCharCode(c);
	}
	return s;
}

function flagsDesc(fb: number, role?: string): string {
	if (isNaN(fb)) return '';
	const parts: string[] = [];
	if (role) parts.push(role);
	if (fb & 0x10) parts.push('location');
	if (fb & 0x80) parts.push('name');
	return parts.join(' · ');
}

/**
 * Break a packet's raw hex into labelled fields plus the byte ranges used to
 * colour the hex dump. Transport routes (detected via the presence of transport
 * codes) carry 4 extra bytes before the path-length byte.
 */
export function buildPacketFields(ev: LiveEvent): { fields: PacketField[]; ranges: ByteRange[] } {
	const hex = (ev.raw ?? '').replace(/\s+/g, '');
	const fields: PacketField[] = [];
	const ranges: ByteRange[] = [];
	if (hex.length < 2) return { fields, ranges };

	const totalBytes = Math.floor(hex.length / 2);
	const byte = (b: number) => hex.slice(b * 2, b * 2 + 2);
	const slice = (b: number, n: number) => hex.slice(b * 2, (b + n) * 2);
	const add = (
		off: number | null,
		bytes: number,
		label: string,
		value: string,
		key: string,
		desc?: string
	) => {
		fields.push({ off, bytes, label, value, key, desc });
		if (off != null && bytes > 0) ranges.push({ start: off, end: off + bytes - 1, key, label });
	};

	// Header byte
	add(0, 1, 'Header', '0x' + up(byte(0)), 'header', `Route: ${ev.routeType} · Payload: ${ev.payloadType}`);
	let off = 1;

	// Transport codes (transport routes only) precede the path-length byte.
	if (ev.transportCodes && totalBytes >= 5) {
		add(off, 2, 'Next Hop', up(slice(off, 2)), 'transport');
		add(off + 2, 2, 'Last Hop', up(slice(off + 2, 2)), 'transport');
		off += 4;
	}
	if (off >= totalBytes) return { fields, ranges };

	// Path-length byte: top 2 bits = hash size (1-3), low 6 bits = hop count.
	const pb = parseInt(byte(off), 16);
	const hashSize = isNaN(pb) ? 1 : (pb >> 6) + 1;
	const hashCount = isNaN(pb) ? 0 : pb & 0x3f;
	add(
		off,
		1,
		'Path Length',
		'0x' + up(byte(off)),
		'pathlen',
		hashCount === 0 ? 'hash_count=0 (direct)' : `hash_size=${hashSize}B · hash_count=${hashCount}`
	);
	off += 1;

	// Path hops
	const hops = ev.path ?? [];
	if (hops.length) {
		for (let i = 0; i < hops.length; i++) {
			add(off + i * hashSize, hashSize, `Hop ${i + 1}`, up(hops[i]), 'path');
		}
		off += hashSize * hops.length;
	} else if (hashCount > 0) {
		off += hashSize * hashCount; // advance past path bytes even if not enumerated
	}
	if (off >= totalBytes) return { fields, ranges };

	// Payload — sub-fields for the types we can position from raw bytes.
	const ps = off;
	if (ev.payloadType === 'Advert' && totalBytes - ps >= 100) {
		add(ps, 32, 'Public Key', trunc(up(slice(ps, 32)), 24), 'pubkey');
		add(ps + 32, 4, 'Timestamp', up(slice(ps + 32, 4)), 'timestamp', advTime(slice(ps + 32, 4)));
		add(ps + 36, 64, 'Signature', trunc(up(slice(ps + 36, 64)), 24), 'signature');
		const appStart = ps + 100;
		if (appStart < totalBytes) {
			const fb = parseInt(byte(appStart), 16);
			add(appStart, 1, 'App Flags', '0x' + up(byte(appStart)), 'flags', flagsDesc(fb, ev.node?.role));
			let fOff = appStart + 1;
			if (!isNaN(fb)) {
				if (fb & 0x10 && fOff + 8 <= totalBytes) {
					add(fOff, 4, 'Latitude', latLon(ev.node?.latitude, slice(fOff, 4)), 'location');
					add(fOff + 4, 4, 'Longitude', latLon(ev.node?.longitude, slice(fOff + 4, 4)), 'location');
					fOff += 8;
				}
				if (fb & 0x20 && fOff + 2 <= totalBytes) fOff += 2; // feature flags
				if (fb & 0x40 && fOff + 2 <= totalBytes) fOff += 2;
				if (fb & 0x80 && fOff < totalBytes) {
					add(fOff, totalBytes - fOff, 'Name', ev.node?.name || asciiName(hex.slice(fOff * 2)), 'name');
				}
			}
		}
	} else if (ev.payloadType === 'GroupText' && totalBytes - ps >= 3) {
		add(ps, 1, 'Channel Hash', '0x' + up(byte(ps)), 'channel');
		add(ps + 1, 2, 'MAC', up(slice(ps + 1, 2)), 'mac');
		add(ps + 3, totalBytes - (ps + 3), 'Encrypted Data', trunc(up(hex.slice((ps + 3) * 2)), 32), 'encrypted');
	} else {
		add(ps, totalBytes - ps, 'Payload', trunc(up(hex.slice(ps * 2)), 32), 'payload');
	}

	return { fields, ranges };
}
