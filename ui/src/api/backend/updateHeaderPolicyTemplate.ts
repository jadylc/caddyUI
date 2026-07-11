import * as api from "./base";
import type { HeaderPolicyTemplate } from "./models";

export async function updateHeaderPolicyTemplate(item: HeaderPolicyTemplate): Promise<HeaderPolicyTemplate> {
	const { id, createdOn: _, modifiedOn: __, owner: ___, hostCount: ____, builtin: _____, ...data } = item;

	return await api.put({
		url: `/caddy/header-policy-templates/${id}`,
		data,
	});
}
