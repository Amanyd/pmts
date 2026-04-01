/**
 * Express middleware — automatically tracks request count and latency.
 *
 * @example
 * ```ts
 * import express from 'express';
 * import { DataCat } from '@datacat/node';
 * import { expressMiddleware } from '@datacat/node/express';
 *
 * const dc = new DataCat({ apiKey: 'sk_...' });
 * const app = express();
 * app.use(expressMiddleware(dc));
 * ```
 */
export function expressMiddleware(dc) {
    return function dataCatMiddleware(req, res, next) {
        const start = Date.now();
        res.on('finish', () => {
            const duration = Date.now() - start;
            const labels = {
                method: req.method,
                route: req.route?.path ?? req.path,
                status: String(res.statusCode),
            };
            dc.track('http_requests_total', 1, labels);
            dc.track('http_request_duration_ms', duration, labels);
        });
        next();
    };
}
//# sourceMappingURL=express.js.map