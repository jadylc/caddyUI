import * as api from "./base";

export async function getLogs(n = 200): Promise<string[]> {
	return await api.get({
		url: "/logs",
		params: { n },
	});
}
