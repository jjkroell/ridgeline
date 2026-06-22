<script lang="ts">
	import { onMount } from 'svelte';
	import {
		admin,
		type InjectionReport,
		type BlockEntry,
		type BridgeCandidate,
		type InjectorCandidate
	} from '$lib/api';
	import { roleColor, roleLabel } from '$lib/format';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import WindowToggle from '$lib/components/WindowToggle.svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';

	const TOKEN_KEY = 'ridgeline-admin-token';

	let token = $state('');
	let authed = $state(false);
	let authError = $state('');
	let checking = $state(true);

	let windowSec = $state(86400);
	const windows = [
		{ label: '24h', sec: 86400 },
		{ label: '3d', sec: 259200 },
		{ label: '7d', sec: 604800 }
	];

	let report = $state<InjectionReport | null>(null);
	let blocks = $state<BlockEntry[]>([]);
	let detecting = $state(false);
	let busy = $state(''); // key currently acting on
	let msg = $state('');
	let expanded = $state<Record<string, boolean>>({});

	onMount(async () => {
		const saved = sessionStorage.getItem(TOKEN_KEY);
		if (saved) {
			token = saved;
			await tryAuth();
		}
		checking = false;
	});

	async function tryAuth() {
		authError = '';
		try {
			await admin.check(token);
			authed = true;
			sessionStorage.setItem(TOKEN_KEY, token);
			await refreshBlocks();
		} catch (e) {
			authed = false;
			authError = String((e as Error).message ?? e) === '503' ? 'Admin API is disabled (no admin token configured on the server).' : 'Invalid token.';
		}
	}

	function logout() {
		sessionStorage.removeItem(TOKEN_KEY);
		token = '';
		authed = false;
		report = null;
		blocks = [];
	}

	async function refreshBlocks() {
		try {
			blocks = await admin.blocklist(token);
		} catch (e) {
			msg = `blocklist: ${(e as Error).message}`;
		}
	}

	async function runDetect() {
		detecting = true;
		msg = '';
		try {
			report = await admin.detect(token, windowSec);
		} catch (e) {
			msg = `detect: ${(e as Error).message}`;
		} finally {
			detecting = false;
		}
	}

	const isBlocked = (kind: string, key: string) =>
		blocks.some((b) => b.kind === kind && b.key.toUpperCase() === key.toUpperCase());
	const isAllowed = (key: string) =>
		blocks.some((b) => b.kind === 'allow' && b.key.toUpperCase() === key.toUpperCase());

	// Bridge candidates minus any the admin has dismissed as known-good.
	const visibleBridges = $derived((report?.bridges ?? []).filter((b) => !isAllowed(b.nodeKey)));

	async function quarantineBridge(b: BridgeCandidate) {
		busy = b.nodeKey;
		msg = '';
		try {
			// Block the bridge AND its CAPTIVE foreign nodes (the ones with no
			// alternative route) so the injected set disappears from maps/lists.
			// Non-captive nodes are left alone — they reach the mesh other ways too.
			const captive = b.foreign.filter((f) => f.captive).map((f) => f.key);
			await admin.block(token, {
				kind: 'bridge',
				key: b.nodeKey,
				name: b.name,
				reason: 'RF bridge (detected)',
				nodes: captive
			});
			await refreshBlocks();
			msg = `Quarantined ${b.name} + ${captive.length} captive nodes — hidden from maps/feed and dropped at ingest.`;
		} catch (e) {
			msg = `quarantine: ${(e as Error).message}`;
		} finally {
			busy = '';
		}
	}

	async function dismissBridge(b: BridgeCandidate) {
		busy = b.nodeKey;
		msg = '';
		try {
			await admin.block(token, { kind: 'allow', key: b.nodeKey, name: b.name, reason: 'dismissed — not a bridge' });
			await refreshBlocks();
			msg = `Dismissed ${b.name} — it won't be flagged as a candidate again.`;
		} catch (e) {
			msg = `dismiss: ${(e as Error).message}`;
		} finally {
			busy = '';
		}
	}

	async function purgeBridge(b: BridgeCandidate) {
		const captive = b.foreign.filter((f) => f.captive).map((f) => f.key);
		if (!confirm(`Permanently delete ${b.name} and its ${captive.length} captive nodes plus all their stored packets? This cannot be undone.`))
			return;
		busy = b.nodeKey;
		msg = '';
		try {
			const res = await admin.purge(token, { bridges: [b.nodeKey], nodes: captive });
			await refreshBlocks();
			report = null;
			msg = `Purged ${b.name}: deleted ${res.observations} observations and ${res.nodes} node rows.`;
		} catch (e) {
			msg = `purge: ${(e as Error).message}`;
		} finally {
			busy = '';
		}
	}

	async function quarantineInjector(i: InjectorCandidate) {
		busy = i.observer;
		msg = '';
		try {
			await admin.block(token, { kind: 'observer', key: i.observer, name: i.observer, reason: 'MQTT injector (detected)' });
			await refreshBlocks();
			msg = `Quarantined observer ${i.observer} — its published packets will now be dropped.`;
		} catch (e) {
			msg = `quarantine: ${(e as Error).message}`;
		} finally {
			busy = '';
		}
	}

	async function purgeInjector(i: InjectorCandidate) {
		if (!confirm(`Permanently delete all packets published by ${i.observer} and its ${i.exclusiveCount} exclusively-sourced nodes? This cannot be undone.`))
			return;
		busy = i.observer;
		msg = '';
		try {
			const res = await admin.purge(token, { observers: [i.observer], nodes: i.exclusive.map((f) => f.key) });
			await refreshBlocks();
			report = null;
			msg = `Purged ${i.observer}: deleted ${res.observations} observations and ${res.nodes} node rows.`;
		} catch (e) {
			msg = `purge: ${(e as Error).message}`;
		} finally {
			busy = '';
		}
	}

	async function removeBlock(b: BlockEntry) {
		busy = b.kind + b.key;
		msg = '';
		try {
			await admin.unblock(token, b.kind, b.key);
			await refreshBlocks();
		} catch (e) {
			msg = `unblock: ${(e as Error).message}`;
		} finally {
			busy = '';
		}
	}

	const kindColor: Record<string, string> = {
		bridge: 'var(--color-coral)',
		observer: 'var(--color-amber)',
		node: 'var(--color-fg-dim)',
		allow: 'var(--color-signal)'
	};
	const kindLabel = (k: string) => (k === 'allow' ? 'dismissed' : k);
</script>

<PageHeader eyebrow="Restricted" title="Admin — Injection Control">
	{#if authed}
		<button onclick={logout} class="label hover:text-coral transition-colors">Lock</button>
	{/if}
</PageHeader>

<div class="px-6 py-6 md:px-10">
	{#if checking}
		<div class="panel text-fg-faint px-5 py-12 text-center text-sm">Checking…</div>
	{:else if !authed}
		<!-- Auth gate -->
		<div class="panel mx-auto mt-10 max-w-md px-6 py-8">
			<h2 class="font-display text-fg mb-1 text-base font-700">Admin access</h2>
			<p class="text-fg-faint mb-4 text-sm">Enter the admin token to manage injection detection and quarantine.</p>
			<form
				onsubmit={(e) => {
					e.preventDefault();
					tryAuth();
				}}
			>
				<input
					type="password"
					bind:value={token}
					placeholder="admin token"
					class="border-line bg-ink-2 text-fg focus:border-signal w-full rounded-[var(--radius)] border px-3 py-2 text-sm outline-none"
				/>
				{#if authError}
					<p class="text-coral mt-2 text-xs">{authError}</p>
				{/if}
				<button
					type="submit"
					class="bg-signal/15 text-signal border-signal/40 hover:bg-signal/25 mt-4 w-full rounded-[var(--radius)] border px-4 py-2 text-sm font-600 transition-colors"
				>
					Unlock
				</button>
			</form>
		</div>
	{:else}
		<!-- Controls -->
		<div class="flex flex-wrap items-center gap-3">
			<button
				onclick={runDetect}
				disabled={detecting}
				class="bg-signal/15 text-signal border-signal/40 hover:bg-signal/25 rounded-[var(--radius)] border px-4 py-2 text-sm font-600 transition-colors disabled:opacity-50"
			>
				{detecting ? 'Scanning…' : 'Run detection'}
			</button>
			<WindowToggle options={windows} bind:value={windowSec} />
			<span class="text-fg-faint text-xs">Scans adverts for foreign-traffic ingress points.</span>
		</div>

		{#if msg}
			<div class="panel border-signal/40 text-fg-dim mt-4 px-4 py-2.5 text-sm">{msg}</div>
		{/if}

		<!-- Detection results -->
		{#if report}
			<!-- RF bridges -->
			<section class="panel rise mt-6">
				<div class="border-line/70 flex items-center gap-2.5 border-b px-5 py-3.5">
					<h2 class="font-display text-fg text-sm font-700 tracking-wide">RF BRIDGE CANDIDATES</h2>
					<span class="label normal-case text-fg-faint">nodes funnelling never-heard-direct traffic in</span>
					<span class="label ml-auto tnum">{visibleBridges.length}</span>
				</div>
				{#if visibleBridges.length === 0}
					<div class="text-fg-faint px-5 py-8 text-center text-sm">No RF bridge signature detected in this window.</div>
				{:else}
					<div class="divide-line/40 divide-y">
						{#each visibleBridges as b (b.nodeKey)}
							<div class="px-5 py-3">
								<div class="flex flex-wrap items-center gap-3">
									<span class="h-2 w-2 shrink-0 rounded-full" style="background:var(--color-coral)"></span>
									<a href="/nodes/{b.nodeKey}" class="text-fg hover:text-signal font-600">{b.name}</a>
									<span class="font-mono text-fg-faint text-[0.62rem]">{b.nodeKey.slice(0, 12)}…</span>
									<Tooltip text="foreign nodes with no alternative route — ≥95% of their traffic transits this node">
										<span class="label normal-case tnum text-coral">{b.captiveCount}/{b.foreignThrough} captive</span>
									</Tooltip>
									<span class="label normal-case tnum text-fg-faint">{(b.captiveFraction * 100).toFixed(0)}% of foreign</span>
									{#if b.foreignKm > 5}
										<Tooltip text="distance of the captive cluster from the mesh — a hint only, not used for ranking">
											<span class="label normal-case tnum text-fg-faint">{b.foreignKm.toFixed(0)} km</span>
										</Tooltip>
									{/if}
									<div class="ml-auto flex items-center gap-2">
										<button onclick={() => (expanded[b.nodeKey] = !expanded[b.nodeKey])} class="label hover:text-signal">
											{expanded[b.nodeKey] ? 'hide' : 'show'} nodes
										</button>
										{#if isBlocked('bridge', b.nodeKey)}
											<span class="label text-amber">quarantined</span>
										{:else}
											<button
												onclick={() => dismissBridge(b)}
												disabled={busy === b.nodeKey}
												class="border-line/60 text-fg-faint hover:text-fg hover:bg-line/30 rounded-[var(--radius)] border px-3 py-1 text-xs font-600 transition-colors disabled:opacity-50"
												title="Not a bridge — stop flagging this node"
											>Dismiss</button>
											<button
												onclick={() => quarantineBridge(b)}
												disabled={busy === b.nodeKey}
												class="border-amber/40 text-amber hover:bg-amber/15 rounded-[var(--radius)] border px-3 py-1 text-xs font-600 transition-colors disabled:opacity-50"
											>Quarantine</button>
										{/if}
										<button
											onclick={() => purgeBridge(b)}
											disabled={busy === b.nodeKey}
											class="border-coral/40 text-coral hover:bg-coral/15 rounded-[var(--radius)] border px-3 py-1 text-xs font-600 transition-colors disabled:opacity-50"
										>Purge</button>
									</div>
								</div>
								{#if expanded[b.nodeKey]}
									<div class="mt-2 flex flex-wrap gap-1.5 pl-5">
										{#each b.foreign as f (f.key)}
											<span
												class="flex items-center gap-1.5 rounded-[var(--radius)] border px-2 py-0.5 text-[0.68rem] {f.captive ? 'border-coral/40 text-fg-dim' : 'border-line/40 text-fg-faint'}"
											>
												<span class="h-1.5 w-1.5 rounded-full" style="background:{roleColor(f.role ?? '')}"></span>
												{f.name}
												<span class="tnum {f.captive ? 'text-coral' : 'text-fg-faint'}">{(f.transitPct ?? 0).toFixed(0)}%</span>
											</span>
										{/each}
									</div>
									<p class="text-fg-faint mt-1.5 pl-5 text-[0.62rem]">
										% = share of that node's traffic transiting this candidate; <span class="text-coral">coral</span> = captive (≥95%, no alternative route). Quarantine/Purge act on captive nodes only.
									</p>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			</section>

			<!-- MQTT injectors -->
			<section class="panel rise mt-6">
				<div class="border-line/70 flex items-center gap-2.5 border-b px-5 py-3.5">
					<h2 class="font-display text-fg text-sm font-700 tracking-wide">MQTT INJECTOR CANDIDATES</h2>
					<span class="label normal-case text-fg-faint">observers that are the sole source of nodes</span>
					<span class="label ml-auto tnum">{report.injectors?.length ?? 0}</span>
				</div>
				{#if (report.injectors?.length ?? 0) === 0}
					<div class="text-fg-faint px-5 py-8 text-center text-sm">No rogue MQTT publisher detected in this window.</div>
				{:else}
					<div class="divide-line/40 divide-y">
						{#each report.injectors as i (i.observer)}
							<div class="px-5 py-3">
								<div class="flex flex-wrap items-center gap-3">
									<span class="h-2 w-2 shrink-0 rounded-full" style="background:var(--color-amber)"></span>
									<span class="text-fg font-600">{i.observer}</span>
									<span class="label normal-case tnum text-amber">{i.exclusiveCount} exclusive nodes</span>
									<div class="ml-auto flex items-center gap-2">
										<button onclick={() => (expanded[i.observer] = !expanded[i.observer])} class="label hover:text-signal">
											{expanded[i.observer] ? 'hide' : 'show'} nodes
										</button>
										{#if isBlocked('observer', i.observer)}
											<span class="label text-amber">quarantined</span>
										{:else}
											<button
												onclick={() => quarantineInjector(i)}
												disabled={busy === i.observer}
												class="border-amber/40 text-amber hover:bg-amber/15 rounded-[var(--radius)] border px-3 py-1 text-xs font-600 transition-colors disabled:opacity-50"
											>Quarantine</button>
										{/if}
										<button
											onclick={() => purgeInjector(i)}
											disabled={busy === i.observer}
											class="border-coral/40 text-coral hover:bg-coral/15 rounded-[var(--radius)] border px-3 py-1 text-xs font-600 transition-colors disabled:opacity-50"
										>Purge</button>
									</div>
								</div>
								{#if expanded[i.observer]}
									<div class="mt-2 flex flex-wrap gap-1.5 pl-5">
										{#each i.exclusive as f (f.key)}
											<span class="border-line/60 text-fg-dim rounded-[var(--radius)] border px-2 py-0.5 text-[0.68rem]">{f.name}</span>
										{/each}
									</div>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			</section>
		{/if}

		<!-- Current blocklist -->
		<section class="panel rise mt-6">
			<div class="border-line/70 flex items-center gap-2.5 border-b px-5 py-3.5">
				<h2 class="font-display text-fg text-sm font-700 tracking-wide">QUARANTINE LIST</h2>
				<span class="label normal-case text-fg-faint">blocked = dropped at ingest + hidden · dismissed = excluded from detection</span>
				<span class="label ml-auto tnum">{blocks.length}</span>
			</div>
			{#if blocks.length === 0}
				<div class="text-fg-faint px-5 py-8 text-center text-sm">Nothing quarantined or dismissed.</div>
			{:else}
				<div class="divide-line/40 divide-y">
					{#each blocks as b (b.kind + b.key)}
						<div class="flex items-center gap-3 px-5 py-2.5 text-sm">
							<span class="label !text-[0.58rem]" style="color:{kindColor[b.kind] ?? 'var(--color-fg-dim)'}">{kindLabel(b.kind)}</span>
							<span class="text-fg min-w-0 flex-1 truncate">{b.name || b.key}</span>
							{#if b.reason}<span class="text-fg-faint text-xs">{b.reason}</span>{/if}
							<button
								onclick={() => removeBlock(b)}
								disabled={busy === b.kind + b.key}
								class="label hover:text-signal disabled:opacity-50"
							>release</button>
						</div>
					{/each}
				</div>
			{/if}
		</section>
	{/if}
</div>
