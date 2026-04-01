<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getStoredKey } from '$lib/auth';

	let key = $state('');
	let host = $state('localhost');
	let tab = $state<'agent' | 'sdk' | 'http'>('agent');

	onMount(() => {
		host = window.location.hostname;
		const k = getStoredKey();
		if (!k) { goto('/'); return; }
		key = k;
	});
</script>

<svelte:head><title>Docs — DataCat</title></svelte:head>

<div class="page" style="max-width: 720px;">
	<div class="section-header" style="margin-bottom: 32px;">
		<h2>integration guide</h2>
	</div>

	<!-- Tab bar -->
	<div style="display: flex; gap: 0; margin-bottom: 32px; border-bottom: 1px solid var(--border);">
		{#each ([['agent', 'server agent'], ['sdk', 'node.js sdk'], ['http', 'http api']] as const) as [id, label]}
			<button
				onclick={() => tab = id}
				style="font-family: var(--font); font-size: 12px; text-transform: uppercase;
					letter-spacing: 0.08em; padding: 8px 20px; background: none; border: none;
					border-bottom: 2px solid {tab === id ? 'var(--fg)' : 'transparent'};
					color: {tab === id ? 'var(--fg)' : 'var(--muted)'}; cursor: pointer; margin-bottom: -1px;"
			>{label}</button>
		{/each}
	</div>

	<!-- Option 1: Server Agent -->
	{#if tab === 'agent'}
		<p style="color: var(--muted); margin-bottom: 16px;">
			For any Linux or macOS server. Runs in the background, reports system metrics every 5 seconds.
		</p>

		<p style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--muted); margin-bottom: 4px;">run via docker</p>
		<pre style="margin-bottom: 20px;">docker run -d --name datacat-agent amanyd139/datacat-agent:latest --key={key} --ingest=http://{host}:8080/api/ingest</pre>

		<p style="color: var(--muted); font-size: 13px; margin-bottom: 8px;">What gets reported automatically:</p>
		<ul style="color: var(--muted); font-size: 13px; margin-left: 20px; margin-bottom: 24px; line-height: 2;">
			<li><code>system_cpu_percent</code> — overall CPU load</li>
			<li><code>system_mem_percent</code> — RAM usage</li>
			<li><code>system_disk_percent</code> — disk usage</li>
			<li><code>system_load_1</code> / <code>_5</code> / <code>_15</code> — load averages</li>
			<li><code>system_net_sent_bytes</code> / <code>system_net_recv_bytes</code></li>
		</ul>

		<hr>

		<h3 style="font-size: 13px; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 20px; margin-top: 24px;">
			scraping custom metrics
		</h3>
		<p style="color: var(--muted); margin-bottom: 16px;">
			Expose a <code>/metrics</code> endpoint in your app that prints <code>name value</code> lines.
			Bind it to <code>127.0.0.1</code> so it's not publicly accessible.
		</p>

		<p style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--muted); margin-bottom: 4px;">in your Node.js app</p>
		<pre style="margin-bottom: 16px;">{`app.get('/metrics', (req, res) => {
  res.send(\`
active_users \${getActiveUsers()}
jobs_processed \${getJobCount()}
\`);
});`}</pre>

		<p style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--muted); margin-bottom: 4px;">start agent with --network host to reach localhost</p>
		<pre style="margin-bottom: 12px;">docker run -d --name datacat-agent --network host \
  amanyd139/datacat-agent:latest \
  --key={key} --scrape=http://localhost:3000/metrics</pre>
		<p style="color: var(--muted); font-size: 12px; margin-bottom: 24px;">Note: If the <code>/metrics</code> server goes down or returns 404, the agent perfectly ignores it and continues sending your hardware stats!</p>

	<!-- Option 2: Node.js SDK -->
	{:else if tab === 'sdk'}
		<p style="color: var(--muted); margin-bottom: 24px;">
			For Next.js, Vercel, Cloudflare Workers, or any Node.js app where you can't run a background agent.
			Metrics are batched in memory and flushed automatically — no per-request HTTP overhead.
		</p>

		<p style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--muted); margin-bottom: 4px;">install</p>
		<pre style="margin-bottom: 24px;">npm install @datacat/node</pre>

		<hr>
		<h3 style="font-size: 13px; text-transform: uppercase; letter-spacing: 0.08em; margin: 24px 0 16px;">basic usage</h3>
		<p style="color: var(--muted); font-size: 13px; margin-bottom: 8px;">Works in Express, Fastify, or any long-running Node.js server. Auto-flushes every 5s.</p>
		<pre style="margin-bottom: 24px;">{`import { DataCat } from '@datacat/node';

const dc = new DataCat({ apiKey: '${key}' });

// Track cumulative totals for line charts that go UP (Counters)
let totalSignups = 100;
totalSignups += 1;
dc.track('total_signups', totalSignups);

// Track absolute point-in-time values (Gauges)
dc.track('api_latency_ms', 230, { route: '/users' });

// Flush on shutdown
process.on('SIGTERM', () => dc.shutdown());`}</pre>

		<hr>
		<h3 style="font-size: 13px; text-transform: uppercase; letter-spacing: 0.08em; margin: 24px 0 16px;">express — auto-instrument all routes</h3>
		<pre style="margin-bottom: 24px;">{`import express from 'express';
import { DataCat } from '@datacat/node';
import { expressMiddleware } from '@datacat/node/express';

const dc = new DataCat({ apiKey: '${key}' });
const app = express();

// auto-tracks http_requests_total + http_request_duration_ms
app.use(expressMiddleware(dc));

app.get('/api/orders', (req, res) => res.json({ orders: [] }));

process.on('SIGTERM', () => dc.shutdown());`}</pre>

		<hr>
		<h3 style="font-size: 13px; text-transform: uppercase; letter-spacing: 0.08em; margin: 24px 0 16px;">next.js app router (vercel / serverless)</h3>
		<p style="color: var(--muted); font-size: 13px; margin-bottom: 8px;">
			Use <code>flushInterval: 0</code> to disable the background timer, and <code>wrapHandler</code> to flush
			before each serverless function exits. <br><br>
			<strong>Note:</strong> Since serverless functions don't keep memory alive between requests, you should send database counts if you want an accumulating line chart.
		</p>
		<pre style="margin-bottom: 24px;">{`// app/api/checkout/route.ts
import { DataCat } from '@datacat/node';
import { wrapHandler } from '@datacat/node/next';

// flushInterval: 0 = serverless mode (no background timer)
const dc = new DataCat({ apiKey: '${key}', flushInterval: 0 });

async function handler(request: Request) {
  // Good: Track absolute database counts for charts that go UP
  // const totalCheckouts = await db.checkouts.count();
  dc.track('total_checkouts', 1234, { env: 'production' });

  return Response.json({ ok: true });
}

// wrapHandler auto-flushes after every request
export const POST = wrapHandler(dc, handler, {
  route: '/api/checkout',
  method: 'POST',
});`}</pre>

	<!-- Option 3: HTTP API -->
	{:else}
		<p style="color: var(--muted); margin-bottom: 16px;">
			POST metrics from any language using the raw ingest endpoint.
		</p>
		<pre style="margin-bottom: 24px;">{`curl -X POST https://api.datacat.com/api/ingest \\
  -H "X-API-Key: ${key}" \\
  -H "Content-Type: application/json" \\
  -d '[{"name":"my_metric","value":42.0,"timestamp":'$(date +%s)'}]'`}</pre>

		<p style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--muted); margin-bottom: 4px;">payload schema</p>
		<pre style="margin-bottom: 24px;">{`// Array of one or more metrics
[
  {
    "name":      "metric_name",   // required
    "value":     42.0,            // required, float
    "timestamp": 1711929600,      // required, unix epoch (seconds)
    "labels":    { "env": "prod" } // optional
  }
]`}</pre>
	{/if}
</div>
