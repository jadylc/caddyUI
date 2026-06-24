import * as api from "./base";
import type { HostExpansion } from "./expansions";
import type { Stream } from "./models";

export async function getStream(id: number, expand?: HostExpansion[], params = {}): Promise<Stream> {
	return await api.get({
		url: `/caddy/streams/${id}`,
		params: {
			expand: expand?.join(","),
			...params,
		},
	});
}
