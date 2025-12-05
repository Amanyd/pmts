<script lang="ts">
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';

    let apiKey = $state("");

    function logout() {
        localStorage.removeItem("datacat_key");
        goto('/');
    }

    onMount(() => {
        const savedKey = localStorage.getItem("datacat_key");
        if (!savedKey) {
            goto('/');
            return;
        }
        apiKey = savedKey;
    });
</script>

<div style="background: #000; color: #fff; min-height: 100vh; padding: 20px; font-family: monospace;">
    
    <div style="margin-bottom: 20px;">
        <a href="/" style="color: #fff; text-decoration: none;">datacat</a>
        <a href="/setrule" style="margin-left: 20px; color: #fff;">[SET RULE]</a>
        <a href="/docs" style="margin-left: 10px; color: #fff;">[DOCS]</a>
        <a href="/about" style="margin-left: 10px; color: #fff;">[ABOUT]</a>
        <button onclick={logout} style="margin-left: 10px; background: none; border: none; color: #fff; cursor: pointer;">[LOGOUT]</button>
    </div>

    <hr style="border-color: #fff; margin: 20px 0;">

    <div style="font-size: 18px;">Integration Guide</div>
    <br>
    <div>DataCat supports two primary integration methods depending on your infrastructure.</div>
    <br>

    <hr style="border-color: #333; margin: 20px 0;">

    <div style="font-size: 16px;">1. Serverless Environment (Vercel, Netlify, AWS Lambda)</div>
    <br>
    <div>Use this method for Node.js applications where you cannot run a persistent background process.</div>
    <br>

    <div>Step 1: Install the SDK</div>
    <pre style="background: #111; padding: 10px; overflow-x: auto;">npm install @datacat/node</pre>
    <br>

    <div>Step 2: Configure Environment</div>
    <div>Add to your deployment's environment variables:</div>
    <pre style="background: #111; padding: 10px; overflow-x: auto;">DATACAT_KEY={apiKey}
DATACAT_URL=https://api.datacat.com/api/ingest</pre>
    <br>

    <div>Step 3: Enable Automatic Metrics</div>
    <div>Wrap your API route handler with withDataCat:</div>
    <pre style="background: #111; padding: 10px; overflow-x: auto;">{`import { withDataCat } from '@datacat/node';

async function handler(request) {
  return Response.json({ status: 'ok' });
}

export const GET = withDataCat(handler);`}</pre>
    <br>

    <div>Automatic metrics captured:</div>
    <div>- http_request_duration_ms: Execution time per request</div>
    <div>- http_request_count: Total traffic volume</div>
    <div>- function_memory_mb: RAM used by the function</div>
    <br>

    <div>Step 4: Add Custom Metrics</div>
    <pre style="background: #111; padding: 10px; overflow-x: auto;">{`import { report } from '@datacat/node';

export async function POST(req) {
  await processPayment();
  report('checkout_success', 1);
  return Response.json({ success: true });
}`}</pre>
    <br>

    <hr style="border-color: #333; margin: 20px 0;">

    <div style="font-size: 16px;">2. Server Environment (VPS, EC2, Dedicated)</div>
    <br>
    <div>Use this method for persistent Linux servers (Ubuntu, Debian, CentOS).</div>
    <br>

    <div>Step 1: Download & Install Agent</div>
    <div>Run this single command on your server:</div>
    <pre style="background: #111; padding: 10px; overflow-x: auto;">curl -sfL https://datacat.com/install.sh | DATACAT_KEY={apiKey} sh -</pre>
    <br>

    <div>Step 2: Automatic System Metrics</div>
    <div>Once running, the agent reports:</div>
    <div>- system_cpu_percent: Overall processor load</div>
    <div>- system_mem_percent: RAM usage percentage</div>
    <div>- system_disk_percent: Storage space used</div>
    <div>- system_load_1: 1-minute load average</div>
    <br>

    <div>Step 3: Add Custom Metrics (Scraping)</div>
    <div>Expose metrics in your app:</div>
    <pre style="background: #111; padding: 10px; overflow-x: auto;">{`app.get('/metrics', (req, res) => {
  res.send(\`
    active_users 42
    jobs_processed 150
  \`);
});`}</pre>
    <br>

    <div>Configure the agent to scrape:</div>
    <pre style="background: #111; padding: 10px; overflow-x: auto;">./datacat-agent --key={apiKey} --scrape=http://localhost:3000/metrics</pre>
    <br>

    <div>Step 4: Security</div>
    <div>Bind your metrics endpoint to localhost only:</div>
    <pre style="background: #111; padding: 10px; overflow-x: auto;">{`app.listen(3000, '127.0.0.1', () => {
  console.log("Metrics hidden from public internet");
});`}</pre>
    <br>

    <hr style="border-color: #333; margin: 20px 0;">

    <div style="font-size: 16px;">Manual Download</div>
    <br>
    <div>Linux (AMD64): <a href="/agent-linux" style="color: #fff;">agent-linux</a></div>
    <div>macOS (ARM64): <a href="/agent-mac" style="color: #fff;">agent-mac</a></div>
    <br>
    <div>Run manually:</div>
    <pre style="background: #111; padding: 10px; overflow-x: auto;">./agent-linux --key={apiKey} --ingest=http://localhost:8080/api/ingest</pre>

</div>
