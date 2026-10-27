import { useQuery } from "@tanstack/react-query";
import { type ProviderSpec, getProviders } from "src/api/backend";

const fetchProviders = () => {
	return getProviders();
};

const useProviders = (options = {}) => {
	return useQuery<ProviderSpec[], Error>({
		queryKey: ["providers"],
		queryFn: () => fetchProviders(),
		staleTime: 0,
		...options,
	});
};

export { fetchProviders, useProviders };
