<script lang="ts">
	import { BRAND_NAME, BRAND_SUPPORT_EMAIL, BRAND_SUPPORT_WECHAT } from '$lib/brand';
</script>

<section class="border-b border-line">
	<div class="mx-auto max-w-5xl px-5 py-16">
		<div class="font-mono text-xs uppercase tracking-wider text-brand">contact</div>
		<h1 class="mt-3 text-4xl font-bold tracking-tight sm:text-5xl">联系我们</h1>
		<p class="mt-4 max-w-3xl text-muted">
			支付、额度、API 接入、企业方案或分销合作，都可以联系 {BRAND_NAME} 支持团队。请在邮件里附上账号邮箱、问题类型和必要的请求 ID。
		</p>
	</div>
</section>

<section class="mx-auto max-w-5xl px-5 py-14">
		<div class="grid gap-4 lg:grid-cols-3">
		<article class="rounded-lg border border-line bg-card/60 p-6">
			<div class="font-mono text-xs uppercase tracking-wider text-brand">email</div>
			<h2 class="mt-3 text-xl font-semibold">支持邮箱</h2>
			<p class="mt-3 text-sm text-faint">账单、API Key、模型可用性、余额异常和企业采购都从这个入口开始。</p>
			<div class="mt-5 rounded-md border border-line bg-black/20 p-3 font-mono text-sm text-brand">{BRAND_SUPPORT_EMAIL}</div>
		</article>
			<article class="rounded-lg border border-line bg-card/60 p-6">
				<div class="font-mono text-xs uppercase tracking-wider text-brand">partner</div>
				<h2 class="mt-3 text-xl font-semibold">分销合作</h2>
			<p class="mt-3 text-sm text-faint">
				如果你是教程作者、技术社区、工具站或企业服务商，请说明渠道类型、预计推广方式和是否需要微信教程素材。
			</p>
				<div class="mt-5 rounded-md border border-line bg-black/20 p-3 text-sm text-muted">主题建议：分销合作 / 企业采购 / API 接入支持</div>
			</article>
			<article class="rounded-lg border border-line bg-card/60 p-6">
				<div class="font-mono text-xs uppercase tracking-wider text-brand">wechat</div>
				<h2 class="mt-3 text-xl font-semibold">加微信获取教程</h2>
				<p class="mt-3 text-sm text-faint">
					参考项目的同类入口已在视觉层保留。当前自有微信入口尚未配置，请先通过支持邮箱说明账号和渠道类型。
				</p>
				<div class="mt-5 rounded-md border border-line bg-black/20 p-3 font-mono text-sm text-faint">微信：{BRAND_SUPPORT_WECHAT}</div>
			</article>
		</div>
	</section>
