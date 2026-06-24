import * as api from "./base";
import type { CredentialDetail } from "./models";

export async function updateCredential(item: CredentialDetail): Promise<void> {
	await api.put({
		url: `/credentials/${item.id}`,
		data: item,
	});
}
