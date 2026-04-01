<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getStoredKey } from '$lib/auth';
	import { getMetricNames, getMetrics, deleteMetric } from '$lib/api';
	import type { Sample } from '$lib/api';
	import LineChart from '$lib/components/Chart.svelte';

	let names = $state<string[]>([]);
	let selected = $state('');
	let chartData = $state<Sample[]>([]);
	let lastValue = $derived(chartData.at(-1)?.v ?? null);
	let range = $state<'1h' | '6h' | '24h' | '7d'>('1h');
	let chartType = $state<'line' | 'scatter'>('line');
	let loading = $state(false);
	let deleting = $state(false);

	const rangeSeconds: Record<typeof range, number> = {
		'1h':  3600,
		'6h':  21600,
		'24h': 86400,
		'7d':  604800
	};
	// ... rest of script is handled by allowMultiple: false, but to be sure I'll just match the target perfectly.

	async function loadChart() {
		if (!selected) return;
		loading = true;
		const now = Math.floor(Date.now() / 1000);
		try {
			const data = await getMetrics({
				name: selected,
				from: now - rangeSeconds[range],
				to: now
			});
			chartData = data[0]?.samples ?? [];
		} finally {
			loading = false;
		}
	}

	async function doDelete() {
		if (!selected) return;
		if (!confirm(`Are you sure you want to delete all data for "${selected}"? This cannot be undone.`)) return;
		deleting = true;
		try {
			await deleteMetric(selected);
			names = await getMetricNames();
			if (names.length > 0) {
				selected = names[0];
			} else {
				selected = '';
				chartData = [];
			}
		} catch (e) {
			alert('Failed to delete metric: ' + e);
		} finally {
			deleting = false;
		}
	}

	$effect(() => { selected; range; loadChart(); });

	onMount(async () => {
		if (!getStoredKey()) { goto('/'); return; }
		names = await getMetricNames();
		if (names.length) selected = names[0];
	});
</script>

<svelte:head><title>Metrics — DataCat</title></svelte:head>

<div class="page">
	<div class="section-header" style="margin-bottom: 24px;">
		<h2>metrics explorer</h2>
	</div>

	<div style="display: flex; gap: 16px; margin-bottom: 24px; flex-wrap: wrap;">
		<!-- metric selector -->
		<div style="flex: 1; min-width: 200px;">
			<div class="form-row">
				<label for="metric-select">metric</label>
				<div style="display: flex; gap: 8px;">
					<select id="metric-select" bind:value={selected} style="flex: 1;">
						{#if names.length === 0}
							<option>no metrics yet</option>
						{/if}
						{#each names as n}
							<option value={n}>{n.replace(/_/g, ' ')}</option>
						{/each}
					</select>
					{#if selected}
						<button class="btn" style="color: #ff4444; border-color: #ff4444; width: auto;" onclick={doDelete} disabled={deleting}>
							{deleting ? 'deleting...' : 'delete'}
						</button>
					{/if}
				</div>
			</div>
		</div>

		<!-- time range -->
		<div>
			<div style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--muted); margin-bottom: 6px;">
				range
			</div>
			<div style="display: flex; gap: 0;">
				{#each (['1h', '6h', '24h', '7d'] as const) as r}
					<button
						class="btn"
						style="border-right: {r !== '7d' ? 'none' : ''};"
						class:btn-fill={range === r}
						onclick={() => range = r}
					>{r}</button>
				{/each}
			</div>
		</div>
	</div>

	{#if selected}
		<div class="box" style="margin-bottom: 24px;">
			<div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px;">
				<div style="display: flex; align-items: baseline; gap: 16px;">
					<span style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--muted);">
						{selected.replace(/_/g, ' ')}
					</span>
					<span style="font-size: 24px; font-weight: 600;">
						{lastValue !== null ? lastValue.toFixed(4) : '—'}
					</span>
					{#if loading}<span style="font-size: 12px; color: var(--muted);">loading...</span>{/if}
				</div>
				
				<div style="display: flex; gap: 0;">
					<button class="btn" style="border-right: none;" class:btn-fill={chartType === 'line'} onclick={() => chartType = 'line'}>line</button>
					<button class="btn" class:btn-fill={chartType === 'scatter'} onclick={() => chartType = 'scatter'}>scatter</button>
				</div>
			</div>
			<LineChart data={chartData} label={selected} height={300} type={chartType} />
		</div>

		<!-- raw samples table -->
		{#if chartData.length > 0}
			<div class="section-header">
				<h2>raw samples <span style="font-size: 11px; text-transform: lowercase; color: var(--muted); margin-left: 8px;">(latest of {chartData.length})</span></h2>
			</div>
			<table>
				<thead>
					<tr>
						<th>time</th>
						<th>value</th>
					</tr>
				</thead>
				<tbody>
					{#each chartData.slice(-50).reverse() as s}
						<tr>
							<td>{new Date(s.t * 1000).toLocaleString()}</td>
							<td>{s.v.toFixed(6)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	{:else}
		<div class="empty">
			<h3>no metrics yet</h3>
			<p>Run the agent to start collecting data.</p>
		</div>
	{/if}
</div>
