export interface CustomerUser {
	id: string;
	email: string;
	name?: string;
	status: string;
	plan?: string;
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
	status: string;
	amount_usd: number;
	checkout_url?: string;
	created_at: string;
	paid_at?: string | null;
}

export interface ReferralSummary {
	code: string;
	url?: string;
	signups: number;
	credits_usd: number;
}
