import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getSystemSettings, updateSystemSettings, type SystemSettings } from "src/api/backend";

const useSystemSettings = () => {
	return useQuery<SystemSettings, Error>({
		queryKey: ["system-settings"],
		queryFn: getSystemSettings,
		staleTime: 60 * 1000,
	});
};

const useSetSystemSettings = () => {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (values: SystemSettings) => updateSystemSettings(values),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: ["system-settings"] });
		},
	});
};

export { useSystemSettings, useSetSystemSettings };
