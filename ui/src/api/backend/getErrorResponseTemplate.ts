import * as api from "./base";
import type { ErrorResponseTemplate } from "./models";

export async function getErrorResponseTemplate(id: number): Promise<ErrorResponseTemplate> {
	return await api.get({
		url: `/caddy/error-response-templates/${id}`,
	});
}
