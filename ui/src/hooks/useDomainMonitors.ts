import { useQuery } from "@tanstack/react-query";
import { getDomainMonitors, type DomainMonitor } from "src/api/backend";

const fetchDomainMonitors = () => getDomainMonitors();

const useDomainMonitors = (options = {}) => {
	return useQuery<DomainMonitor[], Error>({
		queryKey: ["domain-monitor"],
		queryFn: fetchDomainMonitors,
		staleTime: 60 * 1000,
		...options,
	});
};

export { fetchDomainMonitors, useDomainMonitors };
