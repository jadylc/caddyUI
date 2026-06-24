import * as api from "./base";
import type { NotificationChannel } from "./models";

export async function testNotificationChannel(id: number): Promise<NotificationChannel> {
	return await api.post({
		url: `/notifications/channels/${id}/test`,
	});
}
