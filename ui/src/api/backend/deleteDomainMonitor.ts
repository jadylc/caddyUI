import * as api from "./base";

export async function deleteDomainMonitor(id: number): Promise<boolean> {
	return await api.del({
		url: `/caddy/domain-monitor/${id}`,
	});
}
