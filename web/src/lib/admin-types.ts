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
	last_active_at: string | null;
	total_cost: number;
}

export interface DashboardEvent {
	type: string;
	account_id?: string;
	message: string;
	ts: string;
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
	priority: number;
	priority_mode: string;
	last_used_at: string | null;
	cooldown_until: string | null;
	cell_id?: string;
	cell?: EgressCellSummary | null;
	windows: UtilWindow[];
}

export interface DashboardData {
	health: HealthInfo;
	usage: UsagePeriod[];
	accounts: AccountListItem[];
	users: UserSummary[];
	events: DashboardEvent[];
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
