import * as api from "./base";
import type { StaticHost } from "./models";

export async function updateStaticHost(item: StaticHost): Promise<StaticHost> {
	const { id, createdOn: _, modifiedOn: __, ...data } = item;

	return await api.put({
		url: `/caddy/static-hosts/${id}`,
		data,
	});
}
