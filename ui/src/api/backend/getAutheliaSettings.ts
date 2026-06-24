import * as api from "./base";
import type { AutheliaSettings } from "./models";

export async function getAutheliaSettings(): Promise<AutheliaSettings> {
	return await api.get({
		url: "/caddy/settings/authelia",
	});
}
