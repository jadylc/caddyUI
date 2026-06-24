import * as api from "./base";

export async function toggleDomainMonitor(id: number, enabled: boolean): Promise<boolean> {
	return await api.post({
		url: `/caddy/domain-monitor/${id}/${enabled ? "enable" : "disable"}`,
	});
}
