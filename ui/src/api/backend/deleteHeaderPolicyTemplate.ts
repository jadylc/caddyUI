import * as api from "./base";

export async function deleteHeaderPolicyTemplate(id: number): Promise<boolean> {
	return await api.del({
		url: `/caddy/header-policy-templates/${id}`,
	});
}
