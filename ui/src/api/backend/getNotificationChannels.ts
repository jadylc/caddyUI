import * as api from "./base";
import type { NotificationChannelsResponse } from "./models";

export async function getNotificationChannels(params = {}): Promise<NotificationChannelsResponse> {
	return await api.get({
		url: "/notifications/channels",
		params,
	});
}
