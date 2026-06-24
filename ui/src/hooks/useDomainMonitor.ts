import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createDomainMonitor, getDomainMonitor, type DomainMonitor, updateDomainMonitor } from "src/api/backend";

const fetchDomainMonitor = (id: number | "new") => {
	if (id === "new") {
		return Promise.resolve({
			id: 0,
			createdOn: "",
			modifiedOn: "",
			ownerUserId: 0,
			name: "",
			domainNames: [],
			checkSsl: false,
			checkDns: true,
			checkDomain: true,
			checkInterval: "24h",
			thresholdDays: 30,
			resolvers: [],
			meta: {},
			enabled: true,
		} as DomainMonitor);
	}
	return getDomainMonitor(id);
};

const useDomainMonitor = (id: number | "new", options = {}) => {
	return useQuery<DomainMonitor, Error>({
		queryKey: ["domain-monitor", id],
		queryFn: () => fetchDomainMonitor(id),
		staleTime: 60 * 1000,
		...options,
	});
};

const useSetDomainMonitor = () => {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (values: DomainMonitor) => (values.id ? updateDomainMonitor(values) : createDomainMonitor(values)),
		onMutate: (values: DomainMonitor) => {
			if (!values.id) return;
			const previousObject = queryClient.getQueryData(["domain-monitor", values.id]);
			queryClient.setQueryData(["domain-monitor", values.id], (old: DomainMonitor) => ({ ...old, ...values }));
			return () => queryClient.setQueryData(["domain-monitor", values.id], previousObject);
		},
		onError: (_, __, rollback: any) => rollback?.(),
		onSuccess: async ({ id }: DomainMonitor) => {
			queryClient.invalidateQueries({ queryKey: ["domain-monitor", id] });
			queryClient.invalidateQueries({ queryKey: ["domain-monitor"] });
			queryClient.invalidateQueries({ queryKey: ["audit-logs"] });
		},
	});
};

export { useDomainMonitor, useSetDomainMonitor };
