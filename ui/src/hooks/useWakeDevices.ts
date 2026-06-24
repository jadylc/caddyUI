import { useQuery } from "@tanstack/react-query";
import { getWakeDevices, type WakeDevice } from "src/api/backend";

const fetchWakeDevices = () => getWakeDevices();

const useWakeDevices = (options = {}) => {
	return useQuery<WakeDevice[], Error>({
		queryKey: ["wake-devices"],
		queryFn: fetchWakeDevices,
		staleTime: 15 * 1000,
		refetchInterval: 15 * 1000,
		...options,
	});
};

export { fetchWakeDevices, useWakeDevices };
