import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createWakeDevice, getWakeDevice, type WakeDevice, updateWakeDevice } from "src/api/backend";

const fetchWakeDevice = (id: number | "new") => {
	if (id === "new") {
		return Promise.resolve({
			id: 0,
			createdOn: "",
			modifiedOn: "",
			ownerUserId: 0,
			name: "",
			macAddress: "",
			broadcastAddress: "255.255.255.255",
			port: 9,
			secureOn: "",
			host: "",
			description: "",
			meta: {},
			enabled: true,
		} as WakeDevice);
	}
	return getWakeDevice(id);
};

const useWakeDevice = (id: number | "new", options = {}) => {
	return useQuery<WakeDevice, Error>({
		queryKey: ["wake-device", id],
		queryFn: () => fetchWakeDevice(id),
		staleTime: 60 * 1000,
		...options,
	});
};

const useSetWakeDevice = () => {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (values: WakeDevice) => (values.id ? updateWakeDevice(values) : createWakeDevice(values)),
		onMutate: (values: WakeDevice) => {
			if (!values.id) return;
			const previousObject = queryClient.getQueryData(["wake-device", values.id]);
			queryClient.setQueryData(["wake-device", values.id], (old: WakeDevice) => ({ ...old, ...values }));
			return () => queryClient.setQueryData(["wake-device", values.id], previousObject);
		},
		onError: (_, __, rollback: any) => rollback?.(),
		onSuccess: async ({ id }: WakeDevice) => {
			queryClient.invalidateQueries({ queryKey: ["wake-device", id] });
			queryClient.invalidateQueries({ queryKey: ["wake-devices"] });
			queryClient.invalidateQueries({ queryKey: ["audit-logs"] });
		},
	});
};

export { useWakeDevice, useSetWakeDevice };
