import * as api from "./base";
import type { ErrorResponseTemplate } from "./models";

export async function getErrorResponseTemplates(params = {}): Promise<ErrorResponseTemplate[]> {
	return await api.get({
		url: "/caddy/error-response-templates",
		params,
	});
}
