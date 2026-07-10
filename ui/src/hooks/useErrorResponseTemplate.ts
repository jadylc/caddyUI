import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
	createErrorResponseTemplate,
	getErrorResponseTemplate,
	type ErrorResponseTemplate,
	updateErrorResponseTemplate,
} from "src/api/backend";

const defaultErrorResponseTemplate = (): ErrorResponseTemplate => ({
	id: 0,
	createdOn: "",
	modifiedOn: "",
	ownerUserId: 0,
	name: "",
	description: "",
	enabled: true,
	staticRoot: "",
	rules: [
		{
			id: 1,
			statuses: ["404"],
			action: "static_file",
			staticFile: "/404.html",
		},
		{
			id: 2,
			statuses: ["5xx"],
			action: "static_file",
			staticFile: "/50x.html",
		},
	],
	meta: {},
	hostCount: 0,
});

const fetchErrorResponseTemplate = (id: number | "new") => {
	if (id === "new") {
		return Promise.resolve(defaultErrorResponseTemplate());
	}
	return getErrorResponseTemplate(id);
};

const useErrorResponseTemplate = (id: number | "new", options = {}) => {
	return useQuery<ErrorResponseTemplate, Error>({
		queryKey: ["error-response-template", id],
		queryFn: () => fetchErrorResponseTemplate(id),
		staleTime: 60 * 1000,
		...options,
	});
};

const useSetErrorResponseTemplate = () => {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (values: ErrorResponseTemplate) =>
			values.id ? updateErrorResponseTemplate(values) : createErrorResponseTemplate(values),
		onMutate: (values: ErrorResponseTemplate) => {
			if (!values.id) return;
			const previousObject = queryClient.getQueryData(["error-response-template", values.id]);
			queryClient.setQueryData(["error-response-template", values.id], (old: ErrorResponseTemplate) => ({
				...old,
				...values,
			}));
			return () => queryClient.setQueryData(["error-response-template", values.id], previousObject);
		},
		onError: (_, __, rollback: any) => rollback?.(),
		onSuccess: async ({ id }: ErrorResponseTemplate) => {
			queryClient.invalidateQueries({ queryKey: ["error-response-template", id] });
			queryClient.invalidateQueries({ queryKey: ["error-response-templates"] });
			queryClient.invalidateQueries({ queryKey: ["proxy-hosts"] });
			queryClient.invalidateQueries({ queryKey: ["static-hosts"] });
			queryClient.invalidateQueries({ queryKey: ["audit-logs"] });
		},
	});
};

export { useErrorResponseTemplate, useSetErrorResponseTemplate };
