import * as api from "./base";

export async function toggleWakeDevice(id: number, enabled: boolean): Promise<boolean> {
	return await api.post({
		url: `/caddy/wake-devices/${id}/${enabled ? "enable" : "disable"}`,
	});
}
