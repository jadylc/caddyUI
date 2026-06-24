import * as api from "./base";
import type { AuthentikSettings } from "./models";

export async function getAuthentikSettings(): Promise<AuthentikSettings> {
	return await api.get({
		url: "/caddy/settings/authentik",
	});
}
