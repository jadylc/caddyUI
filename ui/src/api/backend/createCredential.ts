import * as api from "./base";

export interface CredentialInput {
	name: string;
	provider: string;
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
