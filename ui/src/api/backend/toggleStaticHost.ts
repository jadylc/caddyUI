import * as api from "./base";

export async function toggleStaticHost(id: number, enabled: boolean): Promise<boolean> {
	return await api.post({
		url: `/caddy/static-hosts/${id}/${enabled ? "enable" : "disable"}`,
	});
}
