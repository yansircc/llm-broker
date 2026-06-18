export function statusLabel(status: string | null | undefined): string {
	switch (status) {
		case 'active':
			return '正常';
		case 'disabled':
			return '已停用';
		case 'blocked':
			return '已阻断';
		case 'error':
			return '异常';
		case 'pending':
			return '待处理';
		case 'paid':
			return '已支付';
		case 'failed':
			return '失败';
		case 'ok':
			return '正常';
		case 'cooling':
			return '冷却中';
		case 'overloaded':
			return '过载';
		case 'transport_error':
			return '传输错误';
		case 'retry':
			return '重试';
		default:
			return status || '-';
	}
}

export function surfaceLabel(surface: string | null | undefined): string {
	switch (surface) {
		case 'native':
			return 'Responses';
		case 'compat':
			return '兼容层';
		case 'all':
			return '全部';
		default:
			return surface || '-';
	}
}

export function roleLabel(role: string | null | undefined): string {
	switch (role) {
		case 'admin':
			return '管理员';
		case 'user':
			return '普通用户';
		default:
			return role || '普通用户';
	}
}

export function egressLabel(value: string | null | undefined): string {
	return value || '直连';
}

export function providerLabel(provider: string | null | undefined): string {
	if (provider === 'openai_compatible') return 'OpenAI 兼容上游';
	return provider || '-';
}

export function weightModeLabel(mode: string | null | undefined): string {
	switch (mode) {
		case 'auto':
			return '自动';
		case 'manual':
			return '手动';
		default:
			return mode || '-';
	}
}

export function healthLabel(value: string | null | undefined): string {
	if (value === 'ok') return '正常';
	return value || '-';
}

export function emptyLabel(value: string | null | undefined): string {
	return value || '-';
}
