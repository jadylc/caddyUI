import * as api from "./base";
import type { HeaderPolicyTemplate } from "./models";

export async function getHeaderPolicyTemplate(id: number): Promise<HeaderPolicyTemplate> {
	return await api.get({
		url: `/caddy/header-policy-templates/${id}`,
	});
}
