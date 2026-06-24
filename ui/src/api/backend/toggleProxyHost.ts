import * as api from "./base";

export async function toggleProxyHost(id: number, enabled: boolean): Promise<boolean> {
	return await api.post({
		url: `/caddy/proxy-hosts/${id}/${enabled ? "enable" : "disable"}`,
	});
}
