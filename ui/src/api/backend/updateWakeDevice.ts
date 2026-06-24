import * as api from "./base";
import type { WakeDevice } from "./models";

export async function updateWakeDevice(item: WakeDevice): Promise<WakeDevice> {
	const { id, createdOn: _, modifiedOn: __, owner: ___, ...data } = item;

	return await api.put({
		url: `/caddy/wake-devices/${id}`,
		data,
	});
}
