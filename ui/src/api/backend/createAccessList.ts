import * as api from "./base";
import type { AccessList } from "./models";

export async function createAccessList(item: AccessList): Promise<AccessList> {
	return await api.post({
		url: "/caddy/access-lists",
		data: item,
	});
}
