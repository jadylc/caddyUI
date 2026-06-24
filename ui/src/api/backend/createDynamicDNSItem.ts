import * as api from "./base";
import type { DynamicDNS } from "./models";

export async function createDynamicDNSItem(item: DynamicDNS): Promise<DynamicDNS> {
	return await api.post({
		url: "/caddy/dynamic-dns",
		data: item,
	});
}
