import * as api from "./base";
import type { DeadHost } from "./models";

export async function createDeadHost(item: DeadHost): Promise<DeadHost> {
	return await api.post({
		url: "/caddy/dead-hosts",
		data: item,
	});
}
