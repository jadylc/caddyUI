import * as api from "./base";
import type { AuthentikSettings, AutheliaSettings } from "./models";

export type ForwardAuthSettings = AutheliaSettings | AuthentikSettings;

export async function getForwardAuthSettings<T extends ForwardAuthSettings>(provider: string): Promise<T> {
	return await api.get({
		url: `/caddy/settings/forward-auth/${provider}`,
	});
}
