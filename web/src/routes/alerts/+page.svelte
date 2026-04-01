<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getStoredKey } from '$lib/auth';
	import { getRules, createRule, deleteRule, getMetricNames } from '$lib/api';
	import type { AlertRule } from '$lib/api';

	let rules = $state<AlertRule[]>([]);
	let names = $state<string[]>([]);
	let metric = $state('');
	let threshold = $state('');
	let webhook = $state('');
	let status = $state('');
	let loading = $state(false);

	async function load() {
		try { rules = await getRules(); } catch { rules = []; }
		try {
			names = await getMetricNames();
			if (names.length && !metric) metric = names[0];
		} catch { /* names stays empty */ }
	}

	async function handleCreate() {
		if (!metric) { status = 'Pick a metric first.'; return; }
		const t = parseFloat(threshold);
		if (isNaN(t)) { status = 'Threshold must be a number.'; return; }
		status = 'saving...';
		loading = true;
		try {
			await createRule({ metric, threshold: t, webhook_url: webhook });
			status = 'rule saved.';
			threshold = '';
			webhook = '';
			await load();
		} catch (e: any) {
			status = e.message ?? 'failed.';
		} finally {
			loading = false;
		}
	}

	async function handleDelete(id: number) {
		if (!confirm('Delete this rule?')) return;
		await deleteRule(id);
		rules = rules.filter((r) => r.id !== id);
	}

	onMount(() => {
		if (!getStoredKey()) { goto('/'); return; }
		load();
	});
</script>

<svelte:head><title>Alerts — DataCat</title></svelte:head>

<div class="page">
	<div class="section-header">
		<h2>alert rules</h2>
		<span class="count">{rules.length}</span>
	</div>

	<!-- existing rules -->
	{#if rules.length === 0}
		<div class="empty" style="margin-bottom: 32px;">
			<h3>no rules yet</h3>
			<p>Create one below and DataCat will POST to your webhook when the threshold is crossed.</p>
		</div>
	{:else}
		<table style="margin-bottom: 32px;">
			<thead>
				<tr>
					<th>metric</th>
					<th>threshold</th>
					<th>webhook</th>
					<th></th>
				</tr>
			</thead>
			<tbody>
				{#each rules as rule (rule.id)}
					<tr>
						<td><code>{rule.metric}</code></td>
						<td>{rule.threshold}</td>
						<td style="color: var(--muted); font-size: 12px; max-width: 240px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
							{rule.webhook_url || '—'}
						</td>
						<td>
							<button
								onclick={() => handleDelete(rule.id)}
								style="font-family: var(--font); font-size: 12px; background: none; border: 1px solid var(--border-dim); color: var(--muted); padding: 3px 10px; cursor: pointer;"
							>delete</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}

	<!-- create form -->
	<div class="box">
		<div style="font-size: 12px; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 20px;">
			new rule
		</div>

		<div class="grid-2" style="margin-bottom: 16px;">
			<div class="form-row">
				<label for="rule-metric">metric</label>
				<select id="rule-metric" bind:value={metric}>
					{#each names as n}
						<option value={n}>{n.replace(/_/g, ' ')}</option>
					{/each}
				</select>
			</div>
			<div class="form-row">
				<label for="rule-threshold">alert when above</label>
				<input id="rule-threshold" type="number" step="any" bind:value={threshold} placeholder="90" />
			</div>
		</div>

		<div class="form-row">
			<label for="rule-webhook">webhook URL (optional)</label>
			<input
				id="rule-webhook"
				type="url"
				bind:value={webhook}
				placeholder="https://hooks.slack.com/..."
			/>
		</div>
		<div style="display: flex; align-items: center; gap: 16px; margin-top: 4px;">
			<button class="btn btn-fill" onclick={handleCreate} disabled={loading}>save rule</button>
			{#if status}
				<span style="font-size: 12px; color: var(--muted);">{status}</span>
			{/if}
		</div>
	</div>
</div>
