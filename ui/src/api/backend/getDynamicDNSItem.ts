import * as api from "./base";
import type { DynamicDNS } from "./models";

export async function getDynamicDNSItem(id: number, params = {}): Promise<DynamicDNS> {
	return await api.get({
		url: `/caddy/dynamic-dns/${id}`,
		params,
	});
}
