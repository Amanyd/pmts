<script lang="ts">
	import '../app.css';
	import { page } from '$app/stores';
	import { getStoredKey, clearKey } from '$lib/auth';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let { children } = $props();

	let loggedIn = $state(false);
	let currentPath = $state('');

	onMount(() => {
		loggedIn = !!getStoredKey();
		currentPath = window.location.pathname;

		const unsub = page.subscribe((p) => {
			currentPath = p.url.pathname;
			loggedIn = !!getStoredKey();
		});
		return unsub;
	});

	function logout() {
		clearKey();
		loggedIn = false;
		goto('/');
	}

	function isActive(path: string) {
		return currentPath === path || currentPath.startsWith(path + '/');
	}
</script>

<nav class="nav">
	<a href="/" class="nav-brand">datacat</a>

	{#if loggedIn}
		<div class="nav-links">
			<a href="/dashboard" class:active={isActive('/dashboard')}>dashboard</a>
			<a href="/metrics" class:active={isActive('/metrics')}>metrics</a>
			<a href="/alerts" class:active={isActive('/alerts')}>alerts</a>
			<a href="/docs" class:active={isActive('/docs')}>docs</a>
			<a href="/settings" class:active={isActive('/settings')}>settings</a>
		</div>
		<button class="btn" onclick={logout}>logout</button>
	{/if}
</nav>

{@render children()}
