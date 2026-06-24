import * as api from "./base";
import type { HostExpansion } from "./expansions";
import type { DeadHost } from "./models";

export async function getDeadHost(id: number, expand?: HostExpansion[], params = {}): Promise<DeadHost> {
	return await api.get({
		url: `/caddy/dead-hosts/${id}`,
		params: {
			expand: expand?.join(","),
			...params,
		},
	});
}
