import * as api from "./base";

export async function downloadCertificate(id: number): Promise<void> {
	await api.download(
		{
			url: `/caddy/certificates/${id}/download`,
		},
		`certificate-${id}.zip`,
	);
}
