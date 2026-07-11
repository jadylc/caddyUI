import * as api from "./base";

export async function toggleHeaderPolicyTemplate(id: number, enabled: boolean): Promise<boolean> {
	return await api.post({
		url: `/caddy/header-policy-templates/${id}/${enabled ? "enable" : "disable"}`,
	});
}
