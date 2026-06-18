import { base } from '$app/paths';

export class CustomerApiError extends Error {
	status: number;

	constructor(status: number, message: string) {
		super(message);
		this.status = status;
	}
}

export async function customerApi<T = unknown>(
	path: string,
	opts: RequestInit & { timeout?: number } = {}
): Promise<T> {
	const { timeout: customTimeout, ...fetchOpts } = opts;
	const controller = new AbortController();
	const timeout = setTimeout(() => controller.abort(), customTimeout ?? 15000);

	try {
		const res = await fetch(`/api${path}`, {
			credentials: 'same-origin',
			signal: controller.signal,
			headers: {
				'Content-Type': 'application/json',
				...fetchOpts.headers
			},
			...fetchOpts
		});

		if (res.status === 401) {
			window.location.href = `${base}/app/login`;
			throw new CustomerApiError(401, 'Unauthorized');
		}

		if (!res.ok) {
			const contentType = res.headers.get('content-type');
			if (contentType?.includes('application/json')) {
				const payload = await res.json().catch(() => null);
				const message = payload?.error?.message || payload?.message || payload?.error;
				throw new CustomerApiError(res.status, message || res.statusText);
			}
			const text = await res.text();
			const trimmed = text.trim();
			const message = trimmed.startsWith('<!doctype') || trimmed.startsWith('<html') ? res.statusText : trimmed;
			throw new CustomerApiError(res.status, message || res.statusText);
		}

		if (res.status === 204) {
			return undefined as T;
		}

		const contentType = res.headers.get('content-type');
		if (contentType?.includes('application/json')) {
			return res.json();
		}
		return res.text() as T;
	} catch (e) {
		if (e instanceof DOMException && e.name === 'AbortError') {
			throw new CustomerApiError(0, 'Request timeout');
		}
		throw e;
	} finally {
		clearTimeout(timeout);
	}
}
