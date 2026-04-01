# @datacat/node

Push custom metrics to [DataCat](https://datacat.com) from any Node.js server or serverless function.

## Install

```bash
npm install @datacat/node
# or
bun add @datacat/node
```

## Quick Start

```typescript
import { DataCat } from '@datacat/node';

const dc = new DataCat({ apiKey: 'sk_...' });

// Track anything
dc.track('signups_today', 1);
dc.track('api_latency_ms', 230, { route: '/users' });

// Graceful shutdown (flushes remaining metrics)
process.on('SIGTERM', () => dc.shutdown());
```

Metrics are **buffered in memory** and flushed to DataCat in a single batch every 5 seconds. No per-request HTTP overhead.

---

## Express (auto-instrument all routes)

```typescript
import express from 'express';
import { DataCat } from '@datacat/node';
import { expressMiddleware } from '@datacat/node/express';

const dc = new DataCat({ apiKey: 'sk_...' });
const app = express();

// Auto-tracks http_requests_total + http_request_duration_ms
app.use(expressMiddleware(dc));

app.get('/api/orders', (req, res) => {
  res.json({ orders: [] });
});

process.on('SIGTERM', () => dc.shutdown());
```

---

## Next.js App Router (Vercel / serverless)

Use `flushInterval: 0` to disable auto-flush (no persistent timer in serverless) and use `wrapHandler` to flush after each request.

```typescript
// app/api/checkout/route.ts
import { DataCat } from '@datacat/node';
import { wrapHandler } from '@datacat/node/next';

const dc = new DataCat({ apiKey: 'sk_...', flushInterval: 0 });

async function handler(request: Request) {
  // Track anything business-specific
  dc.track('checkout_attempts', 1, { region: 'us-east' });
  return Response.json({ ok: true });
}

// Wraps handler: auto-tracks latency + calls flush() before function exits
export const POST = wrapHandler(dc, handler, { route: '/api/checkout', method: 'POST' });
```

---

## API Reference

### `new DataCat(options)`

| Option | Type | Default | Description |
|---|---|---|---|
| `apiKey` | `string` | required | Your DataCat API key |
| `endpoint` | `string` | `https://api.datacat.com/api/ingest` | Custom ingest URL |
| `flushInterval` | `number` | `5000` | ms between auto-flushes. Set `0` for serverless. |
| `maxBatchSize` | `number` | `500` | Force-flush when buffer hits this size |

### `dc.track(name, value, labels?)`
Buffer a metric. Non-blocking, synchronous.

### `dc.flush()`
Send buffered metrics immediately. Returns `Promise<void>`.

### `dc.shutdown()`
Stop auto-flush timer + final flush. Call in `SIGTERM` handler.
