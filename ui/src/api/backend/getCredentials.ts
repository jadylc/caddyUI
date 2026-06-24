import * as api from "./base";
import type { CredentialSummary } from "./models";

export async function getCredentials(params = {}): Promise<CredentialSummary[]> {
	return await api.get({
		url: "/credentials",
		params,
	});
}
