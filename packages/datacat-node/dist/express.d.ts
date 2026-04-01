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
export declare function expressMiddleware(dc: DataCat): (req: Request, res: Response, next: NextFunction) => void;
//# sourceMappingURL=express.d.ts.map