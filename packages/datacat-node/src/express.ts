import type { Request, Response, NextFunction } from 'express';
import { DataCat } from './index.js';

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
export function expressMiddleware(dc: DataCat) {
  return function dataCatMiddleware(req: Request, res: Response, next: NextFunction): void {
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
