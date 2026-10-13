import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createStaticHost, getStaticHost, type StaticHost, updateStaticHost } from "src/api/backend";

const fetchStaticHost = (id: number | "new") => {
	if (id === "new") {
		return Promise.resolve({
			id: 0,
			createdOn: "",
			modifiedOn: "",
			ownerUserId: 0,
			serviceName: "",
			domainNames: [],
			listenPort: 0,
			listenPorts: [],
			rootPath: "",
			mode: "static",
			indexFile: "/index.html",
			directoryBrowsing: false,
			rootRedirectPath: "",
			rootRedirectCode: 302,
			accessListId: 0,
			certificateId: 0,
			sslForced: false,
			meta: {},
			http2Support: false,
			enabled: true,
			hstsEnabled: false,
			hstsSubdomains: false,
			waf: {
				enabled: false,
				mode: "detection_only",
			},
		} as StaticHost);
	}
	return getStaticHost(id, ["owner"]);
};

const useStaticHost = (id: number | "new", options = {}) => {
	return useQuery<StaticHost, Error>({
		queryKey: ["static-host", id],
		queryFn: () => fetchStaticHost(id),
		staleTime: 60 * 1000,
		...options,
	});
};

const useSetStaticHost = () => {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (values: StaticHost) => (values.id ? updateStaticHost(values) : createStaticHost(values)),
		onMutate: (values: StaticHost) => {
			if (!values.id) {
				return;
			}
			const previousObject = queryClient.getQueryData(["static-host", values.id]);
			queryClient.setQueryData(["static-host", values.id], (old: StaticHost) => ({
				...old,
				...values,
			}));
			return () => queryClient.setQueryData(["static-host", values.id], previousObject);
		},
		onError: (_, __, rollback: any) => rollback(),
		onSuccess: async ({ id }: StaticHost) => {
			queryClient.invalidateQueries({ queryKey: ["static-host", id] });
			queryClient.invalidateQueries({ queryKey: ["static-hosts"] });
			queryClient.invalidateQueries({ queryKey: ["access-lists"] });
			queryClient.invalidateQueries({ queryKey: ["audit-logs"] });
			queryClient.invalidateQueries({ queryKey: ["host-report"] });
			queryClient.invalidateQueries({ queryKey: ["certificates"] });
		},
	});
};

export { useSetStaticHost, useStaticHost };
