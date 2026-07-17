// Consent-gated analytics (this build: self-hosted Umami).
//
// Umami is only injected AFTER the visitor opts in via the cookie-consent banner
// (see consent.svelte.ts). Nothing is loaded or sent before that — which is what
// GDPR / the ePrivacy Directive require for non-essential analytics. Umami is
// cookieless and collects only anonymized usage (no personal data), but we still
// treat it as opt-in to be safe under GDPR and PIPEDA.
import { consent } from './consent.svelte';

/** Whether this build offers an analytics category in the consent banner. */
export const ANALYTICS_ENABLED = true;
/** Provider name shown in the banner + privacy policy. */
export const ANALYTICS_PROVIDER = 'Umami';

const SRC = 'https://umami.ve7kod.ca/script.js';
const WEBSITE_ID = '4c10287b-81fe-4ae9-9197-e285b2fd4919';

let injected = false;

/** True once the analytics script has actually been added to the page. */
export function analyticsInjected(): boolean {
	return injected;
}

/** Inject the Umami script once, if the visitor has consented to analytics. Safe
 * to call repeatedly (e.g. from a reactive effect on the consent state). */
export function syncAnalytics(): void {
	if (!ANALYTICS_ENABLED || injected || !consent.analytics) return;
	if (typeof document === 'undefined') return;
	const s = document.createElement('script');
	s.defer = true;
	s.src = SRC;
	s.setAttribute('data-website-id', WEBSITE_ID);
	document.head.appendChild(s);
	injected = true;
}

/** Report WebGL availability once per session as a custom Umami event (drives the
 * dashboard's WebGL-fallback share). No-op unless analytics is consented + loaded;
 * retries briefly because the Umami script loads asynchronously. */
export function trackWebGLOnce(enabled: boolean): void {
	if (!ANALYTICS_ENABLED || !consent.analytics) return;
	try {
		if (sessionStorage.getItem('rl-webgl-tracked')) return;
	} catch {
		return; // storage unavailable — skip rather than risk firing every load
	}
	let tries = 0;
	const fire = (): boolean => {
		const u = (window as unknown as { umami?: { track: (n: string, d?: unknown) => void } }).umami;
		if (!u?.track) return false;
		u.track('webgl', { enabled });
		try {
			sessionStorage.setItem('rl-webgl-tracked', '1');
		} catch {
			/* ignore */
		}
		return true;
	};
	if (fire()) return;
	const iv = setInterval(() => {
		if (fire() || ++tries > 40) clearInterval(iv); // give up after ~10s
	}, 250);
}
