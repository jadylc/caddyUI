import * as api from "./base";
import type { AuthentikSettings, AutheliaSettings } from "./models";

type ForwardAuthSettings = AutheliaSettings | AuthentikSettings;

export async function updateForwardAuthSettings<T extends ForwardAuthSettings>(
	provider: string,
	settings: T,
): Promise<T> {
	return await api.put({
		url: `/caddy/settings/forward-auth/${provider}`,
		data: settings,
	});
}
