import { useQuery } from "@tanstack/react-query";
import { getDynamicDNSItems, type DynamicDNS } from "src/api/backend";

const fetchDynamicDNSItems = () => getDynamicDNSItems();

const useDynamicDNSItems = (options = {}) => {
	return useQuery<DynamicDNS[], Error>({
		queryKey: ["dynamic-dns"],
		queryFn: fetchDynamicDNSItems,
		staleTime: 15 * 1000,
		refetchInterval: 15 * 1000,
		...options,
	});
};

export { fetchDynamicDNSItems, useDynamicDNSItems };
