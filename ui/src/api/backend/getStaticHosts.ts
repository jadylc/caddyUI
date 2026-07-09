import * as api from "./base";
import type { StaticHostExpansion } from "./expansions";
import type { StaticHost } from "./models";

export async function getStaticHosts(expand?: StaticHostExpansion[], params = {}): Promise<StaticHost[]> {
	return await api.get({
		url: "/caddy/static-hosts",
		params: {
			expand: expand?.join(","),
			...params,
		},
	});
}
