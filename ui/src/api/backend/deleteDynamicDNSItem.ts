import * as api from "./base";

export async function deleteDynamicDNSItem(id: number): Promise<boolean> {
	return await api.del({
		url: `/caddy/dynamic-dns/${id}`,
	});
}
