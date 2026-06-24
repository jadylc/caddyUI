import { useQuery } from "@tanstack/react-query";
import { type CredentialSummary, getCredentials } from "src/api/backend";

const fetchCredentials = () => {
	return getCredentials();
};

const useCredentials = (options = {}) => {
	return useQuery<CredentialSummary[], Error>({
		queryKey: ["credentials"],
		queryFn: () => fetchCredentials(),
		staleTime: 300 * 1000,
		...options,
	});
};

export { fetchCredentials, useCredentials };
