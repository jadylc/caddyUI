import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
	createHeaderPolicyTemplate,
	getHeaderPolicyTemplate,
	type HeaderPolicyTemplate,
	updateHeaderPolicyTemplate,
} from "src/api/backend";

const defaultHeaderPolicyTemplate = (): HeaderPolicyTemplate => ({
	id: 0,
	createdOn: "",
	modifiedOn: "",
	ownerUserId: 0,
	name: "",
	description: "",
	category: "custom",
	enabled: true,
	builtin: false,
	headers: [
		{ action: "set", name: "X-Content-Type-Options", value: "nosniff" },
		{ action: "set", name: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
	],
	meta: {},
	hostCount: 0,
});

const fetchHeaderPolicyTemplate = (id: number | "new") => {
	if (id === "new") {
		return Promise.resolve(defaultHeaderPolicyTemplate());
	}
	return getHeaderPolicyTemplate(id);
};

const useHeaderPolicyTemplate = (id: number | "new", options = {}) => {
	return useQuery<HeaderPolicyTemplate, Error>({
		queryKey: ["header-policy-template", id],
		queryFn: () => fetchHeaderPolicyTemplate(id),
		staleTime: 60 * 1000,
		...options,
	});
};

const useSetHeaderPolicyTemplate = () => {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (values: HeaderPolicyTemplate) =>
			values.id ? updateHeaderPolicyTemplate(values) : createHeaderPolicyTemplate(values),
		onMutate: (values: HeaderPolicyTemplate) => {
			if (!values.id) return;
			const previousObject = queryClient.getQueryData(["header-policy-template", values.id]);
			queryClient.setQueryData(["header-policy-template", values.id], (old: HeaderPolicyTemplate) => ({
				...old,
				...values,
			}));
			return () => queryClient.setQueryData(["header-policy-template", values.id], previousObject);
		},
		onError: (_, __, rollback: any) => rollback?.(),
		onSuccess: async ({ id }: HeaderPolicyTemplate) => {
			queryClient.invalidateQueries({ queryKey: ["header-policy-template", id] });
			queryClient.invalidateQueries({ queryKey: ["header-policy-templates"] });
			queryClient.invalidateQueries({ queryKey: ["proxy-hosts"] });
			queryClient.invalidateQueries({ queryKey: ["static-hosts"] });
			queryClient.invalidateQueries({ queryKey: ["audit-logs"] });
		},
	});
};

export { useHeaderPolicyTemplate, useSetHeaderPolicyTemplate };
