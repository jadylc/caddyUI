import * as api from "./base";
import type { DomainMonitor } from "./models";

export async function renewDomainMonitor(id: number, domain?: string): Promise<DomainMonitor> {
	return await api.post({
		url: `/caddy/domain-monitor/${id}/renew`,
		data: domain ? { domain } : {},
	});
}
