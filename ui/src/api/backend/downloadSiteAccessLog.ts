import * as api from "./base";
import type { SiteAccessLogKind } from "./getSiteAccessLogs";

const hostPath = (kind: SiteAccessLogKind) => (kind === "static" ? "static-hosts" : "proxy-hosts");

export async function downloadSiteAccessLog(kind: SiteAccessLogKind, id: number): Promise<void> {
	await api.download(
		{
			url: `/caddy/${hostPath(kind)}/${id}/logs/download`,
		},
		`${kind}-host-${id}-access.log`,
	);
}
