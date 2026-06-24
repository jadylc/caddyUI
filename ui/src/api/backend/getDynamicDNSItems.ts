import * as api from "./base";
import type { DynamicDNS } from "./models";

export async function getDynamicDNSItems(params = {}): Promise<DynamicDNS[]> {
	return await api.get({
		url: "/caddy/dynamic-dns",
		params,
	});
}
