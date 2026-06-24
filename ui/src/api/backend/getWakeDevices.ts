import * as api from "./base";
import type { WakeDevice } from "./models";

export async function getWakeDevices(params = {}): Promise<WakeDevice[]> {
	return await api.get({
		url: "/caddy/wake-devices",
		params,
	});
}
