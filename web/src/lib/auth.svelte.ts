// Current-user session state, backed by the server's HttpOnly session cookie.
// The cookie is authoritative; this store mirrors it for the UI and holds the
// CSRF token that authenticated mutations must echo. Initialised once from
// /api/auth/me on app start (auth.init()).
import { authApi, shares, type AuthUser } from './api';

class Auth {
	user = $state<AuthUser | null>(null);
	/** Session CSRF token, sent as X-CSRF-Token on authenticated mutations. */
	csrf = $state('');
	/** True once the initial /me probe has resolved (so UI can avoid flicker). */
	ready = $state(false);
	/** Nodes newly shared with the user, not yet seen — drives the account badge. */
	unseenShares = $state(0);

	async init() {
		try {
			const r = await authApi.me();
			this.user = r.user;
			this.csrf = r.csrfToken ?? '';
			this.unseenShares = r.unseenShares ?? 0;
		} catch {
			// Offline or server error — treat as signed out; UI stays usable.
		}
		this.ready = true;
	}

	/** Clear the "new shares" badge once the user has seen their list. */
	async markSharesSeen() {
		if (this.unseenShares === 0 || !this.csrf) return;
		try {
			await shares.markSeen(this.csrf);
			this.unseenShares = 0;
		} catch {
			/* leave the badge; it clears on next successful attempt */
		}
	}

	/** Register. Returns the response so the caller can show a "check your email"
	 *  screen when verification was sent (user is null in that case). */
	async register(email: string, password: string, displayName: string) {
		const r = await authApi.register(email, password, displayName);
		if (r.user) this.#adopt(r);
		return r;
	}

	async login(email: string, password: string) {
		const r = await authApi.login(email, password);
		this.#adopt(r);
	}

	/** Confirm an emailed verification token; logs the user in on success. */
	async verifyEmail(token: string) {
		const r = await authApi.verifyEmail(token);
		this.#adopt(r);
		return r;
	}

	async logout() {
		try {
			await authApi.logout();
		} finally {
			this.user = null;
			this.csrf = '';
			this.unseenShares = 0;
		}
	}

	#adopt(r: { user: AuthUser | null; csrfToken?: string; unseenShares?: number }) {
		this.user = r.user;
		this.csrf = r.csrfToken ?? '';
		this.unseenShares = r.unseenShares ?? 0;
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
