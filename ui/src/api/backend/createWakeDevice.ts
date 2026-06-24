import * as api from "./base";
import type { WakeDevice } from "./models";

export async function createWakeDevice(item: WakeDevice): Promise<WakeDevice> {
	return await api.post({
		url: "/caddy/wake-devices",
		data: item,
	});
}
