export const forwardAuthProviders = {
	authelia: {
		id: "authelia",
		name: "Authelia",
		settingsPath: "/settings/authelia",
		settingsLabelId: "authelia.settings",
		descriptionId: "authelia.description",
		enabledId: "authelia.enabled",
		summaryEnabledId: "authelia.summary.enabled",
		summaryDisabledId: "authelia.summary.disabled",
		docsUrl: "https://github.com/authelia/authelia",
		upstreamPlaceholder: "http://authelia:9091",
		uriPlaceholder: "/api/authz/forward-auth",
		upstreamHelpId: "forward-auth.upstream.help.authelia",
		supportsFailOpen: true,
	},
	authentik: {
		id: "authentik",
		name: "authentik",
		settingsPath: "/settings/authentik",
		settingsLabelId: "authentik.settings",
		descriptionId: "authentik.description",
		enabledId: "authentik.enabled",
		summaryEnabledId: "authentik.summary.enabled",
		summaryDisabledId: "authentik.summary.disabled",
		docsUrl: "https://docs.goauthentik.io/add-secure-apps/providers/proxy/server_caddy/",
		upstreamPlaceholder: "http://authentik-server:9000",
		uriPlaceholder: "/outpost.goauthentik.io/auth/caddy",
		upstreamHelpId: "forward-auth.upstream.help.authentik",
		supportsFailOpen: false,
	},
} as const;

export type ForwardAuthProviderID = keyof typeof forwardAuthProviders;
