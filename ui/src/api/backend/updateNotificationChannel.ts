import * as api from "./base";
import type { NotificationChannel } from "./models";

export async function updateNotificationChannel(item: NotificationChannel): Promise<NotificationChannel> {
	return await api.put({
		url: `/notifications/channels/${item.id}`,
		data: item,
	});
}
