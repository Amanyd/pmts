# DataCat 📊

A production-ready, highly-scalable open-source metrics monitoring and alerting platform. Built to handle massive throughput using a distributed Go microservice architecture, backed by Protocol Buffers, NATS, and PostgreSQL, all visualized through a lightning-fast SvelteKit 5 dashboard.

## Overview

DataCat consists of three main systems:

1. **The Ingest Engine (Backend)**: Horizontally scalable Go microservices (`api-gateway`, `storage-service`, `alert-service`) communicating via gRPC and a NATS event bus.
2. **The Dashboard (Frontend)**: A neo-brutalist SPA built in SvelteKit that provides live charting, metric exploration, API key management, and alert rule configuration.
3. **The Data Sources**: 
   - **Go System Agent**: A standalone binary that scrapes host CPU/RAM metrics.
   - **Node.js SDK (`@datacat/node`)**: A lightweight Typescript SDK with custom wrappers for Express (auto-instrumentation) and Serverless environments (Next.js App Router).
   - **Raw HTTP API**: Universal ingestion endpoint for any language.

## Quickstart

Run the entire distributed architecture locally using Docker Compose:

```bash
docker compose up --build -d
```

This spins up:
- The SvelteKit Dashboard on `http://localhost:5173`
- The API Gateway on `http://localhost:8080`
- The custom NATS message broker
- The PostgreSQL database
- The Storage and Alerting microservices

### Trying it out
1. Navigate to [http://localhost:5173](http://localhost:5173) to view the landing page.
2. Click **Get Started** to generate an API Key.
3. Once in the dashboard, you have full access to explore metrics and define Webhook thresholds. Navigate to the **Docs** tab in the UI for copy-paste integration snippets injected with your live API Key.

## The Node.js SDK

DataCat was built with modern TypeScript ecosystems in mind. The local SDK handles background flush timers, request batching, and graceful process shutdowns.

### Express Auto-Instrumentation

```typescript
import express from 'express';
import { DataCat } from '@datacat/node';
import { expressMiddleware } from '@datacat/node/express';

const dc = new DataCat({ apiKey: 'sk_YOUR_KEY' });
const app = express();

// Automatically tracks http_request_duration_ms and http_requests_total
app.use(expressMiddleware(dc));
```

### Next.js (Serverless) Support

Serverless providers (like Vercel and AWS Lambda) kill background timers. DataCat provides a custom route wrapper to forcefully flush your metrics before the instance vanishes:

```typescript
// app/api/checkout/route.ts
import { DataCat } from '@datacat/node';
import { wrapHandler } from '@datacat/node/next';

// Disable background timers
const dc = new DataCat({ apiKey: 'sk_YOUR_KEY', flushInterval: 0 });

async function handler(request: Request) {
  // Good: Track absolute database counts for accumulating charts
  const count = await db.checkouts.count();
  dc.track('total_checkouts', count);

  return Response.json({ ok: true });
}

// wrapHandler ensures the metric hits the ingest API before Vercel kills the container
export const POST = wrapHandler(dc, handler, { route: '/api/checkout', method: 'POST' });
```

## Alerting Engine

The `alert-service` runs continuously to evaluate streams against user-defined alert thresholds. If a metric breaches a threshold, the system immediately dispatches a JSON payload via an asynchronous HTTP POST request to your configured webhook URL.

## Architecture & Legacy

The original monolith code that this platform evolved from has been deliberately completely pruned in favor of the production-ready gRPC-driven distributed architecture found in `/cmd` and `/proto`.
