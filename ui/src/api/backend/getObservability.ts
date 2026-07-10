import * as api from "./base";
import type { ObservabilityOverview } from "./models";

export async function getObservability(): Promise<ObservabilityOverview> {
	return await api.get({
		url: "/caddy/observability",
	});
}
