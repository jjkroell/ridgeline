<script lang="ts">
	// Admin → Survey: the #mayday survey's answers (/survey), as tallies per
	// question and the written answers, newest first. Each answer carries a
	// short code for the connection it came from (a salted IP hash, never the
	// IP): codes with several answers are flagged, and "One per connection"
	// counts only each code's latest answer. Shared by /admin and
	// /m/admin like SeasonsPanel; `compact` switches to the mobile card chrome.
	import { onMount } from 'svelte';
	import { adminSurvey, type SurveyResponse } from '$lib/api';
	import { USED, MONITOR, FEATURES, SURVEY_ID } from '$lib/survey';

	let { compact = false }: { compact?: boolean } = $props();
	const shell = $derived(
		compact ? 'border-line/60 bg-panel mb-3 rounded-2xl border overflow-hidden' : 'panel rise mt-6 overflow-hidden'
	);
	const pad = $derived(compact ? 'px-4' : 'px-5');

	let rs = $state<SurveyResponse[]>([]);
	let loaded = $state(false);
	let error = $state('');
	let showText = $state(false);
	let onePer = $state(false);

	/** How many answers each connection code sent. */
	const perGroup = $derived(
		rs.reduce<Record<string, number>>((m, r) => {
			if (r.group) m[r.group] = (m[r.group] ?? 0) + 1;
			return m;
		}, {})
	);
	const repeats = $derived(
		Object.entries(perGroup)
			.filter(([, n]) => n > 1)
			.sort((a, b) => b[1] - a[1])
	);
	// rs is newest first, so the first answer seen per code is its latest.
	const counted = $derived(
		onePer ? rs.filter((r, i) => !r.group || rs.findIndex((x) => x.group === r.group) === i) : rs
	);

	async function load() {
		try {
			rs = (await adminSurvey.list(SURVEY_ID)).responses;
			error = '';
		} catch (e) {
			error = String((e as Error).message ?? e);
		} finally {
			loaded = true;
		}
	}
	onMount(load);

	type Opt = { id: string; label: string };
	const count = (pick: (r: SurveyResponse) => string[] | string | undefined, id: string) =>
		counted.filter((r) => {
			const v = pick(r);
			return Array.isArray(v) ? v.includes(id) : v === id;
		}).length;
	const groups: { title: string; opts: Opt[]; pick: (r: SurveyResponse) => string[] | string | undefined }[] = [
		{ title: 'How they used #sos', opts: USED, pick: (r) => r.answers.used },
		{ title: 'Monitor an emergency channel again?', opts: MONITOR, pick: (r) => r.answers.monitor },
		{ title: 'Features wanted', opts: FEATURES, pick: (r) => r.answers.features }
	];
	// Written-in channel names (question 3), case-folded and counted, most first.
	const names = $derived(
		Object.entries(
			counted.reduce<Record<string, number>>((m, r) => {
				const n = r.answers.nameOther?.trim().toLowerCase().replace(/^#?/, '#');
				if (n && n !== '#') m[n] = (m[n] ?? 0) + 1;
				return m;
			}, {})
		).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
	);
	const label = (opts: Opt[], ids?: string[] | string) =>
		(Array.isArray(ids) ? ids : ids ? [ids] : []).map((id) => opts.find((o) => o.id === id)?.label ?? id).join(', ');
	const when = (iso: string) =>
		new Date(iso).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
</script>

<section class={shell}>
	<div class="border-line/70 flex items-start gap-3 border-b {pad} {compact ? 'py-3' : 'py-3.5'}">
		<div class="min-w-0 flex-1">
			<h2 class="font-display text-fg font-700 tracking-wide {compact ? 'text-xs' : 'text-sm'}">EMERGENCY CHANNEL SURVEY</h2>
			<p class="text-fg-dim mt-1 text-xs">
				<span class="text-fg tnum font-600">{rs.length}</span>
				{rs.length === 1 ? 'response' : 'responses'} from
				<span class="text-fg tnum font-600">{Object.keys(perGroup).length}</span>
				{Object.keys(perGroup).length === 1 ? 'connection' : 'connections'} ·
				<a href="/survey" class="text-signal hover:underline">/survey</a>
			</p>
		</div>
		<button onclick={load} class="text-fg-dim hover:text-fg text-xs">Refresh</button>
	</div>
	{#if error}
		<p class="text-coral {pad} py-3 text-xs">{error}</p>
	{:else if loaded && rs.length === 0}
		<p class="text-fg-faint {pad} py-3 text-xs">No answers yet.</p>
	{:else if rs.length}
		{#if repeats.length}
			<div class="border-line/60 text-xs {pad} border-b py-2.5">
				<span class="text-amber font-600">More than one answer from:</span>
				{#each repeats as [g, n] (g)}
					<span class="bg-amber/10 text-amber ml-1.5 inline-block rounded px-1.5 py-0.5 font-mono">{g} ×{n}</span>
				{/each}
				<span class="text-fg-faint block pt-1">
					Same code = same connection: one person, or several people sharing a network (a household, a mobile carrier).
				</span>
			</div>
		{/if}
		<label class="text-fg-dim flex cursor-pointer items-center gap-2 text-xs {pad} pt-3">
			<input type="checkbox" bind:checked={onePer} class="accent-coral h-3.5 w-3.5" />
			One per connection (each code’s latest answer: {counted.length})
		</label>
		<div class="grid gap-x-6 gap-y-4 {pad} py-3 {compact ? '' : 'md:grid-cols-2'}">
			{#each groups as g (g.title)}
				<div>
					<div class="text-fg mb-1.5 text-xs font-600">{g.title}</div>
					{#each g.opts as o (o.id)}
						{@const n = count(g.pick, o.id)}
						<div class="flex items-center gap-2 py-0.5 text-xs">
							<span class="text-fg-dim min-w-0 flex-1 truncate">{o.label}</span>
							<span class="bg-panel-2 relative h-1.5 w-16 shrink-0 overflow-hidden rounded-full">
								<span class="bg-coral absolute inset-y-0 left-0" style="width:{counted.length ? (n / counted.length) * 100 : 0}%"></span>
							</span>
							<span class="text-fg tnum w-6 shrink-0 text-right">{n}</span>
						</div>
					{/each}
				</div>
			{/each}
			<div>
				<div class="text-fg mb-1.5 text-xs font-600">Suggested channel names</div>
				{#if names.length}
					{#each names as [n, c] (n)}
						<div class="flex items-center gap-2 py-0.5 text-xs">
							<span class="text-fg-dim min-w-0 flex-1 truncate font-mono">{n}</span>
							<span class="text-fg tnum w-6 shrink-0 text-right">{c}</span>
						</div>
					{/each}
				{:else}
					<p class="text-fg-faint text-xs">None written in yet.</p>
				{/if}
			</div>
		</div>
		<div class="border-line/60 border-t {pad} py-2.5">
			<button onclick={() => (showText = !showText)} class="text-signal text-xs hover:underline">
				{showText ? 'Hide' : 'Show'} answers ({rs.length})
			</button>
		</div>
		{#if showText}
			<ul class="divide-line/60 border-line/60 divide-y border-t">
				{#each rs as r (r.id)}
					<li class="{pad} space-y-1 py-2.5 text-xs">
						<div class="text-fg-faint">
							#{r.id} · {when(r.createdAt)}{#if r.group}
								· <span
									class="rounded px-1 font-mono {(perGroup[r.group] ?? 0) > 1 ? 'bg-amber/10 text-amber' : ''}"
									title="Connection code (salted IP hash)">{r.group}</span
								>{/if}{r.userId ? ' · signed in' : ''}{#if r.answers.email}
								· <a href="mailto:{r.answers.email}" class="text-signal hover:underline">{r.answers.email}</a>{/if}
						</div>
						{#if r.answers.used?.length}<div class="text-fg-dim">Used: <span class="text-fg">{label(USED, r.answers.used)}</span></div>{/if}
						{#if r.answers.monitor}<div class="text-fg-dim">Monitor again: <span class="text-fg">{label(MONITOR, r.answers.monitor)}</span></div>{/if}
						{#if r.answers.nameOther}<div class="text-fg-dim">Name: <span class="text-fg font-mono">{r.answers.nameOther}</span></div>{/if}
						{#if r.answers.features?.length}<div class="text-fg-dim">Wants: <span class="text-fg">{label(FEATURES, r.answers.features)}</span></div>{/if}
						{#if r.answers.concerns}<div class="text-fg-dim whitespace-pre-wrap">Concerns: <span class="text-fg">{r.answers.concerns}</span></div>{/if}
						{#if r.answers.other}<div class="text-fg-dim whitespace-pre-wrap">Else: <span class="text-fg">{r.answers.other}</span></div>{/if}
					</li>
				{/each}
			</ul>
		{/if}
	{/if}
</section>
