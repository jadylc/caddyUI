import * as api from "./base";
import type { Certificate } from "./models";

export async function uploadCertificate(id: number, data: FormData): Promise<Certificate> {
	return await api.post({
		url: `/caddy/certificates/${id}/upload`,
		data,
	});
}
