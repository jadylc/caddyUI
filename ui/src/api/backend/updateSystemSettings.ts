import * as api from "./base";
import type { SystemSettings } from "./models";

export async function updateSystemSettings(settings: SystemSettings): Promise<SystemSettings> {
	return await api.put({ url: "/caddy/settings", data: settings });
}
