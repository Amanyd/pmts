<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getStoredKey } from '$lib/auth';
	import { getMetricNames, getMetrics } from '$lib/api';
	import type { Metric } from '$lib/api';
	import LineChart from '$lib/components/Chart.svelte';

	let loggedIn = $state(false);
	let names = $state<string[]>([]);
	let selected = $state('');
	let chartData = $state<{ t: number; v: number }[]>([]);
	let allMetrics = $state<Metric[]>([]);
	let connected = $state(false);
	let error = $state('');

	// cards: last known value per metric
	let cards = $derived(
		names.slice(0, 8).map((n) => {
			const m = allMetrics.find((x) => x.name === n);
			const last = m?.samples.at(-1);
			return { name: n, value: last?.v ?? null };
		})
	);

	async function load() {
		try {
			names = await getMetricNames();
			if (names.length && !selected) selected = names[0];
			allMetrics = await getMetrics();
			connected = true;
			error = '';
		} catch (e: any) {
			connected = false;
			error = e.message === '401' ? 'Invalid key.' : 'Cannot reach backend.';
		}
	}

	// Refresh chart data when selected metric changes
	$effect(() => {
		if (!selected) return;
		const now = Math.floor(Date.now() / 1000);
		getMetrics({ name: selected, from: now - 3600 })
			.then((data) => {
				chartData = data[0]?.samples ?? [];
			})
			.catch(() => {});
	});

	onMount(() => {
		const key = getStoredKey();
		if (!key) { goto('/'); return; }
		loggedIn = true;
		load();
		const iv = setInterval(load, 5000);
		return () => clearInterval(iv);
	});
</script>

<svelte:head><title>Dashboard — DataCat</title></svelte:head>

<div class="page-wide">

	<div style="display: flex; align-items: center; gap: 16px; margin-bottom: 28px;">
		<h1 style="font-size: 14px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.1em;">
			dashboard
		</h1>
		<span class="dot" class:off={!connected}></span>
		<span style="font-size: 12px; color: var(--muted);">{connected ? 'live' : 'disconnected'}</span>
		{#if error}<span style="font-size: 12px; color: var(--muted);">{error}</span>{/if}
	</div>

	{#if names.length === 0}
		<div class="empty">
			<h3>no data yet</h3>
			<p>Run the agent to start seeing metrics. Check the <a href="/docs">docs</a> for setup.</p>
		</div>
	{:else}

		<!-- metric cards -->
		<div class="grid-4" style="margin-bottom: 32px;">
			{#each cards as card (card.name)}
				<button
					class="metric-card"
					style="text-align: left; background: none; cursor: pointer; border: {selected === card.name ? '2px solid var(--fg)' : '1px solid var(--border)'};"
					onclick={() => selected = card.name}
				>
					<div class="label">{card.name.replace(/_/g, ' ')}</div>
					<div class="value">
						{card.value !== null ? card.value.toFixed(2) : '—'}
					</div>
				</button>
			{/each}
		</div>

		<!-- chart -->
		{#if selected}
			<div class="box" style="margin-bottom: 32px;">
				<div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px;">
					<div style="font-size: 12px; text-transform: uppercase; letter-spacing: 0.08em;">
						{selected.replace(/_/g, ' ')}
					</div>
					<div style="font-size: 11px; color: var(--muted);">last 1h</div>
				</div>
				<LineChart data={chartData} label={selected} height={260} />
			</div>
		{/if}
	{/if}
</div>
