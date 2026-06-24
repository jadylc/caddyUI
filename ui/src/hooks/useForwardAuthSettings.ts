import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getForwardAuthSettings, updateForwardAuthSettings, type ForwardAuthSettings } from "src/api/backend";
import type { ForwardAuthProviderID } from "src/forwardAuthProviders";

const useForwardAuthSettings = <T extends ForwardAuthSettings>(provider: ForwardAuthProviderID) => {
	return useQuery<T, Error>({
		queryKey: ["forward-auth-settings", provider],
		queryFn: () => getForwardAuthSettings<T>(provider),
		staleTime: 60 * 1000,
	});
};

const useSetForwardAuthSettings = <T extends ForwardAuthSettings>(provider: ForwardAuthProviderID) => {
	const queryClient = useQueryClient();
	return useMutation({
		mutationFn: (values: T) => updateForwardAuthSettings<T>(provider, values),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: ["forward-auth-settings", provider] });
			queryClient.invalidateQueries({ queryKey: ["proxy-hosts"] });
			queryClient.invalidateQueries({ queryKey: ["host-report"] });
		},
	});
};

export { useForwardAuthSettings, useSetForwardAuthSettings };
