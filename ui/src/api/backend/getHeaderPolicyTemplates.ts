import * as api from "./base";
import type { HeaderPolicyTemplate } from "./models";

export async function getHeaderPolicyTemplates(params = {}): Promise<HeaderPolicyTemplate[]> {
	return await api.get({
		url: "/caddy/header-policy-templates",
		params,
	});
}
