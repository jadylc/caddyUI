import * as api from "./base";
import type { AutheliaSettings } from "./models";

export async function updateAutheliaSettings(settings: AutheliaSettings): Promise<AutheliaSettings> {
	return await api.put({
		url: "/caddy/settings/authelia",
		data: settings,
	});
}
