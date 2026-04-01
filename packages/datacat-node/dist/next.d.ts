import { DataCat } from './index.js';
type AnyHandler = (...args: unknown[]) => Promise<unknown> | unknown;
/**
 * Wraps a Next.js App Router handler (or any async function) to:
 * 1. Track `http_requests_total` and `http_request_duration_ms`
 * 2. Call `dc.flush()` after the handler completes (critical for serverless)
 *
 * @example
 * ```ts
 * // app/api/users/route.ts
 * import { DataCat } from '@datacat/node';
 * import { wrapHandler } from '@datacat/node/next';
 *
 * const dc = new DataCat({ apiKey: 'sk_...', flushInterval: 0 }); // serverless mode
 *
 * async function handler(request: Request) {
 *   return new Response(JSON.stringify({ users: [] }));
 * }
 *
 * export const GET = wrapHandler(dc, handler, { route: '/api/users' });
 * ```
 */
export declare function wrapHandler<T extends AnyHandler>(dc: DataCat, handler: T, meta?: {
    route?: string;
    method?: string;
}): T;
export {};
//# sourceMappingURL=next.d.ts.map