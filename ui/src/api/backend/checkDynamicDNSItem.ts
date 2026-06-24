import * as api from "./base";
import type { DynamicDNS } from "./models";

export async function checkDynamicDNSItem(id: number): Promise<DynamicDNS> {
	return await api.post({
		url: `/caddy/dynamic-dns/${id}/check`,
	});
}
