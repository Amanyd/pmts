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
export declare class DataCat {
    private readonly apiKey;
    private readonly endpoint;
    private readonly maxBatchSize;
    private buffer;
    private timer;
    constructor(opts: DataCatOptions);
    /**
     * Buffer a metric. Safe to call on every request — it's synchronous and O(1).
     */
    track(name: string, value: number, labels?: Record<string, string>): void;
    /**
     * Flush buffered metrics to the DataCat ingest API.
     * Always await this before your serverless function returns.
     */
    flush(): Promise<void>;
    /**
     * Stop the auto-flush timer and send remaining buffered metrics.
     * Call this in your process shutdown handler.
     */
    shutdown(): Promise<void>;
}
//# sourceMappingURL=index.d.ts.map