<script lang="ts">
	// Placeholder shapes shown while a page's data loads, in place of a bare
	// "Loading…" line: they hold the layout so content doesn't jump in.
	// `list` = rows with a dot + two lines; `detail` = title block + stat tiles
	// + rows. Screen readers get one "Loading" announcement.
	let { variant = 'list', rows = 6, class: cls = '' }: { variant?: 'list' | 'detail'; rows?: number; class?: string } =
		$props();
</script>

<div class="skel {cls}" role="status" aria-label="Loading">
	{#if variant === 'detail'}
		<div class="mb-5 flex items-center gap-3">
			<span class="sk h-10 w-10 rounded-full"></span>
			<div class="flex-1 space-y-2">
				<span class="sk block h-5 w-2/5"></span>
				<span class="sk block h-3 w-1/4"></span>
			</div>
		</div>
		<div class="mb-5 grid grid-cols-2 gap-3 md:grid-cols-4">
			{#each Array(4) as _, i (i)}<span class="sk block h-16"></span>{/each}
		</div>
	{/if}
	<div class="space-y-4">
		{#each Array(rows) as _, i (i)}
			<div class="flex items-center gap-3">
				<span class="sk h-2.5 w-2.5 shrink-0 rounded-full"></span>
				<div class="flex-1 space-y-1.5">
					<span class="sk block h-3.5" style="width:{55 - ((i * 17) % 30)}%"></span>
					<span class="sk block h-2.5" style="width:{30 - ((i * 11) % 15)}%"></span>
				</div>
			</div>
		{/each}
	</div>
</div>

<style>
	.sk {
		border-radius: 6px;
		background: linear-gradient(
				90deg,
				transparent 0%,
				color-mix(in srgb, var(--color-fg) 6%, transparent) 50%,
				transparent 100%
			)
			0 0 / 200% 100%,
			color-mix(in srgb, var(--color-fg) 8%, transparent);
		animation: shimmer 1.4s ease-in-out infinite;
	}
	@keyframes shimmer {
		from {
			background-position: 150% 0, 0 0;
		}
		to {
			background-position: -50% 0, 0 0;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.sk {
			animation: none;
		}
	}
</style>
