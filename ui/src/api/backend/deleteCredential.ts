import * as api from "./base";

export async function deleteCredential(id: string): Promise<void> {
	await api.del({
		url: `/credentials/${id}`,
	});
}
