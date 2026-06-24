import * as api from "./base";

export async function toggleDynamicDNSItem(id: number, enabled: boolean): Promise<boolean> {
	return await api.post({
		url: `/caddy/dynamic-dns/${id}/${enabled ? "enable" : "disable"}`,
	});
}
