Here is the complete, formatted content for your **Documentation Page**. You can copy-paste this markdown directly into your docs.

-----

# Integration Guide

DataCat supports two primary integration methods depending on your infrastructure. Choose the one that fits your environment.

## 1\. Serverless Environment (Vercel, Netlify, AWS Lambda)

Use this method for Node.js applications where you cannot run a persistent background process. The DataCat SDK wraps your API routes to measure performance automatically.

### Step 1: Install the SDK

Run this command in your project root:

```bash
npm install @datacat/node
```

### Step 2: Configure Environment

Add your API Key to your deployment's environment variables (e.g., in Vercel Settings or `.env.local`):

  * `DATACAT_KEY`: `sk_live_...` (Your API Key)
  * `DATACAT_URL`: `https://api.datacat.com/api/ingest` (Optional, defaults to official gateway)

### Step 3: Enable Automatic Metrics

Wrap your API route handler with `withDataCat`. This automatically captures **Request Latency**, **Throughput**, and **Memory Usage**.

**Example (Next.js App Router):**

```javascript
import { withDataCat } from '@datacat/node';

async function handler(request) {
  // Your database or business logic
  return Response.json({ status: 'ok' });
}

// Export the wrapped handler
export const GET = withDataCat(handler);
```

**What you will see in the Dashboard:**
Without writing any extra code, you will immediately see these metrics in the dropdown:

  * `http_request_duration_ms`: Execution time per request.
  * `http_request_count`: Total traffic volume.
  * `function_memory_mb`: RAM used by the function execution.

### Step 4: Add Custom Metrics

To track business events (like sales or signups), import the `report` function and call it anywhere in your code.

```javascript
import { report } from '@datacat/node';

export async function POST(req) {
  await processPayment();
  
  // Track a custom event (Name, Value)
  // This is non-blocking (Fire & Forget)
  report('checkout_success', 1); 
  
  return Response.json({ success: true });
}
```

**Viewing New Metrics:**
Deploy your changes and trigger the event. The new metric name (e.g., `checkout_success`) will automatically appear in your Dashboard's "Select Metric" dropdown within seconds.

-----

## 2\. Server Environment (VPS, EC2, Dedicated)

Use this method for persistent Linux servers (Ubuntu, Debian, CentOS). The DataCat Agent runs in the background, monitoring system health and scraping custom data from your apps.

### Step 1: Download & Install Agent

Run this single command on your server to download the agent, configure it with your key, and start it as a system service.

*(Replace `{apiKey}` with your actual key from the dashboard)*

```bash
curl -sfL https://datacat.com/install.sh | DATACAT_KEY={apiKey} sh -
```

### Step 2: Automatic System Metrics

Once the agent is running, it immediately begins reporting infrastructure health. You do **not** need to configure anything else.

**What you will see in the Dashboard:**

  * `system_cpu_percent`: Overall processor load.
  * `system_mem_percent`: RAM usage percentage.
  * `system_disk_percent`: Storage space used.
  * `system_load_1`: 1-minute load average.

### Step 3: Add Custom Metrics (Scraping)

To monitor your specific application (e.g., a Node.js or Go backend), your app simply needs to print metrics to a local URL (e.g., `http://localhost:3000/metrics`).

**1. Expose Metrics in Your App:**
Create a simple HTTP route that outputs text in the format `metric_name value`.

*Node.js Example:*

```javascript
app.get('/metrics', (req, res) => {
  res.send(`
    active_users 42
    jobs_processed 150
  `);
});
```

**2. Configure the Agent:**
When installing (or by editing `/etc/systemd/system/datacat.service`), add the scrape flag:

```bash
./datacat-agent --key=... --scrape=http://localhost:3000/metrics
```

### Step 4: Security (Protecting Your Metrics)

You generally do **not** want your `/metrics` endpoint to be visible to the public internet.

**Best Practice: Bind to Localhost**
When starting your web server, ensure it listens ONLY on the local loopback interface (`127.0.0.1`). This ensures the DataCat Agent (which is on the same server) can reach it, but external hackers cannot.

*Secure Node.js Example:*

```javascript
// Listen on 127.0.0.1 specifically
app.listen(3000, '127.0.0.1', () => {
  console.log("Metrics hidden from public internet");
});
```

**Viewing New Metrics:**
Restart your application and the DataCat agent. The custom metrics (e.g., `active_users`) will appear in your Dashboard dropdown alongside the system metrics.