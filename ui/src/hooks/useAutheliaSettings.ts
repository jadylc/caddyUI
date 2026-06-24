import type { AutheliaSettings } from "src/api/backend";
import { useForwardAuthSettings, useSetForwardAuthSettings } from "./useForwardAuthSettings";

const useAutheliaSettings = () => {
	return useForwardAuthSettings<AutheliaSettings>("authelia");
};

const useSetAutheliaSettings = () => {
	return useSetForwardAuthSettings<AutheliaSettings>("authelia");
};

export { useAutheliaSettings, useSetAutheliaSettings };
