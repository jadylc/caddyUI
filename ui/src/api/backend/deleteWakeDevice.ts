import * as api from "./base";

export async function deleteWakeDevice(id: number): Promise<boolean> {
	return await api.del({
		url: `/caddy/wake-devices/${id}`,
	});
}
