import * as api from "./base";
import type { CertificateAuthority } from "./models";

export async function getCertificateAuthorities(): Promise<CertificateAuthority[]> {
	return await api.get({
		url: "/caddy/certificates/authorities",
	});
}
