// Client-side MeshCore group-channel crypto. Mirrors the Go implementation in
// internal/meshcore/channel.go so channel keys can live in the browser
// (localStorage) and decryption happens without sending keys to the server.
//
// Crypto algorithms ported from the MIT-licensed meshcore-decoder by Michael
// Hart (https://github.com/michaelhart/meshcore-decoder), Copyright (c) 2025.
import { SHA256, HmacSHA256, AES, enc, mode, pad, lib } from 'crypto-js';

/** A group-channel AES key is 16 bytes (the first 16 of a SHA-256 digest). */
export const CHANNEL_KEY_BYTES = 16;

function hexToBytes(hex: string): Uint8Array {
	const out = new Uint8Array(hex.length / 2);
	for (let i = 0; i < out.length; i++) out[i] = parseInt(hex.substr(i * 2, 2), 16);
	return out;
}

/**
 * Derive the 16-byte key for a public "hashtag" channel from its name:
 * key = first 16 bytes of SHA-256(name), with '#' prepended if absent
 * (MeshCore's implicit auto-hashtag behaviour). Returns 32 hex chars.
 */
export function deriveHashtagKey(name: string): string {
	const n = name.startsWith('#') ? name : '#' + name;
	return SHA256(enc.Utf8.parse(n)).toString(enc.Hex).slice(0, CHANNEL_KEY_BYTES * 2);
}

/**
 * MeshCore's 1-byte channel identifier: the first byte of SHA-256 over the
 * channel's 16-byte key, as 2 uppercase hex chars. Multiple channels can share
 * a hash byte; the HMAC check during decryption disambiguates.
 */
export function channelHashByte(keyHex: string): string {
	return SHA256(enc.Hex.parse(keyHex)).toString(enc.Hex).slice(0, 2).toUpperCase();
}

export interface Decoded {
	ts: number; // sender's unix timestamp
	sender: string; // parsed "sender" prefix, or '' when absent
	text: string; // message body
}

/**
 * Decrypt a GroupText payload (channel_hash(1) | MAC(2) | ciphertext, all hex)
 * with the given 16-byte key (32 hex). Returns null when the HMAC doesn't match
 * (wrong key) or the plaintext isn't valid. Layout matches channel.go:
 * HMAC-SHA256 over the ciphertext keyed by the 32-byte secret (key||zeros),
 * AES-128-ECB (no padding), then timestamp(4 LE) | flags(1) | UTF-8 text.
 */
export function decryptGroupText(payloadHex: string, keyHex: string): Decoded | null {
	if (payloadHex.length < 6) return null; // need channel_hash + MAC
	const macHex = payloadHex.slice(2, 6).toLowerCase();
	const ctHex = payloadHex.slice(6);
	if (ctHex.length === 0 || (ctHex.length / 2) % CHANNEL_KEY_BYTES !== 0) return null;

	const keyWA = enc.Hex.parse(keyHex);
	const ctWA = enc.Hex.parse(ctHex);

	// HMAC-SHA256 over the ciphertext, keyed by the 32-byte secret (key||zeros).
	const secret = enc.Hex.parse(keyHex + '0'.repeat(CHANNEL_KEY_BYTES * 2));
	const mac = HmacSHA256(ctWA, secret).toString(enc.Hex).slice(0, 4).toLowerCase();
	if (mac !== macHex) return null;

	// AES-128-ECB, no padding — decrypt each block independently.
	const decrypted = AES.decrypt(lib.CipherParams.create({ ciphertext: ctWA }), keyWA, {
		mode: mode.ECB,
		padding: pad.NoPadding
	});
	const outHex = decrypted.toString(enc.Hex);
	if (outHex.length < 10) return null; // timestamp(4) + flags(1)
	const bytes = hexToBytes(outHex);

	const ts = (bytes[0] | (bytes[1] << 8) | (bytes[2] << 16) | (bytes[3] << 24)) >>> 0;
	let body = bytes.subarray(5);
	const nul = body.indexOf(0); // trim at null terminator
	if (nul >= 0) body = body.subarray(0, nul);

	let text: string;
	try {
		text = new TextDecoder('utf-8', { fatal: true }).decode(body);
	} catch {
		return null; // invalid UTF-8 → almost certainly the wrong key
	}
	const { sender, message } = splitSender(text);
	return { ts, sender, text: message };
}

/** Separate a "sender: message" prefix when present and plausible. */
function splitSender(s: string): { sender: string; message: string } {
	const i = s.indexOf(': ');
	if (i > 0 && i < 50 && !/[:[\]]/.test(s.slice(0, i))) {
		return { sender: s.slice(0, i), message: s.slice(i + 2) };
	}
	return { sender: '', message: s };
}
