import * as api from "./base";
import type { StaticHost } from "./models";

export async function createStaticHost(item: StaticHost): Promise<StaticHost> {
	return await api.post({
		url: "/caddy/static-hosts",
		data: item,
	});
}
