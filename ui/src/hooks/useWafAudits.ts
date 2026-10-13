import { useQuery } from "@tanstack/react-query";
import { getWafAudits, type WafAuditParams } from "src/api/backend";
import type { WafAuditResponse } from "src/api/backend";

const fetchWafAudits = (params: WafAuditParams) => {
	return getWafAudits(params);
};

const useWafAudits = (params: WafAuditParams = {}, options = {}) => {
	return useQuery<WafAuditResponse, Error>({
		queryKey: ["waf-audit", params],
		queryFn: () => fetchWafAudits(params),
		staleTime: 10 * 1000,
		...options,
	});
};

export { fetchWafAudits, useWafAudits };