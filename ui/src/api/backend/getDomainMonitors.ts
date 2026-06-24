import * as api from "./base";
import type { DomainMonitor } from "./models";

export async function getDomainMonitors(params = {}): Promise<DomainMonitor[]> {
	return await api.get({
		url: "/caddy/domain-monitor",
		params,
	});
}
