// Current-user session state, backed by the server's HttpOnly session cookie.
// The cookie is authoritative; this store mirrors it for the UI and holds the
// CSRF token that authenticated mutations must echo. Initialised once from
// /api/auth/me on app start (auth.init()).
import { authApi, type AuthUser } from './api';

class Auth {
	user = $state<AuthUser | null>(null);
	/** Session CSRF token, sent as X-CSRF-Token on authenticated mutations. */
	csrf = $state('');
	/** True once the initial /me probe has resolved (so UI can avoid flicker). */
	ready = $state(false);

	async init() {
		try {
			const r = await authApi.me();
			this.user = r.user;
			this.csrf = r.csrfToken ?? '';
		} catch {
			// Offline or server error — treat as signed out; UI stays usable.
		}
		this.ready = true;
	}

	async register(email: string, password: string, displayName: string) {
		const r = await authApi.register(email, password, displayName);
		this.#adopt(r);
	}

	async login(email: string, password: string) {
		const r = await authApi.login(email, password);
		this.#adopt(r);
	}

	async logout() {
		try {
			await authApi.logout();
		} finally {
			this.user = null;
			this.csrf = '';
		}
	}

	#adopt(r: { user: AuthUser | null; csrfToken?: string }) {
		this.user = r.user;
		this.csrf = r.csrfToken ?? '';
	}

	get loggedIn() {
		return this.user !== null;
	}
	get isAdmin() {
		return this.user?.isAdmin ?? false;
	}
	get canClaim() {
		return this.user?.canClaim ?? false;
	}
}

export const auth = new Auth();
