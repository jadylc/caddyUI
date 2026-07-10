import { useQuery } from "@tanstack/react-query";
import { getObservability, type ObservabilityOverview } from "src/api/backend";

const fetchObservability = () => getObservability();

const useObservability = (options = {}) => {
	return useQuery<ObservabilityOverview, Error>({
		queryKey: ["observability"],
		queryFn: fetchObservability,
		refetchOnWindowFocus: false,
		refetchInterval: 5000,
		staleTime: 4000,
		...options,
	});
};

export { fetchObservability, useObservability };
