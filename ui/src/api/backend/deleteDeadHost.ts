import * as api from "./base";

export async function deleteDeadHost(id: number): Promise<boolean> {
	return await api.del({
		url: `/caddy/dead-hosts/${id}`,
	});
}
