import * as api from "./base";
import type { StaticHostExpansion } from "./expansions";
import type { StaticHost } from "./models";

export async function getStaticHost(id: number, expand?: StaticHostExpansion[], params = {}): Promise<StaticHost> {
	return await api.get({
		url: `/caddy/static-hosts/${id}`,
		params: {
			expand: expand?.join(","),
			...params,
		},
	});
}
