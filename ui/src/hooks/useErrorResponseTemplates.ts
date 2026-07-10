import { useQuery } from "@tanstack/react-query";
import { getErrorResponseTemplates, type ErrorResponseTemplate } from "src/api/backend";

const fetchErrorResponseTemplates = () => getErrorResponseTemplates();

const useErrorResponseTemplates = (options = {}) => {
	return useQuery<ErrorResponseTemplate[], Error>({
		queryKey: ["error-response-templates"],
		queryFn: fetchErrorResponseTemplates,
		staleTime: 30 * 1000,
		...options,
	});
};

export { fetchErrorResponseTemplates, useErrorResponseTemplates };
