<script lang="ts">
	interface Props {
		status: string;
	}

	let { status }: Props = $props();

	function badgeClass(value: string) {
		switch (value) {
			case 'active':
			case 'paid':
			case 'ok':
				return 'badge badge-active';
			case 'error':
			case 'blocked':
			case 'failed':
				return 'badge badge-error';
			case 'pending':
			case 'cooling':
				return 'badge badge-pending';
			case 'disabled':
				return 'badge badge-disabled';
			default:
				return 'badge';
		}
	}
</script>

<span class={badgeClass(status)}>{status || '-'}</span>
