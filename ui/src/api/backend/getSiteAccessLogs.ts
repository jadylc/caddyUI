import * as api from "./base";
import type { SiteAccessLog } from "./models";

export type SiteAccessLogKind = "proxy" | "static";

const hostPath = (kind: SiteAccessLogKind) => (kind === "static" ? "static-hosts" : "proxy-hosts");

export async function getSiteAccessLogs(kind: SiteAccessLogKind, id: number, lines = 300): Promise<SiteAccessLog> {
	return api.get({
		url: `/caddy/${hostPath(kind)}/${id}/logs`,
		params: { n: lines },
	});
}
