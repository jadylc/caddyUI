import * as api from "./base";
import type { DomainMonitor } from "./models";

export async function checkDomainMonitor(id: number): Promise<DomainMonitor> {
	return await api.post({
		url: `/caddy/domain-monitor/${id}/check`,
	});
}
