import * as api from "./base";
import type { SystemSettings } from "./models";

export async function getSystemSettings(): Promise<SystemSettings> {
	return await api.get({ url: "/caddy/settings" });
}
