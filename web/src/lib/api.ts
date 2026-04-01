// Thin API client — all fetch calls go through here.
// The Vite proxy forwards /api/* to localhost:8080 in dev.

function apiKey(): string {
	if (typeof localStorage === 'undefined') return '';
	return localStorage.getItem('datacat_key') ?? '';
}

function authHeaders(): HeadersInit {
	return {
		'Content-Type': 'application/json',
		'X-API-Key': apiKey()
	};
}

async function get<T>(path: string): Promise<T> {
	const r = await fetch(path, { headers: authHeaders() });
	if (!r.ok) throw new Error(`${r.status}`);
	return r.json();
}

async function post<T>(path: string, body: unknown): Promise<T> {
	const r = await fetch(path, {
		method: 'POST',
		headers: authHeaders(),
		body: JSON.stringify(body)
	});
	if (!r.ok) {
		const msg = await r.text();
		throw new Error(msg || String(r.status));
	}
	return r.json().catch(() => null as unknown as T);
}

async function del(path: string): Promise<void> {
	const r = await fetch(path, { method: 'DELETE', headers: authHeaders() });
	if (!r.ok) throw new Error(String(r.status));
}

// ── Auth ──────────────────────────────────────────────────────────────────────

export async function register(email: string): Promise<{ api_key: string; user_id: string }> {
	const r = await fetch('/api/register', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email })
	});
	if (!r.ok) {
		const msg = await r.text();
		throw new Error(msg || 'Registration failed');
	}
	return r.json();
}

export async function verifyKey(key: string): Promise<boolean> {
	const r = await fetch('/api/health', {
		headers: { 'X-API-Key': key }
	});
	// Health doesn't require auth, but we can try metrics to verify
	const r2 = await fetch('/api/metrics/names', {
		headers: { 'X-API-Key': key }
	});
	return r2.ok;
}

// ── Metrics ───────────────────────────────────────────────────────────────────

export interface Sample { t: number; v: number }
export interface Metric { name: string; samples: Sample[] }

export async function getMetricNames(): Promise<string[]> {
	return get<string[]>('/api/metrics/names');
}

export async function getMetrics(opts?: { name?: string; from?: number; to?: number }): Promise<Metric[]> {
	const p = new URLSearchParams();
	if (opts?.name) p.set('name', opts.name);
	if (opts?.from) p.set('from', String(opts.from));
	if (opts?.to) p.set('to', String(opts.to));
	const qs = p.toString() ? '?' + p.toString() : '';
	const data = await get<Metric[]>('/api/metrics' + qs);
	return data ?? [];
}

export async function deleteMetric(name: string): Promise<void> {
	await del(`/api/metrics?name=${encodeURIComponent(name)}`);
}

// ── Alert rules ───────────────────────────────────────────────────────────────

export interface AlertRule {
	id: number;
	metric: string;
	threshold: number;
	webhook_url: string;
}

export async function getRules(): Promise<AlertRule[]> {
	const data = await get<AlertRule[]>('/api/rules');
	return data ?? [];
}

export async function createRule(rule: { metric: string; threshold: number; webhook_url: string }): Promise<void> {
	await post('/api/rules', rule);
}

export async function deleteRule(id: number): Promise<void> {
	await del(`/api/rules?id=${id}`);
}
