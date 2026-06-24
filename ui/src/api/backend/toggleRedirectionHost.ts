import * as api from "./base";

export async function toggleRedirectionHost(id: number, enabled: boolean): Promise<boolean> {
	return await api.post({
		url: `/caddy/redirection-hosts/${id}/${enabled ? "enable" : "disable"}`,
	});
}
