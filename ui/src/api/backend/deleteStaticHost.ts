import * as api from "./base";

export async function deleteStaticHost(id: number): Promise<boolean> {
	return await api.del({
		url: `/caddy/static-hosts/${id}`,
	});
}
