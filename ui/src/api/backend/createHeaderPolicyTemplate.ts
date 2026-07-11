import * as api from "./base";
import type { HeaderPolicyTemplate } from "./models";

export async function createHeaderPolicyTemplate(item: HeaderPolicyTemplate): Promise<HeaderPolicyTemplate> {
	return await api.post({
		url: "/caddy/header-policy-templates",
		data: item,
	});
}
