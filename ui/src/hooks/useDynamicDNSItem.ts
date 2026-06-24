import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createDynamicDNSItem, getDynamicDNSItem, type DynamicDNS, updateDynamicDNSItem } from "src/api/backend";

const fetchDynamicDNSItem = (id: number | "new") => {
	if (id === "new") {
		return Promise.resolve({
			id: 0,
			createdOn: "",
			modifiedOn: "",
			ownerUserId: 0,
			name: "",
			domainNames: [],
			credentialId: "",
			ipv4: true,
			ipv6: false,
			checkInterval: "5m",
			ttl: "",
			resolvers: [],
			ipServiceUrl: "",
			meta: {},
			enabled: true,
		} as DynamicDNS);
	}
	return getDynamicDNSItem(id);
};

const useDynamicDNSItem = (id: number | "new", options = {}) => {
	return useQuery<DynamicDNS, Error>({
		queryKey: ["dynamic-dns", id],
		queryFn: () => fetchDynamicDNSItem(id),
		staleTime: 60 * 1000,
		...options,
	});
};

const useSetDynamicDNSItem = () => {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (values: DynamicDNS) => (values.id ? updateDynamicDNSItem(values) : createDynamicDNSItem(values)),
		onMutate: (values: DynamicDNS) => {
			if (!values.id) return;
			const previousObject = queryClient.getQueryData(["dynamic-dns", values.id]);
			queryClient.setQueryData(["dynamic-dns", values.id], (old: DynamicDNS) => ({ ...old, ...values }));
			return () => queryClient.setQueryData(["dynamic-dns", values.id], previousObject);
		},
		onError: (_, __, rollback: any) => rollback?.(),
		onSuccess: async ({ id }: DynamicDNS) => {
			queryClient.invalidateQueries({ queryKey: ["dynamic-dns", id] });
			queryClient.invalidateQueries({ queryKey: ["dynamic-dns"] });
			queryClient.invalidateQueries({ queryKey: ["audit-logs"] });
		},
	});
};

export { useDynamicDNSItem, useSetDynamicDNSItem };
