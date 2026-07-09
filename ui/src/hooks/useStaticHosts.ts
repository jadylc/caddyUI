import { useQuery } from "@tanstack/react-query";
import { getStaticHosts, type StaticHost, type StaticHostExpansion } from "src/api/backend";

const fetchStaticHosts = (expand?: StaticHostExpansion[]) => {
	return getStaticHosts(expand);
};

const useStaticHosts = (expand?: StaticHostExpansion[], options = {}) => {
	return useQuery<StaticHost[], Error>({
		queryKey: ["static-hosts", { expand }],
		queryFn: () => fetchStaticHosts(expand),
		staleTime: 60 * 1000,
		...options,
	});
};

export { fetchStaticHosts, useStaticHosts };
