import * as api from "./base";
import type { AuthentikSettings } from "./models";

export async function updateAuthentikSettings(settings: AuthentikSettings): Promise<AuthentikSettings> {
	return await api.put({
		url: "/caddy/settings/authentik",
		data: settings,
	});
}
