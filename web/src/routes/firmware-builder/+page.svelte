<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import Seo from '$lib/components/Seo.svelte';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import { auth } from '$lib/auth.svelte';
	import {
		api,
		type FirmwareCatalogue,
		type FirmwareEnv,
		type FirmwareJob,
		type FirmwareArtifact,
		type FirmwareBuildSummary
	} from '$lib/api';

	// A build takes minutes, so this page is a job tracker, not a request/response
	// form: submit returns an id, and we poll until it settles.
	const POLL_MS = 4000;

	let cat = $state<FirmwareCatalogue | null>(null);
	let loadErr = $state('');
	let loading = $state(true);

	let tag = $state('');
	let board = $state('');
	let envName = $state('');
	let chosen = $state<Set<string>>(new Set());

	// What the current job was built from. The job row stores compiler flags, not
	// option ids, so remembering the selection here is what lets the card tell
	// "this is your selection" from "this was an earlier one".
	let builtFrom = $state<{ tag: string; env: string; options: string[] } | null>(null);
	let job = $state<FirmwareJob | null>(null);
	let artifacts = $state<FirmwareArtifact[]>([]);
	let ahead = $state(0);
	let cached = $state(false);
	let submitErr = $state('');
	let submitting = $state(false);
	let timer: ReturnType<typeof setTimeout> | null = null;
	let previous = $state<FirmwareBuildSummary[]>([]);

	const boards = $derived(cat?.boards ?? []);
	const envs = $derived<FirmwareEnv[]>(boards.find((b) => b.name === board)?.envs ?? []);
	const env = $derived<FirmwareEnv | undefined>(envs.find((e) => e.name === envName));
	// Only options this firmware can actually honour. An option offered where the
	// build ignores it is indistinguishable from the option being broken.
	const options = $derived((cat?.options ?? []).filter((o) => env?.options.includes(o.id)));
	const busy = $derived(job?.state === 'queued' || job?.state === 'building');
	const chosenLabels = $derived(
		(cat?.options ?? []).filter((o) => chosen.has(o.id)).map((o) => o.label)
	);
	const ready = $derived(Boolean(tag && envName));
	// True while the finished job still corresponds to what is selected. Once it
	// does not, the job is shown as a previous build rather than silently
	// disappearing — someone who just built something should not lose the
	// download link by ticking a box to compare.
	// "Already built" is for OTHER people's builds. The one you just ran is shown
	// in the panel above with its downloads, and listing it twice — the second
	// time labelled "matches your selection" — reads as two different things.
	const otherBuilds = $derived(previous.filter((b) => b.id !== job?.id));
	const jobMatches = $derived(
		!!builtFrom &&
			builtFrom.tag === tag &&
			builtFrom.env === envName &&
			builtFrom.options.length === chosen.size &&
			builtFrom.options.every((o) => chosen.has(o))
	);

	onMount(async () => {
		try {
			cat = await api.firmwareCatalogue();
			tag = cat.tags[0] ?? '';
		} catch (e) {
			loadErr = e instanceof Error ? e.message : 'could not load the firmware catalogue';
		} finally {
			loading = false;
		}
	});

	onDestroy(() => {
		if (timer) clearTimeout(timer);
	});

	// Changing board or firmware invalidates the option selection: the ids are
	// per-environment, and carrying them across would submit options the new
	// firmware rejects.
	function pickBoard(name: string) {
		board = name;
		envName = '';
		chosen = new Set();
	}
	function pickEnv(name: string) {
		envName = name;
		chosen = new Set();
		loadPrevious();
	}

	async function loadPrevious() {
		previous = [];
		if (!envName) return;
		try {
			previous = await api.firmwareBuilds(envName);
		} catch {
			// A failure here costs the convenience list, not the ability to build.
		}
	}

	// Does a previous build match exactly what is selected? Compared on option
	// SETS, not order, since order carries no meaning.
	function sameOptions(a: string[], b: Set<string>): boolean {
		return a.length === b.size && a.every((o) => b.has(o));
	}
	function toggle(id: string) {
		const next = new Set(chosen);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		chosen = next;
	}

	async function submit() {
		if (!tag || !envName || submitting) return;
		submitting = true;
		submitErr = '';
		artifacts = [];
		try {
			const res = await api.firmwareBuild(auth.csrf, {
				tag,
				env: envName,
				options: [...chosen]
			});
			job = res.job;
			builtFrom = { tag, env: envName, options: [...chosen] };
			cached = res.cached;
			if (job.state === 'done') {
				// A cache hit returns done immediately and never enters the poll loop,
				// so the list has to be refreshed here too.
				await refresh();
				await loadPrevious();
			} else {
				poll();
			}
		} catch (e) {
			submitErr = e instanceof Error ? e.message : 'could not queue the build';
		} finally {
			submitting = false;
		}
	}

	async function refresh() {
		if (!job) return;
		const res = await api.firmwareJob(job.id);
		job = res.job;
		artifacts = res.artifacts ?? [];
		ahead = res.ahead;
	}

	function poll() {
		if (timer) clearTimeout(timer);
		timer = setTimeout(async () => {
			try {
				await refresh();
			} catch {
				// A transient failure shouldn't abandon the job; the next tick retries.
			}
			if (job && (job.state === 'queued' || job.state === 'building')) poll();
		else loadPrevious();
		}, POLL_MS);
	}

	// MeshCore tags each firmware line separately — repeater-v1.17.1,
	// companion-v1.17.1, room-server-v1.17.1 — all off the same commit. The line
	// in the tag says nothing about what you are building (that is the firmware
	// choice below), so show the version alone and keep the full tag as the value
	// the API needs.
	function releaseLabel(t: string): string {
		return t.match(/\d+\.\d+\.\d+(?:\.\d+)?/)?.[0] ?? t;
	}

	function size(bytes: number): string {
		if (bytes < 1024) return `${bytes} B`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
		return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
	}

	// What each artifact is FOR. A list of filenames tells someone holding a board
	// nothing about which one to flash.
	const artifactHelp: Record<string, string> = {
		'firmware-merged.bin': 'ESP32: the single image to flash at offset 0x0',
		'firmware.zip': 'nRF52: serial DFU package (adafruit-nrfutil)',
		'firmware.uf2': 'nRF52: drag onto the board’s USB drive — the usual choice',
		'firmware.bin': 'ESP32: application only — needs the bootloader and partitions too',
		'firmware.hex': 'nRF52: raw image for a programmer — only if no .uf2 was produced',
		'bootloader.bin': 'ESP32 component, flash at the board’s bootloader offset',
		'partitions.bin': 'ESP32 component, flash at 0x8000'
	};
</script>

<Seo title="Firmware builder" description="Build MeshCore firmware for a chosen radio and options." />

<PageHeader eyebrow="Firmware" title="Build firmware">
	<p class="muted small">Choose a radio and options; the build runs on the server.</p>
</PageHeader>

{#if loading}
	<p class="muted">Loading the catalogue…</p>
{:else if loadErr}
	<p class="err">{loadErr}</p>
{:else if cat}
	<div class="grid">
		<section class="panel">
			<h2>Release</h2>
			<select bind:value={tag} aria-label="Release">
				{#each cat.tags as t (t)}
					<option value={t}>{releaseLabel(t)}</option>
				{/each}
			</select>

			<h2>Radio</h2>
			<select value={board} onchange={(e) => pickBoard(e.currentTarget.value)} aria-label="Radio">
				<option value="">Choose a radio…</option>
				{#each boards as b (b.name)}
					<option value={b.name}>{b.name}</option>
				{/each}
			</select>

			{#if board}
				<h2>Firmware</h2>
				<select value={envName} onchange={(e) => pickEnv(e.currentTarget.value)} aria-label="Firmware">
					<option value="">Choose a firmware…</option>
					{#each envs as e (e.name)}
						<option value={e.name}>{e.name}{e.isBridge ? '  (RS232 bridge)' : ''}</option>
					{/each}
				</select>
			{/if}

			{#if env}
				<h2>Options</h2>
				{#if options.length === 0}
					<p class="muted small">No build options apply to this firmware.</p>
				{:else}
					{#each options as o (o.id)}
						<label class="opt">
							<input type="checkbox" checked={chosen.has(o.id)} onchange={() => toggle(o.id)} />
							<span>
								<strong>{o.label}</strong>
								<span class="muted small">{o.help}</span>
							</span>
						</label>
					{/each}
				{/if}

				<button class="build" onclick={submit} disabled={submitting || busy}>
					{busy ? 'Building…' : 'Build firmware'}
				</button>
				{#if submitErr}<p class="err">{submitErr}</p>{/if}
			{/if}
		</section>

		<section class="panel">
			<h2>Selection</h2>
			{#if !ready}
				<p class="muted">
					Choose a release, a radio and a firmware. This panel shows what will be built
					before you commit to it.
				</p>
			{:else}
				<dl class="job">
					<dt>Release</dt>
					<dd>{releaseLabel(tag)}</dd>
					<dt>Radio</dt>
					<dd>{board}</dd>
					<dt>Firmware</dt>
					<dd>{envName}</dd>
					<dt>Options</dt>
					<dd>
						{#if chosenLabels.length === 0}
							<span class="muted">none</span>
						{:else}
							{chosenLabels.join(', ')}
						{/if}
					</dd>
				</dl>
				{#if env?.isBridge}
					<p class="muted small">
						This is an RS232 bridge build — a separate firmware, not an option that can be
						added to a plain repeater.
					</p>
				{/if}
			{/if}

			{#if job}
				<h2>{jobMatches ? 'Build' : 'Previous build'}</h2>
				{#if !jobMatches}
					<p class="muted small">
						Your selection has changed since this was built. It stays here so you do not
						lose the download.
					</p>
				{/if}
				<dl class="job">
					<dt>Firmware</dt>
					<dd>{job.env}</dd>
					<dt>Release</dt>
					<dd>{releaseLabel(job.tag)}</dd>
					<dt>Options</dt>
					<dd>{job.flags || 'none'}</dd>
					<dt>State</dt>
					<dd class="state {job.state}">{job.state}</dd>
				</dl>

				{#if cached && job.state === 'done'}
					<p class="muted small">
						This exact firmware had already been built, so it was not compiled again.
					</p>
				{/if}
				{#if job.state === 'queued'}
					<p class="muted small">
						{ahead > 0
							? `Waiting — ${ahead} build${ahead === 1 ? '' : 's'} ahead.`
							: 'Waiting to start…'}
					</p>
				{/if}
				{#if job.state === 'building'}
					<p class="muted small">Compiling. This usually takes a few minutes.</p>
				{/if}
				{#if job.state === 'failed'}
					<p class="err">{job.error || 'The build failed.'}</p>
				{/if}

				{#if job.state === 'done'}
					{#if artifacts.length === 0}
						<p class="muted small">No artifacts — they may have expired.</p>
					{:else}
						<ul class="files">
							{#each artifacts as a (a.name)}
								<li>
									<a href={api.firmwareDownloadUrl(job.id, a.name)} download>{a.downloadName}</a>
									<span class="muted small">{size(a.bytes)}</span>
									{#if artifactHelp[a.name]}
										<span class="muted small help">{artifactHelp[a.name]}</span>
									{/if}
								</li>
							{/each}
						</ul>
					{/if}
					{#if job.expiresAt}
						<p class="muted small">
							Downloads remain available until {job.expiresAt.slice(0, 10)}.
						</p>
					{/if}
				{/if}
			{/if}

			<!-- Hidden while a build is in flight: an empty "nothing has been built"
			     directly under a running compile reads as a contradiction, and the
			     list cannot include the in-flight job anyway. -->
			{#if envName && !busy}
				<h2>Already built</h2>
				{#if otherBuilds.length === 0}
					<p class="muted small">
						{job
							? 'No other builds of this firmware are available.'
							: 'Nothing has been built for this firmware yet, or earlier builds have expired.'}
					</p>
				{:else}
					<p class="muted small">
						Other builds of this firmware that are ready now. Taking one skips the compile
						entirely.
					</p>
					<ul class="prev">
						{#each otherBuilds as p (p.id)}
							{@const match = sameOptions(p.options, chosen) && p.tag === tag}
							<li class:match>
								<div class="prev-head">
									<span class="ver">{releaseLabel(p.tag)}</span>
									<span class="opts">
										{p.options.length
											? (cat?.options ?? [])
													.filter((o) => p.options.includes(o.id))
													.map((o) => o.label)
													.join(', ')
											: 'no options'}
									</span>
									{#if match}<span class="tick">matches your selection</span>{/if}
								</div>
								<div class="prev-files">
									{#each p.artifacts as a (a.name)}
										<a href={api.firmwareDownloadUrl(p.id, a.name)} download>
											{a.downloadName}<span class="muted small"> {size(a.bytes)}</span>
										</a>
									{/each}
								</div>
							</li>
						{/each}
					</ul>
				{/if}
			{/if}
		</section>
	</div>
{/if}

<style>
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
		gap: 1rem;
		align-items: start;
	}
	.panel {
		background: var(--color-panel);
		border: 1px solid var(--color-line);
		border-radius: var(--radius);
		padding: 1rem 1.15rem 1.25rem;
	}
	h2 {
		font-size: 0.78rem;
		letter-spacing: 0.12em;
		text-transform: uppercase;
		color: var(--color-fg-dim);
		margin: 1.1rem 0 0.5rem;
	}
	h2:first-of-type {
		margin-top: 0;
	}
	select {
		width: 100%;
		padding: 0.5rem 0.6rem;
		background: var(--color-ink-2);
		color: var(--color-fg);
		border: 1px solid var(--color-line-bright);
		border-radius: var(--radius);
		font: inherit;
	}
	.opt {
		display: flex;
		gap: 0.6rem;
		align-items: start;
		margin: 0 0 0.7rem;
		cursor: pointer;
	}
	.opt span {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
	}
	.build {
		margin-top: 1.2rem;
		width: 100%;
		padding: 0.6rem;
		background: var(--color-signal);
		color: var(--color-ink);
		border: 0;
		border-radius: var(--radius);
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}
	.build:disabled {
		opacity: 0.55;
		cursor: default;
	}
	.job {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 0.35rem 1rem;
		margin: 0 0 0.8rem;
	}
	.job dt {
		color: var(--color-fg-dim);
		font-size: 0.85rem;
	}
	.job dd {
		margin: 0;
		font-family: var(--font-mono);
		font-size: 0.85rem;
		word-break: break-word;
	}
	.state.done {
		color: var(--color-lime);
	}
	.state.failed {
		color: var(--color-coral);
	}
	.state.building,
	.state.queued {
		color: var(--color-amber);
	}
	.files {
		list-style: none;
		padding: 0;
		margin: 0.5rem 0 0;
	}
	.files li {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		align-items: baseline;
		padding: 0.45rem 0;
		border-bottom: 1px solid var(--color-line);
	}
	.files li:last-child {
		border-bottom: 0;
	}
	.help {
		flex-basis: 100%;
	}
	.prev {
		list-style: none;
		padding: 0;
		margin: 0.5rem 0 0;
	}
	.prev li {
		padding: 0.55rem 0.6rem;
		border: 1px solid var(--color-line);
		border-radius: var(--radius);
		margin-bottom: 0.5rem;
	}
	/* The build that matches the current selection is the one worth taking, so it
	   is marked rather than left for the reader to work out. */
	.prev li.match {
		border-color: var(--color-signal);
	}
	.prev-head {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		align-items: baseline;
		margin-bottom: 0.35rem;
	}
	.ver {
		font-family: var(--font-mono);
		font-weight: 600;
	}
	.opts {
		color: var(--color-fg-dim);
		font-size: 0.85rem;
	}
	.tick {
		color: var(--color-signal);
		font-size: 0.75rem;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}
	.prev-files {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
		font-size: 0.85rem;
	}
	.muted {
		color: var(--color-fg-dim);
	}
	.small {
		font-size: 0.85rem;
	}
	.err {
		color: var(--color-coral);
	}
</style>
