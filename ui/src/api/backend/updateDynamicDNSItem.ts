import * as api from "./base";
import type { DynamicDNS } from "./models";

export async function updateDynamicDNSItem(item: DynamicDNS): Promise<DynamicDNS> {
	const { id, createdOn: _, modifiedOn: __, owner: ___, ...data } = item;

	return await api.put({
		url: `/caddy/dynamic-dns/${id}`,
		data,
	});
}
