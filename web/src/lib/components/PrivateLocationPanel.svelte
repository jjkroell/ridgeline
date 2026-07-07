<!--
  Owner-only editor for a node's PRIVATE exact location. Renders nothing unless
  the signed-in user is the node's verified owner. The pin the owner drops here is
  never shown on the public map or in any public API — it's stored separately and
  read back only by the owner (and, in a later phase, users they share with).
  Collapsible + minimized by default, matching the notes panel.
-->
<script lang="ts">
	import { onDestroy } from 'svelte';
	import { claims, privateLocation, type PrivateLocation } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import { theme } from '$lib/theme.svelte';
	import { isLight } from '$lib/map-util';

	interface Props {
		pubkey: string;
		/** The node's publicly advertised coords, used only to seed the map centre. */
		seedLat?: number | null;
		seedLon?: number | null;
		compact?: boolean;
	}
	let { pubkey, seedLat = null, seedLon = null, compact = false }: Props = $props();

	// Fallback centre if we have neither a saved nor an advertised location — the
	// Salish Sea / BC coast, where this mesh lives.
	const FALLBACK: [number, number] = [49.16, -123.94];

	let owner = $state(false); // caller owns this node
	let ready = $state(false); // finished the ownership probe
	let expanded = $state(false);

	let saved = $state<PrivateLocation | null>(null);
	let lat = $state<number | null>(null);
	let lon = $state<number | null>(null);
	let label = $state('');
	let busy = $state(false);
	let error = $state('');
	let justSaved = $state(false);

	let loadedFor = $state('');
	$effect(() => {
		if (pubkey && pubkey !== loadedFor) {
			loadedFor = pubkey;
			probe();
		}
	});

	async function probe() {
		owner = false;
		ready = false;
		try {
			const st = await claims.status(pubkey);
			owner = st.ownedByMe;
			if (owner) await loadLocation();
		} catch {
			owner = false;
		} finally {
			ready = true;
		}
	}

	async function loadLocation() {
		try {
			const res = await privateLocation.get(pubkey);
			if (res.set && res.location) {
				saved = res.location;
				lat = res.location.latitude;
				lon = res.location.longitude;
				label = res.location.label;
			} else {
				saved = null;
				lat = seedLat ?? null;
				lon = seedLon ?? null;
				label = '';
			}
		} catch (e) {
			error = String((e as Error).message ?? e);
		}
	}

	async function save() {
		if (lat == null || lon == null) {
			error = 'Drop a pin or enter coordinates first.';
			return;
		}
		busy = true;
		error = '';
		try {
			const res = await privateLocation.set(auth.csrf, pubkey, lat, lon, label.trim());
			saved = res.location ?? null;
			justSaved = true;
			setTimeout(() => (justSaved = false), 2500);
		} catch (e) {
			error = String((e as Error).message ?? e);
		} finally {
			busy = false;
		}
	}

	async function clear() {
		busy = true;
		error = '';
		try {
			await privateLocation.remove(auth.csrf, pubkey);
			saved = null;
			lat = seedLat ?? null;
			lon = seedLon ?? null;
			label = '';
			if (map && marker) {
				map.removeLayer(marker);
				marker = null;
			}
		} catch (e) {
			error = String((e as Error).message ?? e);
		} finally {
			busy = false;
		}
	}

	// ---- Interactive Leaflet map (lazy; only mounted while expanded) ----
	let mapEl: HTMLDivElement | undefined = $state();
	/* eslint-disable @typescript-eslint/no-explicit-any */
	let L: any = null;
	let map: any = null;
	let marker: any = null;
	let tiles: any = null;
	let pinIcon: any = null;
	/* eslint-enable @typescript-eslint/no-explicit-any */
	let curLight = false;

	// A themed teal map pin as an inline divIcon — avoids Leaflet's default PNG
	// marker (whose asset path doesn't resolve under the bundler) and matches the
	// site's accent colour.
	const pinSvg =
		'<svg width="28" height="28" viewBox="0 0 24 24" fill="#34e3c4" stroke="#0b1f1a" stroke-width="1.3"><path d="M12 2c-3.87 0-7 3.13-7 7 0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7z"/><circle cx="12" cy="9" r="2.4" fill="#0b1f1a" stroke="none"/></svg>';

	// Selectable raster base layers. Only "map" follows the UI light/dark theme;
	// the rest are fixed imagery. All are free, key-less, Leaflet-friendly tiles.
	interface LayerOpt {
		id: string;
		label: string;
		desc: string;
		themed?: boolean;
	}
	const LAYERS: LayerOpt[] = [
		{ id: 'map', label: 'Map', desc: 'Clean, theme-aware base', themed: true },
		{ id: 'satellite', label: 'Satellite', desc: 'Aerial imagery' },
		{ id: 'street', label: 'Street', desc: 'Detailed streets & places' },
		{ id: 'topo', label: 'Topographic', desc: 'Contours & relief' }
	];
	let layerId = $state('map');
	let layerOpen = $state(false);
	const currentLayer = $derived(LAYERS.find((l) => l.id === layerId) ?? LAYERS[0]);

	const cartoUrl = (light: boolean) =>
		`https://{s}.basemaps.cartocdn.com/${light ? 'light_all' : 'dark_all'}/{z}/{x}/{y}{r}.png`;
	const cartoAttr =
		'© <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap</a> © <a href="https://carto.com/attributions" target="_blank" rel="noopener">CARTO</a>';

	// Build the Leaflet tile layer for a base-layer id at the current theme.
	function makeTiles(id: string, light: boolean) {
		switch (id) {
			case 'satellite':
				return L.tileLayer(
					'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
					{
						maxZoom: 19,
						attribution:
							'Imagery © <a href="https://www.esri.com" target="_blank" rel="noopener">Esri</a>, Maxar, Earthstar Geographics'
					}
				);
			case 'street':
				return L.tileLayer('https://{s}.basemaps.cartocdn.com/rastertiles/voyager/{z}/{x}/{y}{r}.png', {
					subdomains: 'abcd',
					maxZoom: 19,
					attribution: cartoAttr
				});
			case 'topo':
				return L.tileLayer('https://{s}.tile.opentopomap.org/{z}/{x}/{y}.png', {
					subdomains: 'abc',
					maxZoom: 17,
					attribution:
						'© <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap</a> contributors, SRTM | © <a href="https://opentopomap.org" target="_blank" rel="noopener">OpenTopoMap</a> (CC-BY-SA)'
				});
			case 'map':
			default:
				return L.tileLayer(cartoUrl(light), { subdomains: 'abcd', maxZoom: 19, attribution: cartoAttr });
		}
	}

	// Swap the active base layer, preserving the marker + view.
	function selectLayer(id: string) {
		layerId = id;
		layerOpen = false;
		if (!map || !L) return;
		if (tiles) map.removeLayer(tiles);
		tiles = makeTiles(id, curLight).addTo(map);
	}

	// Initialise (or tear down) the map as the panel expands/collapses.
	$effect(() => {
		if (expanded && owner && mapEl && !map) {
			initMap(mapEl);
		}
		if (!expanded && map) {
			map.remove();
			map = null;
			marker = null;
			tiles = null;
		}
	});

	async function initMap(el: HTMLDivElement) {
		await import('leaflet/dist/leaflet.css');
		L = (await import('leaflet')).default ?? (await import('leaflet'));
		if (!el || map) return;
		curLight = isLight();
		pinIcon = L.divIcon({
			className: 'rl-private-pin',
			html: pinSvg,
			iconSize: [28, 28],
			iconAnchor: [14, 26]
		});
		const center: [number, number] = [lat ?? FALLBACK[0], lon ?? FALLBACK[1]];
		map = L.map(el, { center, zoom: lat != null ? 14 : 9, attributionControl: true });
		tiles = makeTiles(layerId, curLight).addTo(map);
		if (lat != null && lon != null) placeMarker(lat, lon);
		// Click anywhere to drop / move the pin.
		map.on('click', (e: { latlng: { lat: number; lng: number } }) => {
			setCoords(e.latlng.lat, e.latlng.lng);
		});
		setTimeout(() => map?.invalidateSize(), 80);
	}

	function placeMarker(la: number, lo: number) {
		if (!L || !map) return;
		if (marker) {
			marker.setLatLng([la, lo]);
			return;
		}
		marker = L.marker([la, lo], { draggable: true, icon: pinIcon }).addTo(map);
		marker.on('dragend', () => {
			const p = marker.getLatLng();
			setCoords(p.lat, p.lng);
		});
	}

	// Round to ~1m precision so the inputs stay tidy.
	function setCoords(la: number, lo: number) {
		lat = Math.round(la * 1e5) / 1e5;
		lon = Math.round(lo * 1e5) / 1e5;
		placeMarker(lat, lon);
	}

	// Keep the marker in sync when the number inputs change.
	function onInput() {
		if (lat != null && lon != null && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180) {
			placeMarker(lat, lon);
			map?.panTo([lat, lon]);
		}
	}

	// Swap tile theme with the UI — only the themed "map" layer changes URL.
	$effect(() => {
		void theme.mode;
		const light = isLight();
		if (!map || !tiles || light === curLight) return;
		curLight = light;
		if (currentLayer.themed) tiles.setUrl(cartoUrl(light));
	});

	onDestroy(() => {
		map?.remove();
		map = null;
	});
</script>

{#if ready && owner}
	<div class="panel {compact ? 'px-4 py-3.5' : 'px-5 py-4'}">
		<!-- Collapsed header -->
		<button
			type="button"
			onclick={() => (expanded = !expanded)}
			class="flex w-full items-center gap-2 text-left"
		>
			<svg
				viewBox="0 0 24 24"
				class="text-fg-faint h-4 w-4 shrink-0"
				fill="none"
				stroke="currentColor"
				stroke-width="1.6"
				stroke-linecap="round"
				stroke-linejoin="round"
				><rect x="5" y="11" width="14" height="9" rx="1.5" /><path d="M8 11V8a4 4 0 0 1 8 0v3" /></svg
			>
			<span class="label normal-case text-fg-dim shrink-0">Private location</span>
			<span
				class="shrink-0 rounded-full px-2 py-0.5 text-[0.62rem] font-600 {saved
					? 'bg-signal/15 text-signal'
					: 'bg-line/60 text-fg-dim'}">{saved ? 'Set' : 'Not set'}</span
			>
			{#if !expanded}
				<span class="text-fg-faint min-w-0 flex-1 truncate text-xs">
					{#if saved}{saved.latitude.toFixed(5)}, {saved.longitude.toFixed(5)}{#if saved.label}
							· {saved.label}{/if}{:else}Owner-only — drop an exact pin{/if}
				</span>
			{:else}
				<span class="flex-1"></span>
			{/if}
			<svg
				viewBox="0 0 24 24"
				class="text-fg-faint h-4 w-4 shrink-0 transition-transform {expanded ? 'rotate-180' : ''}"
				fill="none"
				stroke="currentColor"
				stroke-width="1.8"
				stroke-linecap="round"
				stroke-linejoin="round"><path d="M6 9l6 6 6-6" /></svg
			>
		</button>

		{#if expanded}
			<div class="mt-4">
				<p class="text-fg-faint mb-3 text-xs leading-relaxed">
					This exact location is <span class="text-fg-dim font-600">private</span> — only you can see
					it. It never appears on the public map or in any public data. Drag the pin (or click the map)
					to set your node's true position.
				</p>

				<div class="border-line/70 relative mb-3 h-64 overflow-hidden rounded-[var(--radius)] border">
					<div bind:this={mapEl} class="h-full w-full"></div>

					<!-- Base-layer selector (top-right, above the Leaflet panes) -->
					<div class="absolute top-2 right-2 z-[1000]">
						<button
							type="button"
							onclick={() => (layerOpen = !layerOpen)}
							class="border-line bg-ink-2/85 hover:bg-panel-2/70 flex items-center gap-1.5 rounded-[var(--radius)] border px-2.5 py-1.5 backdrop-blur-md transition-colors"
							aria-label="Base map: {currentLayer.label}"
						>
							<svg
								viewBox="0 0 24 24"
								class="text-fg-dim h-3.5 w-3.5 shrink-0"
								fill="none"
								stroke="currentColor"
								stroke-width="1.7"
								stroke-linecap="round"
								stroke-linejoin="round"><path d="M12 2 2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" /></svg
							>
							<span class="text-fg text-xs font-600">{currentLayer.label}</span>
							<svg
								viewBox="0 0 24 24"
								class="text-fg-faint h-3 w-3 shrink-0 transition-transform {layerOpen ? 'rotate-180' : ''}"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"><path d="M6 9l6 6 6-6" /></svg
							>
						</button>
						{#if layerOpen}
							<div
								class="border-line bg-ink-2/95 mt-1 w-44 overflow-hidden rounded-[var(--radius)] border backdrop-blur-md"
							>
								{#each LAYERS as l (l.id)}
									{@const on = l.id === layerId}
									<button
										type="button"
										onclick={() => selectLayer(l.id)}
										class="hover:bg-panel-2/60 flex w-full items-center gap-2 px-3 py-2 text-left transition-colors {on
											? 'bg-signal/10'
											: ''}"
									>
										<span
											class="h-1.5 w-1.5 shrink-0 rounded-full"
											style="background:{on ? 'var(--color-signal)' : 'var(--color-fg-faint)'}"
										></span>
										<span class="min-w-0 flex-1">
											<span class="block truncate text-xs font-600 {on ? 'text-signal' : 'text-fg'}"
												>{l.label}</span
											>
											<span class="text-fg-faint mt-0.5 block truncate text-[0.62rem] leading-tight"
												>{l.desc}</span
											>
										</span>
									</button>
								{/each}
							</div>
						{/if}
					</div>
				</div>

				<div class="mb-3 flex flex-wrap items-end gap-3">
					<label class="flex flex-col gap-1">
						<span class="text-fg-faint text-[0.68rem]">Latitude</span>
						<input
							type="number"
							step="0.00001"
							bind:value={lat}
							oninput={onInput}
							class="bg-ink-2 text-fg focus:border-signal w-32 rounded-[var(--radius)] border border-transparent px-2.5 py-1.5 text-sm outline-none"
						/>
					</label>
					<label class="flex flex-col gap-1">
						<span class="text-fg-faint text-[0.68rem]">Longitude</span>
						<input
							type="number"
							step="0.00001"
							bind:value={lon}
							oninput={onInput}
							class="bg-ink-2 text-fg focus:border-signal w-32 rounded-[var(--radius)] border border-transparent px-2.5 py-1.5 text-sm outline-none"
						/>
					</label>
					<label class="flex flex-1 flex-col gap-1">
						<span class="text-fg-faint text-[0.68rem]">Label (optional)</span>
						<input
							type="text"
							bind:value={label}
							maxlength="120"
							placeholder="rooftop, repeater site…"
							class="bg-ink-2 text-fg focus:border-signal min-w-32 rounded-[var(--radius)] border border-transparent px-2.5 py-1.5 text-sm outline-none"
						/>
					</label>
				</div>

				{#if error}<p class="text-coral mb-2 text-xs">{error}</p>{/if}

				<div class="flex items-center gap-2">
					<button
						onclick={save}
						disabled={busy || lat == null || lon == null}
						class="bg-signal/15 text-signal border-signal/40 hover:bg-signal/25 rounded-[var(--radius)] border px-3.5 py-1.5 text-sm font-600 transition-colors disabled:opacity-40"
						>{busy ? 'Saving…' : saved ? 'Update location' : 'Save location'}</button
					>
					{#if saved}
						<button
							onclick={clear}
							disabled={busy}
							class="text-fg-faint hover:text-coral rounded-[var(--radius)] px-3 py-1.5 text-sm transition-colors"
							>Clear</button
						>
					{/if}
					{#if justSaved}<span class="text-signal text-xs font-600">Saved ✓</span>{/if}
				</div>
			</div>
		{/if}
	</div>
{/if}
