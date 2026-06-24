import * as api from "./base";
import type { CredentialDetail } from "./models";

export async function getCredential(id: string): Promise<CredentialDetail> {
	return await api.get({
		url: `/credentials/${id}`,
	});
}
