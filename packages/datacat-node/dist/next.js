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
export function wrapHandler(dc, handler, meta) {
    return (async (...args) => {
        const start = Date.now();
        let status = 200;
        try {
            const result = await handler(...args);
            // Next.js Response object
            if (result && typeof result === 'object' && 'status' in result) {
                status = result.status;
            }
            return result;
        }
        catch (err) {
            status = 500;
            throw err;
        }
        finally {
            const duration = Date.now() - start;
            const labels = {
                route: meta?.route ?? 'unknown',
                method: meta?.method ?? 'GET',
                status: String(status),
            };
            dc.track('http_requests_total', 1, labels);
            dc.track('http_request_duration_ms', duration, labels);
            // Flush immediately — serverless functions die after returning
            await dc.flush();
        }
    });
}
//# sourceMappingURL=next.js.map