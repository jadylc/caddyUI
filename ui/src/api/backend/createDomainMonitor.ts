import * as api from "./base";
import type { DomainMonitor } from "./models";

export async function createDomainMonitor(item: DomainMonitor): Promise<DomainMonitor> {
	return await api.post({
		url: "/caddy/domain-monitor",
		data: item,
	});
}
