/**
 * WAF 审计表格的多维组合过滤纯函数：
 * 不同维度取 AND、同一维度内取 OR；全部未选时展示全部。
 */
import type { WafAuditEntry } from "src/api/backend";
import { classifyWafMessage } from "./translateMessage";

export interface WafAuditFilters {
	hosts?: string[];
	severities?: string[];
	actions?: string[];
	types?: string[];
}

export const filterWafAudits = (items: WafAuditEntry[], filters: WafAuditFilters): WafAuditEntry[] => {
	const hosts = new Set(filters.hosts ?? []);
	const severities = new Set(filters.severities ?? []);
	const actions = new Set(filters.actions ?? []);
	const types = new Set(filters.types ?? []);

	return items.filter((item) => {
		if (hosts.size > 0 && !hosts.has(item.host || "")) return false;
		if (severities.size > 0 && !severities.has(item.severity || "")) return false;
		if (actions.size > 0 && !actions.has(item.action || "")) return false;
		if (types.size > 0) {
			const categories = classifyWafMessage(item.message);
			if (!categories.some((c) => types.has(c))) return false;
		}
		return true;
	});
};