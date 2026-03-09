class ApiError extends Error {
	status: number;
	constructor(status: number, message: string) {
		super(message);
		this.status = status;
	}
}

export async function api<T = unknown>(path: string, opts: RequestInit = {}): Promise<T> {
	const controller = new AbortController();
	const timeout = setTimeout(() => controller.abort(), 15000);

	// Append browser timezone for accurate "today"/"yesterday" stats
	const sep = path.includes('?') ? '&' : '?';
	const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
	const url = `/admin${path}${sep}tz=${encodeURIComponent(tz)}`;

	try {
		const res = await fetch(url, {
			credentials: 'same-origin',
			signal: controller.signal,
			headers: {
				'Content-Type': 'application/json',
				...opts.headers
			},
			...opts
		});

		if (res.status === 401) {
			window.location.href = '/login';
			throw new ApiError(401, 'Unauthorized');
		}

		if (!res.ok) {
			const text = await res.text();
			throw new ApiError(res.status, text || res.statusText);
		}

		const contentType = res.headers.get('content-type');
		if (contentType?.includes('application/json')) {
			return res.json();
		}
		return res.text() as unknown as T;
	} catch (e) {
		if (e instanceof DOMException && e.name === 'AbortError') {
			throw new ApiError(0, 'Request timeout');
		}
		throw e;
	} finally {
		clearTimeout(timeout);
	}
}
