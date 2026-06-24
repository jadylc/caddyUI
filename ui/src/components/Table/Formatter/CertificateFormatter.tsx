import type { Certificate } from "src/api/backend";
import { T } from "src/locale";
import { certificateProvider, CertificateProviderLabel } from "./certificateProvider";

interface Props {
	certificate?: Certificate;
	certificateId?: number;
	sslForced?: boolean;
	meta?: Record<string, any>;
}

export function CertificateFormatter({ certificate, certificateId = 0, meta }: Props) {
	if (!certificate) {
		const bindings = Array.isArray(meta?.certificateBindings)
			? meta.certificateBindings
			: Array.isArray(meta?.certificate_bindings)
				? meta.certificate_bindings
				: [];
		const selectedProviders = Array.from(
			new Set(
				bindings
					.filter((binding: any) => binding?.mode === "selected" && binding?.provider)
					.map((binding: any) => binding.provider),
			),
		);
		if (selectedProviders.length > 0) {
			return (
				<>
					{selectedProviders.map((provider, index) => (
						<span key={provider}>
							{index > 0 ? " / " : null}
							<CertificateProviderLabel provider={provider as string} />
						</span>
					))}
				</>
			);
		}
		if (certificateId === -1 || meta?.certificateMode === "auto" || meta?.certificate_mode === "auto") {
			return <>Caddy 自动</>;
		}
		return <T id="http-only" />;
	}
	return <CertificateProviderLabel provider={certificateProvider(certificate) || certificate.niceName} />;
}
