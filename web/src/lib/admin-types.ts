export type UserSurface = 'native' | 'compat' | 'all';

export interface UsagePeriod {
	label: string;
	requests: number;
	input_tokens: number;
	output_tokens: number;
	cache_read_tokens: number;
	cost_usd: number;
}

export interface UtilWindow {
	label: string;
	pct: number;
	reset?: number;
}

export interface HealthInfo {
	sqlite: string;
	uptime: string;
	version: string;
}

export interface UserSummary {
	id: string;
	name: string;
	status: string;
	allowed_surface: UserSurface;
	bound_account_id?: string;
	bound_account_email?: string;
	last_active_at: string | null;
	total_cost: number;
}

export interface DashboardEvent {
	type: string;
	account_id?: string;
	user_id?: string;
	bucket_key?: string;
	cell_id?: string;
	cooldown_until?: string;
	upstream_status?: number;
	upstream_error_type?: string;
	upstream_error_message?: string;
	message: string;
	ts: string;
}

export interface RecentRequestLog {
	id: number;
	user_id: string;
	account_id: string;
	provider: string;
	surface: string;
	model: string;
	path: string;
	cell_id?: string;
	bucket_key?: string;
	session_uuid?: string;
	binding_source?: string;
	client_headers?: Record<string, unknown> | null;
	client_body_excerpt?: string;
	request_meta?: Record<string, unknown> | null;
	input_tokens: number;
	output_tokens: number;
	cache_read_tokens: number;
	cache_create_tokens: number;
	cost_usd: number;
	status: string;
	effect_kind?: string;
	upstream_status?: number;
	upstream_url?: string;
	upstream_request_headers?: Record<string, unknown> | null;
	upstream_request_meta?: Record<string, unknown> | null;
	upstream_request_body_excerpt?: string;
	upstream_request_id?: string;
	upstream_headers?: Record<string, unknown> | null;
	upstream_response_meta?: Record<string, unknown> | null;
	upstream_response_body_excerpt?: string;
	upstream_error_type?: string;
	upstream_error_message?: string;
	request_bytes: number;
	attempt_count: number;
	duration_ms: number;
	created_at: string;
}

export interface RelayOutcomeStat {
	provider: string;
	surface: string;
	effect_kind: string;
	upstream_status?: number;
	requests: number;
	distinct_users: number;
	distinct_accounts: number;
	last_seen_at: string;
}

export interface CellRiskStat {
	cell_id?: string;
	cell_name: string;
	provider: string;
	region: string;
	transport: string;
	requests: number;
	successes: number;
	status_400: number;
	status_403: number;
	status_429: number;
	blocks: number;
	transport_errors: number;
	distinct_users: number;
	distinct_accounts: number;
	last_seen_at: string;
}

export interface EgressCellSummary {
	id: string;
	name: string;
	status: string;
	labels?: Record<string, string>;
	cooldown_until?: string | null;
	account_count?: number;
}

export interface AccountListItem {
	id: string;
	email: string;
	provider: string;
	status: string;
	weight: number;
	weight_mode: string;
	last_used_at: string | null;
	cooldown_until: string | null;
	cell_id?: string;
	available_native: boolean;
	available_compat: boolean;
	cell?: EgressCellSummary | null;
	windows: UtilWindow[];
}

export interface DashboardData {
	health: HealthInfo;
	usage: UsagePeriod[];
	accounts: AccountListItem[];
	users: UserSummary[];
	events: DashboardEvent[];
	outcome_stats: RelayOutcomeStat[];
	cell_risk: CellRiskStat[];
	recent_failures: RecentRequestLog[];
}

export interface ProxyConfig {
	type: string;
	host: string;
	port: number;
	username?: string;
	password?: string;
}

export interface EgressCellAccountRef {
	id: string;
	email: string;
	provider: string;
	status: string;
}

export interface EgressCellView {
	id: string;
	name: string;
	status: string;
	proxy?: ProxyConfig | null;
	labels?: Record<string, string>;
	cooldown_until?: string | null;
	state_json?: string;
	created_at: string;
	updated_at: string;
	accounts: EgressCellAccountRef[];
}
