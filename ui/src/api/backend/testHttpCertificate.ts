import * as api from "./base";

export async function testHttpCertificate(domains: string[]): Promise<Record<string, string>> {
	return await api.post({
		url: "/caddy/certificates/test-http",
		data: {
			domains,
		},
	});
}
