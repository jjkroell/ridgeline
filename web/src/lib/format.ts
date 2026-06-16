// Display helpers shared across the UI.

const ROLE_COLORS: Record<string, string> = {
	Repeater: 'var(--color-role-repeater)',
	ChatNode: 'var(--color-role-companion)',
	RoomServer: 'var(--color-role-room)',
	Sensor: 'var(--color-role-sensor)',
	Observer: 'var(--color-role-observer)'
};

const ROLE_LABELS: Record<string, string> = {
	Repeater: 'Repeater',
	ChatNode: 'Companion',
	RoomServer: 'Room',
	Sensor: 'Sensor',
	Observer: 'Observer'
};

export function roleColor(role: string): string {
	return ROLE_COLORS[role] ?? 'var(--color-fg-faint)';
}

export function roleLabel(role: string): string {
	return ROLE_LABELS[role] ?? role ?? 'Unknown';
}

/** Short form of a 64-hex public key: AB12…F9 */
export function shortKey(key: string, head = 4, tail = 2): string {
	if (!key) return '—';
	if (key.length <= head + tail) return key;
	return `${key.slice(0, head)}…${key.slice(-tail)}`;
}

/** Compact relative time, e.g. "12s", "4m", "3h", "2d". */
export function ago(iso?: string): string {
	if (!iso) return '—';
	const then = new Date(iso).getTime();
	if (Number.isNaN(then)) return '—';
	const s = Math.max(0, (Date.now() - then) / 1000);
	if (s < 60) return `${Math.floor(s)}s`;
	if (s < 3600) return `${Math.floor(s / 60)}m`;
	if (s < 86400) return `${Math.floor(s / 3600)}h`;
	return `${Math.floor(s / 86400)}d`;
}

/** Maps an SNR value (dB) to a color on the signal scale. */
export function snrColor(snr?: number): string {
	if (snr == null) return 'var(--color-fg-faint)';
	if (snr >= 5) return 'var(--color-lime)';
	if (snr >= -5) return 'var(--color-signal)';
	if (snr >= -12) return 'var(--color-amber)';
	return 'var(--color-coral)';
}

export function fmtSnr(snr?: number): string {
	return snr == null ? '—' : `${snr > 0 ? '+' : ''}${snr.toFixed(1)}`;
}

export function fmtNum(n: number): string {
	return n.toLocaleString('en-US');
}

export function fmtCoord(lat?: number, lon?: number): string {
	if (lat == null || lon == null) return '—';
	return `${lat.toFixed(4)}, ${lon.toFixed(4)}`;
}
