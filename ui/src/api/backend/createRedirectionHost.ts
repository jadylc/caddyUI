import * as api from "./base";
import type { RedirectionHost } from "./models";

export async function createRedirectionHost(item: RedirectionHost): Promise<RedirectionHost> {
	return await api.post({
		url: "/caddy/redirection-hosts",
		data: item,
	});
}
