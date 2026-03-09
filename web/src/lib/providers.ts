export interface ProviderOption {
	id: string;
	label: string;
	callback_placeholder: string;
	callback_hint: string;
}

export function addAccountPath(base: string, providerID: string): string {
	return `${base}/add-account/${providerID}`;
}
