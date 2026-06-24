import * as api from "./base";
import type { WakeDevice } from "./models";

export async function getWakeDevice(id: number, params = {}): Promise<WakeDevice> {
	return await api.get({
		url: `/caddy/wake-devices/${id}`,
		params,
	});
}
