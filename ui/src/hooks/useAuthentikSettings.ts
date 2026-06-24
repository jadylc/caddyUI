import type { AuthentikSettings } from "src/api/backend";
import { useForwardAuthSettings, useSetForwardAuthSettings } from "./useForwardAuthSettings";

const useAuthentikSettings = () => {
	return useForwardAuthSettings<AuthentikSettings>("authentik");
};

const useSetAuthentikSettings = () => {
	return useSetForwardAuthSettings<AuthentikSettings>("authentik");
};

export { useAuthentikSettings, useSetAuthentikSettings };
