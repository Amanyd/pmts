<script lang="ts">
	import { onMount } from "svelte";


	let cpu =$state(0);
	let loading = $state(true);
	let error = $state("");

	async function getMetrics(){

		try{

			const res = await fetch('/api/metrics');
			if(!res.ok) throw new Error("Server is offline...");

			const data = await res.json();
			const metric = data.find((m: any)=> m.name==='cpu_usage_demo');

			if(metric && metric.samples.length>0){
				const lastSample= metric.samples[metric.samples.length-1];
				cpu = lastSample.v;
			}
			loading = false;

		} catch(e){

			error = "Failed to connect to api...";
			loading = false;

		}

	}

	onMount(()=>{
		getMetrics();
		const interval = setInterval(getMetrics, 2000);
		return () => clearInterval(interval);
	});

</script>


<main class="p-10 font-sans bg-black min-h-screen text-white">
	<h1 class="text-3xl font-bold mb-5">Datadog??</h1>

	{#if loading}
		<p>loading data...</p>

	{:else if error}
		<p>{error}</p>

	{:else}
		<div class="border p-5 rounded w-64">

			<h2>CPU Usage</h2>
			<p class={`text-4xl font-bold mt-2 ${cpu > 80 ? 'text-red-600': 'text-blue-600'}`}>{cpu.toFixed(1)}%</p>
			<p class="text-xs mt-2">Live from backend</p>

		</div>
	{/if}

	<button
		onclick={getMetrics}
		class="bg-white text-black mt-5 px-4 py-2 rounder hover:bg-neutral-500"
	>
		Refresh
	</button>

</main>