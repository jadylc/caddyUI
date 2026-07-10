import * as api from "./base";

export async function deleteErrorResponseTemplate(id: number): Promise<boolean> {
	return await api.del({
		url: `/caddy/error-response-templates/${id}`,
	});
}
