import { useQuery } from "@tanstack/react-query";
import { getHeaderPolicyTemplates, type HeaderPolicyTemplate } from "src/api/backend";

const fetchHeaderPolicyTemplates = () => getHeaderPolicyTemplates();

const useHeaderPolicyTemplates = (options = {}) => {
	return useQuery<HeaderPolicyTemplate[], Error>({
		queryKey: ["header-policy-templates"],
		queryFn: fetchHeaderPolicyTemplates,
		staleTime: 30 * 1000,
		...options,
	});
};

export { fetchHeaderPolicyTemplates, useHeaderPolicyTemplates };
