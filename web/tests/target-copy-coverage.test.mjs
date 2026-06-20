import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');

const requiredPhrases = {
	'src/routes/+page.svelte': [
		'六月限时活动',
		'BRAND_TAGLINE',
		'一个 Key，调用所有模型',
		'内置一键 Key 测试功能',
		'新功能 · AI 生图',
		'简单透明，用多少付多少'
	],
	'src/routes/app/keys/+page.svelte': [
		'平台 Key 会安全保存',
		'OpenAI 兼容',
		'Anthropic',
		'创建密钥'
	],
	'src/routes/app/key-test/+page.svelte': [
		'一键检测你的 API Key 是否正常工作',
		'测试会发送一条简短消息',
		'只能测试你自己创建的 Key'
	],
	'src/routes/app/images/+page.svelte': [
		'生图分组',
		'POST /v1/images/generations',
		'支持模型名',
		'$0.04 / 张'
	],
	'src/routes/app/usage/+page.svelte': ['平均耗时', '输入:', '缓存写'],
	'src/routes/app/billing/+page.svelte': ['支持模型', '入门版', '轻量版', '高级版', '商业版', '企业版'],
	'src/routes/app/subscriptions/+page.svelte': ['到期后自动停止', '升级套餐'],
	'src/routes/app/redeem/+page.svelte': ['当前余额', '并发数', '每个兑换码只能使用一次'],
	'src/routes/app/referrals/+page.svelte': ['合伙人等级', '申请提现', '提现', '普通合伙人', '超级合伙人'],
	'src/routes/app/referrals/earnings/+page.svelte': ['推广佣金收入和提现记录'],
	'src/routes/docs/+page.svelte': ['新手入门', '注册与充值', '安装配置', '故障排查'],
	'src/routes/partner/+page.svelte': ['无需囤货', '客户永久绑定', '加微信获取教程']
};

for (const [file, phrases] of Object.entries(requiredPhrases)) {
	const content = readFileSync(resolve(root, file), 'utf8');
	for (const phrase of phrases) {
		assert.ok(content.includes(phrase), `${file} missing target copy phrase: ${phrase}`);
	}
}
