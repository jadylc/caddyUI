import { IconAlertTriangle } from "@tabler/icons-react";
import { Field, useFormikContext } from "formik";
import { useState } from "react";
import Select, { type ActionMeta } from "react-select";
import type { DNSProvider } from "src/api/backend";
import { useDnsProviders } from "src/hooks";
import { intl, T } from "src/locale";
import styles from "./DNSProviderFields.module.css";

interface DNSProviderOption {
	readonly value: string;
	readonly label: string;
	readonly credentials: string;
	readonly provider?: string;
	readonly credentialId?: string;
	readonly saved?: boolean;
}

interface Props {
	showBoundaryBox?: boolean;
	optional?: boolean;
}
export function DNSProviderFields({ showBoundaryBox = false, optional = false }: Props) {
	const { values, setFieldValue } = useFormikContext();
	const { data: dnsProviders, isLoading } = useDnsProviders();
	const [dnsProviderId, setDnsProviderId] = useState<string | null>(null);

	const v: any = values || {};

	const handleChange = (newValue: any, _actionMeta: ActionMeta<DNSProviderOption>) => {
		if (newValue?.credentialId) {
			setFieldValue("meta.credentialId", newValue.credentialId);
			setFieldValue("meta.dnsProvider", undefined);
			setFieldValue("meta.dnsProviderCredentials", undefined);
			setFieldValue("meta.cfToken", "");
			setFieldValue("meta.aliyunKey", "");
			setFieldValue("meta.aliyunSecret", "");
			setFieldValue("meta.dnspodToken", "");
			setFieldValue("meta.heApiKey", "");
		} else {
			setFieldValue("meta.credentialId", undefined);
			setFieldValue("meta.dnsProvider", newValue?.provider || newValue?.value);
			setFieldValue("meta.dnsProviderCredentials", "");
			setFieldValue("meta.cfToken", "");
			setFieldValue("meta.aliyunKey", "");
			setFieldValue("meta.aliyunSecret", "");
			setFieldValue("meta.dnspodToken", "");
			setFieldValue("meta.heApiKey", "");
		}
		setDnsProviderId(newValue?.value);
	};

	const options: DNSProviderOption[] =
		dnsProviders?.map((p: DNSProvider) => ({
			value: p.id,
			label: p.name,
			credentials: p.credentials,
			provider: p.provider || p.id,
			credentialId: p.credentialId,
			saved: p.saved,
		})) || [];
	const selectedOption =
		options.find((o) => o.credentialId && o.credentialId === (v.meta?.credentialId || v.meta?.credential_id)) ||
		options.find((o) => !o.credentialId && o.provider === v.meta?.dnsProvider) ||
		null;
	const isSavedCredentialSelected = !!selectedOption?.credentialId || dnsProviderId?.startsWith("saved:");
	const activeProvider = selectedOption?.provider || v.meta?.dnsProvider;

	const updateCredentialField = (field: string, value: string) => {
		setFieldValue(`meta.${field}`, value);
		const next = { ...(v.meta || {}), [field]: value };
		if (activeProvider === "cloudflare") {
			setFieldValue("meta.dnsProviderCredentials", `CLOUDFLARE_API_TOKEN=${next.cfToken || ""}`);
		}
		if (activeProvider === "alidns") {
			setFieldValue(
				"meta.dnsProviderCredentials",
				`ALICLOUD_ACCESS_KEY=${next.aliyunKey || ""}\nALICLOUD_SECRET_KEY=${next.aliyunSecret || ""}`,
			);
		}
		if (activeProvider === "dnspod") {
			setFieldValue("meta.dnsProviderCredentials", `DNSPOD_TOKEN=${next.dnspodToken || ""}`);
		}
		if (activeProvider === "he") {
			setFieldValue("meta.dnsProviderCredentials", `HE_API_KEY=${next.heApiKey || ""}`);
		}
	};

	return (
		<div className={showBoundaryBox ? styles.dnsChallengeWarning : undefined}>
			<p className="text-warning">
				<IconAlertTriangle size={16} className="me-1" />
				<T id="certificates.dns.warning" />
			</p>
			{optional ? <p className="text-muted small">不选择时复用原 DNS 凭据。</p> : null}

			<Field name="meta.dnsProvider">
				{({ field }: any) => (
					<div className="row">
						<label htmlFor="dnsProvider" className="form-label">
							<T id="certificates.dns.provider" />
						</label>
						<Select
							className="react-select-container"
							classNamePrefix="react-select"
							name={field.name}
							id="dnsProvider"
							closeMenuOnSelect={true}
							isClearable={false}
							placeholder={intl.formatMessage({ id: "certificates.dns.provider.placeholder" })}
							isLoading={isLoading}
							isSearchable
							onChange={handleChange}
							options={options}
							value={selectedOption}
							menuPortalTarget={document.body}
						/>
					</div>
				)}
			</Field>

			{activeProvider && !isSavedCredentialSelected ? (
				<>
					{activeProvider === "cloudflare" ? (
						<div className="mt-3">
							<label htmlFor="cfToken" className="form-label">
								Cloudflare API Token
							</label>
							<input
								id="cfToken"
								type="password"
								className="form-control"
								autoComplete="off"
								value={v.meta?.cfToken || ""}
								onChange={(event) => updateCredentialField("cfToken", event.target.value)}
							/>
						</div>
					) : null}
					{activeProvider === "alidns" ? (
						<div className="row mt-3">
							<div className="col-md-6">
								<label htmlFor="aliyunKey" className="form-label">
									AccessKey ID
								</label>
								<input
									id="aliyunKey"
									type="text"
									className="form-control"
									autoComplete="off"
									value={v.meta?.aliyunKey || ""}
									onChange={(event) => updateCredentialField("aliyunKey", event.target.value)}
								/>
							</div>
							<div className="col-md-6">
								<label htmlFor="aliyunSecret" className="form-label">
									AccessKey Secret
								</label>
								<input
									id="aliyunSecret"
									type="password"
									className="form-control"
									autoComplete="off"
									value={v.meta?.aliyunSecret || ""}
									onChange={(event) => updateCredentialField("aliyunSecret", event.target.value)}
								/>
							</div>
						</div>
					) : null}
					{activeProvider === "dnspod" ? (
						<div className="mt-3">
							<label htmlFor="dnspodToken" className="form-label">
								DNSPod Token
							</label>
							<input
								id="dnspodToken"
								type="password"
								className="form-control"
								autoComplete="off"
								placeholder="APP_ID,APP_TOKEN"
								value={v.meta?.dnspodToken || ""}
								onChange={(event) => updateCredentialField("dnspodToken", event.target.value)}
							/>
							<small className="text-muted">APP_ID,APP_TOKEN</small>
						</div>
					) : null}
					{activeProvider === "he" ? (
						<div className="mt-3">
							<label htmlFor="heApiKey" className="form-label">
								Hurricane Electric API Key
							</label>
							<input
								id="heApiKey"
								type="password"
								className="form-control"
								autoComplete="off"
								value={v.meta?.heApiKey || ""}
								onChange={(event) => updateCredentialField("heApiKey", event.target.value)}
							/>
						</div>
					) : null}
					<div>
						<small className="text-danger">
							<T id="certificates.dns.credentials-warning" />
						</small>
					</div>
					<Field name="meta.propagationSeconds">
						{({ field }: any) => (
							<div className="mt-3">
								<label htmlFor="propagationSeconds" className="form-label">
									<T id="certificates.dns.propagation-seconds" />
								</label>
								<input
									id="propagationSeconds"
									type="number"
									className="form-control"
									min={0}
									max={7200}
									{...field}
								/>
								<small className="text-muted">
									<T id="certificates.dns.propagation-seconds-note" />
								</small>
							</div>
						)}
					</Field>
				</>
			) : null}
		</div>
	);
}
