import * as api from "./base";
import type { ErrorResponseTemplate } from "./models";

export async function updateErrorResponseTemplate(item: ErrorResponseTemplate): Promise<ErrorResponseTemplate> {
	const { id, createdOn: _, modifiedOn: __, owner: ___, hostCount: ____, ...data } = item;

	return await api.put({
		url: `/caddy/error-response-templates/${id}`,
		data,
	});
}
