import * as api from "./base";
import type { NotificationChannel } from "./models";

export async function createNotificationChannel(item: NotificationChannel): Promise<NotificationChannel> {
	return await api.post({
		url: "/notifications/channels",
		data: item,
	});
}
