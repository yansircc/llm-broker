export interface CustomerUser {
	id: string;
	email: string;
	name?: string;
	status: string;
	plan?: string;
	email_verified_at?: string | null;
	created_at?: string;
}

export interface CustomerMe {
	user: CustomerUser;
}

export interface CustomerApiKey {
	id: string;
	name: string;
	prefix?: string;
	status: string;
	created_at: string;
	last_used_at?: string | null;
}

export interface CustomerApiKeyCreated extends CustomerApiKey {
	token: string;
}

export interface BillingSummary {
	plan: string;
	status: string;
	balance_usd: number;
	credits_usd: number;
	usage_usd: number;
	period_start?: string;
	period_end?: string;
}

export interface PaymentOrder {
	id: string;
	out_trade_no?: string;
	status: string;
	amount_usd: number;
	amount_cny?: number;
	checkout_url?: string;
	qrcode?: string;
	qr_image?: string;
	created_at: string;
	paid_at?: string | null;
}

export interface ReferralSummary {
	code: string;
	url?: string;
	signups: number;
	paid_invitees?: number;
	credits_usd: number;
}

export interface BillingLedgerEntry {
	seq: number;
	id: string;
	amount_usd: number;
	kind: string;
	source_type: string;
	source_id: string;
	description: string;
	created_at: string;
}

export interface BillingLedgerResponse {
	entries: BillingLedgerEntry[];
	total: number;
}

export interface CustomerUsageLog {
	id: number;
	request_id: string;
	api_key_id?: string;
	model: string;
	surface: string;
	status: string;
	input_tokens: number;
	output_tokens: number;
	cache_read_tokens: number;
	cache_create_tokens: number;
	cost_usd: number;
	duration_ms: number;
	created_at: string;
}

export interface UsagePeriod {
	label: string;
	requests: number;
	input_tokens: number;
	output_tokens: number;
	cache_read_tokens: number;
	cost_usd: number;
}

export interface ModelUsageRow {
	model: string;
	requests: number;
	input_tokens: number;
	output_tokens: number;
	cache_read_tokens: number;
	cost_usd: number;
}

export interface CustomerUsageResponse {
	logs: CustomerUsageLog[];
	total: number;
	periods: UsagePeriod[];
	model_usage: ModelUsageRow[];
}
