import * as api from "./base";

export interface CredentialInput {
	name: string;
	provider: string;
	// 新增 provider 的密钥统一放这里；后端按注册表回填历史字段。
	secrets?: Record<string, string>;
	aliyunKey?: string;
	aliyunSecret?: string;
	cfToken?: string;
	dnspodToken?: string;
	heApiKey?: string;
	digitalPlatApiKey?: string;
	dnsheApiKey?: string;
	dnsheApiSecret?: string;
}

export async function createCredential(data: CredentialInput): Promise<{ id: string }> {
	return await api.post({
		url: "/credentials",
		data,
	});
}
