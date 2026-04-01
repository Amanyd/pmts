export interface DataCatOptions {
  /** Your DataCat API key (sk_...) */
  apiKey: string;
  /** Ingest endpoint. Defaults to https://api.datacat.com/api/ingest */
  endpoint?: string;
  /**
   * How often (ms) to auto-flush buffered metrics.
   * Set to 0 to disable auto-flush (serverless mode — call flush() manually).
   * Defaults to 5000 (5 seconds).
   */
  flushInterval?: number;
  /** Max metrics to buffer before forcing a flush. Defaults to 500. */
  maxBatchSize?: number;
}

export interface Metric {
  name: string;
  value: number;
  timestamp?: number;
  labels?: Record<string, string>;
}

interface IngestPayload {
  name: string;
  value: number;
  timestamp: number;
  labels?: Record<string, string>;
}

export class DataCat {
  private readonly apiKey: string;
  private readonly endpoint: string;
  private readonly maxBatchSize: number;
  private buffer: IngestPayload[] = [];
  private timer: ReturnType<typeof setInterval> | null = null;

  constructor(opts: DataCatOptions) {
    if (!opts.apiKey) throw new Error('[DataCat] apiKey is required');
    this.apiKey = opts.apiKey;
    this.endpoint = opts.endpoint ?? 'https://api.datacat.com/api/ingest';
    this.maxBatchSize = opts.maxBatchSize ?? 500;

    const interval = opts.flushInterval ?? 5000;
    if (interval > 0) {
      this.timer = setInterval(() => { void this.flush(); }, interval);
      // Don't block Node process exit
      if (this.timer.unref) this.timer.unref();
    }
  }

  /**
   * Buffer a metric. Safe to call on every request — it's synchronous and O(1).
   */
  track(name: string, value: number, labels?: Record<string, string>): void {
    this.buffer.push({
      name,
      value,
      timestamp: Math.floor(Date.now() / 1000),
      ...(labels ? { labels } : {}),
    });
    if (this.buffer.length >= this.maxBatchSize) {
      void this.flush();
    }
  }

  /**
   * Flush buffered metrics to the DataCat ingest API.
   * Always await this before your serverless function returns.
   */
  async flush(): Promise<void> {
    if (this.buffer.length === 0) return;
    const batch = this.buffer.splice(0); // take all, clear buffer
    try {
      const res = await fetch(this.endpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-API-Key': this.apiKey,
        },
        body: JSON.stringify(batch),
      });
      if (!res.ok) {
        console.error(`[DataCat] flush failed: ${res.status} ${res.statusText}`);
        // Put metrics back so they aren't lost on transient errors
        this.buffer.unshift(...batch);
      }
    } catch (err) {
      console.error('[DataCat] flush error:', err);
      this.buffer.unshift(...batch);
    }
  }

  /**
   * Stop the auto-flush timer and send remaining buffered metrics.
   * Call this in your process shutdown handler.
   */
  async shutdown(): Promise<void> {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
    await this.flush();
  }
}
