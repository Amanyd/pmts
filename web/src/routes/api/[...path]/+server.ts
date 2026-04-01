import type { RequestHandler } from './$types';

const proxy: RequestHandler = async ({ request, url }) => {
	// In production (Docker Compose), gateway is reachable at 'http://api-gateway:8080'
	// In development, it's 'http://localhost:8080'
	const isProd = process.env.NODE_ENV === 'production';
	const apiUrl = process.env.API_URL || (isProd ? 'http://api-gateway:8080' : 'http://localhost:8080');
	const backendUrl = `${apiUrl}${url.pathname}${url.search}`;

	// Strip 'host' and 'connection' headers to prevent proxy issues
	const headers = new Headers(request.headers);
	headers.delete('host');
	headers.delete('connection');

	const modifiedRequest = new Request(backendUrl, {
		method: request.method,
		headers,
		body: request.body ? request.body : undefined,
		// @ts-ignore - Duplex is required by Node for streaming bodies, but types might be missing
		duplex: 'half'
	});

	try {
		return await fetch(modifiedRequest);
	} catch (e: any) {
		console.error("Proxy Error:", e);
		return new Response(String(e), { status: 502 });
	}
};

export const GET: RequestHandler = proxy;
export const POST: RequestHandler = proxy;
export const PUT: RequestHandler = proxy;
export const PATCH: RequestHandler = proxy;
export const DELETE: RequestHandler = proxy;
