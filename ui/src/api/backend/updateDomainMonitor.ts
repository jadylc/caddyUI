import * as api from "./base";
import type { DomainMonitor } from "./models";

export async function updateDomainMonitor(item: DomainMonitor): Promise<DomainMonitor> {
	const { id, createdOn: _, modifiedOn: __, owner: ___, meta: ____, ...data } = item;

	return await api.put({
		url: `/caddy/domain-monitor/${id}`,
		data,
	});
}
