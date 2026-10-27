import * as api from "./base";
import type { ProviderSpec } from "./models";

export async function getProviders(params = {}): Promise<ProviderSpec[]> {
	return await api.get({
		url: "/caddy/providers",
		params,
	});
}
