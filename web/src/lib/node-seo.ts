// A node page's search title and description. Kept word-for-word with the Go
// server's crawler view (internal/api/seo.go) so a crawler sees the same text
// whether or not it runs JavaScript.
import type { Node } from '$lib/api';

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

function roleWord(role: string | undefined): string {
	switch (role?.toLowerCase()) {
		case 'repeater':
			return 'repeater';
		case 'chatnode':
		case 'companion':
			return 'companion radio';
		case 'roomserver':
		case 'room':
		case 'room_server':
			return 'room server';
		case 'sensor':
			return 'sensor';
		default:
			return 'node';
	}
}

/** "2 Jan 2006" in UTC, as the server writes it. */
function day(ts: string | undefined): string {
	const d = ts ? new Date(ts) : null;
	if (!d || isNaN(d.getTime())) return '';
	return `${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]} ${d.getUTCFullYear()}`;
}

export function nodeSeo(node: Node | null, pubkey: string): { title: string; description: string } {
	const name = node?.name?.trim() || `Node ${pubkey.slice(0, 12).toUpperCase()}`;
	const role = roleWord(node?.role);
	let description = `${name} is a MeshCore ${role} on the mesh across Vancouver Island and the Lower Mainland, BC.`;
	const last = day(node?.lastSeen);
	if (last) description += ` Last heard ${last}.`;
	description += ' Adverts, relay activity, observers and coverage.';
	return { title: `${name} — MeshCore ${role}`, description };
}
