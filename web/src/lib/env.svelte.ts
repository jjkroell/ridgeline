// Instance role, read once from /api/health. The server reports `environment`
// (e.g. "dev") only when its config sets it — production and self-hosted
// instances leave it unset, so isDev is false there and no banner shows. This
// signal is config-driven, not build-driven, so the same web bundle is safe to
// ship everywhere; only a box whose config opts in ever flags itself.
class Env {
	/** Raw environment string from the server; '' means a normal/production box. */
	environment = $state('');
	/** The mesh's single radio preset ("freq,bw,sf,cr"), or '' when the server
	 *  has none or several configured. Shown site-wide by MeshRadio. */
	meshRadio = $state('');
	/** The admin's seasonal-theme switch: null = none set (follow each theme's
	 *  dates), 'none' = all off, else the one theme id turned on. */
	season = $state<string | null>(null);
	/** /api/health has answered (or failed): `season` is final for this load. */
	loaded = $state(false);
	#started = false;

	async init() {
		if (this.#started) return;
		this.#started = true;
		try {
			const r = await fetch('/api/health');
			if (r.ok) {
				const j = await r.json();
				this.environment = typeof j.environment === 'string' ? j.environment : '';
				this.meshRadio = typeof j.meshRadio === 'string' ? j.meshRadio : '';
				this.season = typeof j.season === 'string' && j.season ? j.season : null;
			}
		} catch {
			// Health unreachable — assume a normal instance; never show the banner
			// on a transient error.
		}
		this.loaded = true;
	}

	/** A non-production instance that should carry a visible banner. */
	get isDev(): boolean {
		return this.environment === 'dev' || this.environment === 'staging';
	}

	/** Short uppercase label for the banner, e.g. "DEV". */
	get label(): string {
		return this.environment.toUpperCase();
	}
}

export const env = new Env();
