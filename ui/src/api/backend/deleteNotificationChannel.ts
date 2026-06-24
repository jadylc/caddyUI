import * as api from "./base";

export async function deleteNotificationChannel(id: number): Promise<boolean> {
	return await api.del({
		url: `/notifications/channels/${id}`,
	});
}
