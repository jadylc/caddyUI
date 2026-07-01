import { useFormikContext } from "formik";
import { useCallback, useEffect, useMemo } from "react";
import type { Certificate, CertificateBinding } from "src/api/backend";
import { certificateProvider, certificateProviderText } from "src/components";
import { useCertificates } from "src/hooks";
import { formatDateTime, intl } from "src/locale";

const normalizeDomain = (domain: string) => domain.trim().toLowerCase().replace(/^\*\./, "");

const wildcardMatches = (wildcard: string, domain: string) => {
	const parent = normalizeDomain(wildcard);
	const host = normalizeDomain(domain);
	if (!host.endsWith(`.${parent}`)) return false;
	return host.split(".").length === parent.split(".").length + 1;
};

const certCoversDomain = (certDomain: string, domain: string) => {
	const normalizedCertDomain = certDomain.trim().toLowerCase();
	const normalizedDomain = domain.trim().toLowerCase();
	if (!normalizedCertDomain || !normalizedDomain) return false;
	if (normalizedCertDomain.startsWith("*.")) {
		return wildcardMatches(normalizedCertDomain, normalizedDomain);
	}
	return normalizedCertDomain === normalizedDomain;
};

const showHTTPNewCertificate = async (domain: string, onCreated: (certificate: Certificate) => void) => {
	const { showHTTPCertificateModal } = await import("src/modals/HTTPCertificateModal");
	showHTTPCertificateModal({ domainNames: [domain] }, onCreated);
};

const showDNSNewCertificate = async (domain: string, onCreated: (certificate: Certificate) => void) => {
	const { showDNSCertificateModal } = await import("src/modals/DNSCertificateModal");
	showDNSCertificateModal({ domainNames: [domain] }, onCreated);
};

const metaValue = (meta: Record<string, any> | undefined, snake: string, camel: string) =>
	meta?.[camel] ?? meta?.[snake];

const bindingFromCertificate = (domain: string, cert: Certificate) => {
	const issuerProvider = certificateProvider(cert);
	return {
		domain,
		mode: "selected",
		certificateId: cert.id,
		certificateDomain: cert.domainNames?.[0] || cert.niceName,
		provider: issuerProvider,
		niceName: cert.niceName,
		challengePref: metaValue(cert.meta, "sign_method", "signMethod") === "DNS-01" ? "dns" : "http",
		credentialId: metaValue(cert.meta, "credential_id", "credentialId") || "",
		issuer: {
			provider: issuerProvider,
			caDirectory: metaValue(cert.meta, "ca_directory", "caDirectory") || "",
			eabKeyId: metaValue(cert.meta, "eab_key_id", "eabKeyId") || "",
			eabMacKey: metaValue(cert.meta, "eab_mac_key", "eabMacKey") || "",
			zerosslApiKey: metaValue(cert.meta, "zerossl_api_key", "zerosslApiKey") || "",
		},
	};
};

export function DomainCertificateBindingsField() {
	const { data: certificates = [], isLoading } = useCertificates();
	const { values, setFieldValue } = useFormikContext<any>();
	const domainNames = useMemo(
		() =>
			Array.isArray(values.domainNames) ? values.domainNames.map((d: string) => d.trim()).filter(Boolean) : [],
		[values.domainNames],
	);
	const currentBindings: CertificateBinding[] = useMemo(
		() => (Array.isArray(values.meta?.certificateBindings) ? values.meta.certificateBindings : []),
		[values.meta?.certificateBindings],
	);

	const syncCertificateId = useCallback(
		(bindings: CertificateBinding[]) => {
			const hasSelected = bindings.some((b) => b.mode === "selected");
			setFieldValue("certificateId", hasSelected ? -1 : 0);
		},
		[setFieldValue],
	);

	useEffect(() => {
		const next = domainNames.map((domain: string) => {
			const existing = currentBindings.find((binding) => binding.domain === domain);
			return existing || { domain, mode: "auto" };
		});
		if (JSON.stringify(next) !== JSON.stringify(currentBindings)) {
			setFieldValue("meta.certificateBindings", next);
			syncCertificateId(next);
		}
	}, [currentBindings, domainNames, setFieldValue, syncCertificateId]);

	const updateBinding = (domain: string, rawValue: string) => {
		const certificateId = Number(rawValue || 0);
		const selectedCert = certificates.find((cert: Certificate) => cert.id === certificateId);
		const next = domainNames.map((item: string) => {
			if (item !== domain) {
				return currentBindings.find((binding) => binding.domain === item) || { domain: item, mode: "auto" };
			}
			if (certificateId > 0 && selectedCert) {
				return bindingFromCertificate(domain, selectedCert);
			}
			return { domain, mode: "auto" };
		});
		syncCertificateId(next);
		setFieldValue("meta.certificateBindings", next);
	};

	const applyCreatedCertificate = (domain: string, cert: Certificate) => {
		const next = domainNames.map((item: string) => {
			if (item !== domain) {
				return currentBindings.find((binding) => binding.domain === item) || { domain: item, mode: "auto" };
			}
			return bindingFromCertificate(domain, cert);
		});
		syncCertificateId(next);
		setFieldValue("meta.certificateBindings", next);
	};

	if (domainNames.length === 0) {
		return null;
	}

	return (
		<div className="mb-3">
			<div className="form-label">域名证书绑定</div>
			<div className="table-responsive border rounded">
				<table className="table table-vcenter mb-0">
					<thead>
						<tr>
							<th>域名</th>
							<th>当前证书</th>
							<th className="w-1">操作</th>
						</tr>
					</thead>
					<tbody>
						{domainNames.map((domain: string) => {
							const binding = currentBindings.find((item) => item.domain === domain);
							const matchingCerts = certificates.filter((cert: Certificate) =>
								cert.domainNames.some((certDomain) => certCoversDomain(certDomain, domain)),
							);
							const selectedValue =
								binding?.mode === "selected" ? String(binding.certificateId || 0) : "0";
							const selectedCert = matchingCerts.find((cert) => cert.id === binding?.certificateId);
							return (
								<tr key={domain}>
									<td className="text-nowrap">{domain}</td>
									<td>
										{matchingCerts.length > 0 ? (
											<select
												className="form-select"
												value={selectedValue}
												disabled={isLoading}
												onChange={(event) => updateBinding(domain, event.target.value)}
											>
												<option value="0">未绑定证书</option>
												{matchingCerts.map((cert) => (
													<option key={cert.id} value={cert.id}>
														{cert.niceName} ·{" "}
														{certificateProviderText(certificateProvider(cert))} ·{" "}
														{cert.expiresOn
															? intl.formatMessage(
																	{ id: "expires.on" },
																	{ date: formatDateTime(cert.expiresOn) },
																)
															: "N/A"}
													</option>
												))}
											</select>
										) : (
											<div className="form-control-plaintext text-secondary">暂无可用证书</div>
										)}
										<div className="text-secondary small mt-1">
											{selectedCert
												? `已绑定：${selectedCert.niceName} · ${certificateProviderText(certificateProvider(selectedCert))}`
												: "未绑定时不会强行套用其他域名的证书；请申请或选择一张匹配证书。"}
										</div>
									</td>
									<td className="text-nowrap">
										<div className="btn-group flex-shrink-0">
											<button
												type="button"
												className="btn btn-outline-secondary"
												onClick={() =>
													showHTTPNewCertificate(domain, (cert) =>
														applyCreatedCertificate(domain, cert),
													)
												}
											>
												HTTP 申请
											</button>
											<button
												type="button"
												className="btn btn-outline-secondary"
												onClick={() =>
													showDNSNewCertificate(domain, (cert) =>
														applyCreatedCertificate(domain, cert),
													)
												}
											>
												DNS 申请
											</button>
										</div>
									</td>
								</tr>
							);
						})}
					</tbody>
				</table>
			</div>
		</div>
	);
}
