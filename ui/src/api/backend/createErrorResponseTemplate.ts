import * as api from "./base";
import type { ErrorResponseTemplate } from "./models";

export async function createErrorResponseTemplate(item: ErrorResponseTemplate): Promise<ErrorResponseTemplate> {
	return await api.post({
		url: "/caddy/error-response-templates",
		data: item,
	});
}
