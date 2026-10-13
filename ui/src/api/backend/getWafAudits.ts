import * as api from "./base";
import type { WafAuditResponse } from "./models";

export interface WafAuditParams {
	limit?: number;
	offset?: number;
	host?: string;
}

export async function getWafAudits(params: WafAuditParams = {}): Promise<WafAuditResponse> {
	return await api.get({
		url: "/caddy/waf/audit",
		params: {
			limit: 500,
			offset: 0,
			...params,
		},
	});
}