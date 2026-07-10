import * as api from "./base";

export async function toggleErrorResponseTemplate(id: number, enabled: boolean): Promise<boolean> {
	return await api.post({
		url: `/caddy/error-response-templates/${id}/${enabled ? "enable" : "disable"}`,
	});
}
