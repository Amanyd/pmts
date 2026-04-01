<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { register } from '$lib/api';
	import { saveKey, getStoredKey } from '$lib/auth';

	let mode = $state<'login' | 'signup'>('login');
	let email = $state('');
	let apiKey = $state('');
	let error = $state('');
	let generatedKey = $state('');
	let loading = $state(false);

	onMount(() => {
		if (getStoredKey()) goto('/dashboard');
	});

	async function handleSignup() {
		error = '';
		if (!email.includes('@')) { error = 'Enter a valid email.'; return; }
		loading = true;
		try {
			const data = await register(email);
			generatedKey = data.api_key;
		} catch (e: any) {
			error = e.message ?? 'Signup failed.';
		} finally {
			loading = false;
		}
	}

	function handleLogin() {
		error = '';
		if (!apiKey.startsWith('sk_')) { error = 'Key must start with sk_'; return; }
		saveKey(apiKey);
		goto('/dashboard');
	}

	function useGeneratedKey() {
		saveKey(generatedKey);
		goto('/dashboard');
	}
</script>

<svelte:head>
	<title>DataCat — server monitoring for developers</title>
</svelte:head>

<div class="page" style="max-width: 560px;">

	{#if generatedKey}
		<div style="margin-top: 48px;">
			<div style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.1em; color: var(--muted); margin-bottom: 16px;">
				account created
			</div>
			<p style="margin-bottom: 16px;">Your API key. Save it — this is the only time we show it.</p>
			<pre style="margin-bottom: 24px; word-break: break-all;">{generatedKey}</pre>
			<button class="btn btn-fill" onclick={useGeneratedKey}>go to dashboard →</button>
		</div>

	{:else}

		<div style="margin-top: 48px; margin-bottom: 48px;">
			<h1 style="font-size: 28px; font-weight: 600; letter-spacing: 0.04em; margin-bottom: 12px;">
				DataCat
			</h1>
			<p style="color: var(--muted); margin-bottom: 8px;">
				Server and app metrics without the complexity.
			</p>
			<p style="color: var(--muted); font-size: 13px;">
				Drop one binary on your server. Metrics appear within seconds.
			</p>
		</div>

		<hr>

		<div style="display: flex; gap: 0; margin-bottom: 24px;">
			<button
				class="btn"
				style="border-right: none;"
				class:btn-fill={mode === 'login'}
				onclick={() => mode = 'login'}
			>login</button>
			<button
				class="btn"
				class:btn-fill={mode === 'signup'}
				onclick={() => mode = 'signup'}
			>sign up</button>
		</div>

		{#if mode === 'login'}
			<div class="form-row">
				<label for="api-key">API Key</label>
				<input id="api-key" type="text" bind:value={apiKey} placeholder="sk_..." />
			</div>
			<button class="btn btn-fill" onclick={handleLogin}>login →</button>

		{:else}
			<div class="form-row">
				<label for="email">Email</label>
				<input id="email" type="email" bind:value={email} placeholder="you@example.com" />
			</div>
			<button class="btn btn-fill" onclick={handleSignup} disabled={loading}>
				{loading ? 'creating...' : 'create account →'}
			</button>
		{/if}

		{#if error}
			<div class="notice err" style="margin-top: 16px;">{error}</div>
		{/if}
	{/if}

	<hr>

	<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 20px; margin-top: 8px;">
		<div>
			<div style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--muted); margin-bottom: 6px;">
				for servers
			</div>
			<p style="font-size: 13px; color: var(--muted);">
				One binary. Runs on any Linux server. Reports CPU, memory, disk, and network automatically.
			</p>
		</div>
		<div>
			<div style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--muted); margin-bottom: 6px;">
				custom metrics
			</div>
			<p style="font-size: 13px; color: var(--muted);">
				Expose a <code>/metrics</code> endpoint from your app. The agent scrapes it automatically.
			</p>
		</div>
	</div>
</div>
