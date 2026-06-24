import type { Certificate } from "src/api/backend";
import { intl, T } from "src/locale";

export const certificateProvider = (certificate?: Certificate) =>
	certificate?.meta?.issuerProvider || certificate?.meta?.issuer_provider || certificate?.provider || "";

export const certificateProviderText = (provider?: string) => {
	switch (provider) {
		case "google":
			return "Google Trust Services";
		case "letsencrypt":
			return intl.formatMessage({ id: "lets-encrypt" });
		case "letsencrypt-staging":
			return "Let's Encrypt Staging";
		case "zerossl":
			return "ZeroSSL";
		case "other":
			return intl.formatMessage({ id: "certificates.custom" });
		case "auto":
			return "Caddy 自动";
		default:
			return provider || "Unknown";
	}
};

export const CertificateProviderLabel = ({ provider }: { provider?: string }) => {
	switch (provider) {
		case "letsencrypt":
			return <T id="lets-encrypt" />;
		case "other":
			return <T id="certificates.custom" />;
		default:
			return <>{certificateProviderText(provider)}</>;
	}
};
