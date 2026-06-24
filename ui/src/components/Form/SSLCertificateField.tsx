import { IconShield } from "@tabler/icons-react";
import { Field, useFormikContext } from "formik";
import Select, { type ActionMeta, components, type OptionProps } from "react-select";
import type { Certificate } from "src/api/backend";
import { certificateProvider, certificateProviderText } from "src/components";
import { useCertificates } from "src/hooks";
import { formatDateTime, intl, T } from "src/locale";

interface CertOption {
	readonly value: number | "new";
	readonly label: string;
	readonly subLabel: string;
	readonly icon: React.ReactNode;
}

const Option = (props: OptionProps<CertOption>) => {
	return (
		<components.Option {...props}>
			<div className="flex-fill">
				<div className="font-weight-medium">
					{props.data.icon} <strong>{props.data.label}</strong>
				</div>
				<div className="text-secondary mt-1 ps-3">{props.data.subLabel}</div>
			</div>
		</components.Option>
	);
};

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

const certificateMatchesDomains = (cert: Certificate, domains: string[]) => {
	const normalizedDomains = domains.map((d) => d.trim()).filter(Boolean);
	if (normalizedDomains.length === 0) return true;
	return normalizedDomains.every((domain) =>
		cert.domainNames.some((certDomain) => certCoversDomain(certDomain, domain)),
	);
};

interface Props {
	id?: string;
	name?: string;
	label?: string;
	required?: boolean;
	allowNew?: boolean;
	forHttp?: boolean; // the sslForced, http2Support, hstsEnabled, hstsSubdomains fields
}
export function SSLCertificateField({
	name = "certificateId",
	label = "ssl-certificate",
	id = "certificateId",
	required,
	allowNew,
	forHttp = true,
}: Props) {
	const { isLoading, isError, error, data } = useCertificates();
	const { values, setFieldValue } = useFormikContext();
	const v: any = values || {};
	const currentDomains = Array.isArray(v.domainNames) ? v.domainNames : [];

	const handleChange = (newValue: any, _actionMeta: ActionMeta<CertOption>) => {
		setFieldValue(name, newValue?.value);
		const { sslForced, http2Support, hstsEnabled, hstsSubdomains } = v;
		if (forHttp && !newValue?.value) {
			sslForced && setFieldValue("sslForced", false);
			http2Support && setFieldValue("http2Support", false);
			hstsEnabled && setFieldValue("hstsEnabled", false);
			hstsSubdomains && setFieldValue("hstsSubdomains", false);
		}
		if (newValue?.value !== "new") {
			setFieldValue("meta.dnsChallenge", undefined);
			setFieldValue("meta.dnsProvider", undefined);
			setFieldValue("meta.dnsProviderCredentials", undefined);
			setFieldValue("meta.credentialId", undefined);
			setFieldValue("meta.propagationSeconds", undefined);
			setFieldValue("meta.issuerProvider", undefined);
			setFieldValue("meta.caDirectory", undefined);
			setFieldValue("meta.eabKeyId", undefined);
			setFieldValue("meta.eabMacKey", undefined);
			setFieldValue("meta.keyType", undefined);
			setFieldValue("meta.cfToken", undefined);
			setFieldValue("meta.aliyunKey", undefined);
			setFieldValue("meta.aliyunSecret", undefined);
			setFieldValue("meta.dnspodToken", undefined);
			setFieldValue("meta.heApiKey", undefined);
		}
	};

	const options: CertOption[] =
		data
			?.filter((cert: Certificate) => certificateMatchesDomains(cert, currentDomains))
			.map((cert: Certificate) => ({
				value: cert.id,
				label: cert.niceName,
				subLabel: `${certificateProviderText(certificateProvider(cert))} — ${intl.formatMessage({ id: "expires.on" }, { date: cert.expiresOn ? formatDateTime(cert.expiresOn) : "N/A" })}`,
				icon: <IconShield size={14} className="text-pink" />,
			})) || [];

	// Prepend the Add New option
	if (allowNew) {
		options?.unshift({
			value: "new",
			label: intl.formatMessage({ id: "certificates.request.title" }),
			subLabel: intl.formatMessage({ id: "certificates.request.subtitle" }),
			icon: <IconShield size={14} className="text-lime" />,
		});
		options?.unshift({
			value: -1,
			label: "Caddy 自动",
			subLabel: "按每个域名单独自动管理证书，适合多个域名使用不同证书",
			icon: <IconShield size={14} className="text-azure" />,
		});
	}

	// Prepend the None option
	if (!required) {
		options?.unshift({
			value: 0,
			label: intl.formatMessage({ id: "certificate.none.title" }),
			subLabel: forHttp
				? intl.formatMessage({ id: "certificate.none.subtitle.for-http" })
				: intl.formatMessage({ id: "certificate.none.subtitle" }),
			icon: <IconShield size={14} className="text-red" />,
		});
	}

	return (
		<Field name={name}>
			{({ field, form }: any) => (
				<div className="mb-3">
					<label className="form-label" htmlFor={id}>
						<T id={label} />
					</label>
					{isLoading ? <div className="placeholder placeholder-lg col-12 my-3 placeholder-glow" /> : null}
					{isError ? <div className="invalid-feedback">{`${error}`}</div> : null}
					{!isLoading && !isError ? (
						<Select
							className="react-select-container"
							classNamePrefix="react-select"
							value={options.find((o) => o.value === field.value) || options[0]}
							options={options}
							components={{ Option }}
							styles={{
								option: (base) => ({
									...base,
									height: "100%",
								}),
							}}
							onChange={handleChange}
							menuPortalTarget={document.body}
						/>
					) : null}
					{form.errors[field.name] ? (
						<div className="invalid-feedback">
							{form.errors[field.name] && form.touched[field.name] ? form.errors[field.name] : null}
						</div>
					) : null}
				</div>
			)}
		</Field>
	);
}
