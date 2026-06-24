import * as api from "./base";
import type { WakeDevice } from "./models";

export async function wakeDevice(id: number): Promise<WakeDevice> {
	return await api.post({
		url: `/caddy/wake-devices/${id}/wake`,
	});
}
