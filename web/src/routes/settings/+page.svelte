<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getStoredKey, clearKey } from '$lib/auth';

	let key = $state('');
	let copied = $state(false);

	onMount(() => {
		const k = getStoredKey();
		if (!k) { goto('/'); return; }
		key = k;
	});

	async function copyKey() {
		await navigator.clipboard.writeText(key);
		copied = true;
		setTimeout(() => copied = false, 2000);
	}

	function logout() {
		clearKey();
		goto('/');
	}
</script>

<svelte:head><title>Settings — DataCat</title></svelte:head>

<div class="page" style="max-width: 560px;">
	<div class="section-header" style="margin-bottom: 32px;">
		<h2>settings</h2>
	</div>

	<div class="box" style="margin-bottom: 24px;">
		<div style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--muted); margin-bottom: 12px;">
			your API key
		</div>
		<pre style="margin-bottom: 16px; word-break: break-all;">{key}</pre>
		<div style="display: flex; gap: 8px;">
			<button class="btn" onclick={copyKey}>
				{copied ? 'copied!' : 'copy'}
			</button>
		</div>
		<p style="font-size: 12px; color: var(--muted); margin-top: 16px;">
			This key authenticates all requests. Keep it out of client-side code.
		</p>
	</div>

	<hr>

	<div>
		<div style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--muted); margin-bottom: 12px;">
			session
		</div>
		<button class="btn" onclick={logout}>log out</button>
		<p style="font-size: 12px; color: var(--muted); margin-top: 12px;">
			Your key stays in localStorage. Logging out removes it from this browser only.
		</p>
	</div>
</div>
