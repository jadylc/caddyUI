import { IconAlertTriangle } from "@tabler/icons-react";
import { Field, useFormikContext } from "formik";
import { useState } from "react";
import Select, { type ActionMeta } from "react-select";
import type { DNSProvider, ProviderField } from "src/api/backend";
import { useDnsProviders } from "src/hooks";
import { intl, T } from "src/locale";
import { ProviderCredentialFields } from "./ProviderCredentialFields";
import styles from "./DNSProviderFields.module.css";

interface DNSProviderOption {
	readonly value: string;
	readonly label: string;
	readonly credentials: string;
	readonly provider?: string;
	readonly credentialId?: string;
	readonly saved?: boolean;
	readonly fields?: ProviderField[];
}

interface Props {
	showBoundaryBox?: boolean;
	optional?: boolean;
}

// 历史 meta 字段名：旧版本把单个密钥直接写进 meta，这里在编辑旧证书时回显。
const legacySecretKeys: Record<string, string> = {
	cf_token: "cfToken",
	aliyun_key: "aliyunKey",
	aliyun_secret: "aliyunSecret",
	dnspod_token: "dnspodToken",
	he_api_key: "heApiKey",
};

export function DNSProviderFields({ showBoundaryBox = false, optional = false }: Props) {
	const { values, setFieldValue } = useFormikContext();
	const { data: dnsProviders, isLoading } = useDnsProviders();
	const [dnsProviderId, setDnsProviderId] = useState<string | null>(null);

	const v: any = values || {};
	const meta: any = v.meta || {};

	// 从历史 meta 字段补齐密钥，保证旧证书仍能回显。
	const legacySecrets: Record<string, string> = {};
	for (const [key, metaKey] of Object.entries(legacySecretKeys)) {
		const value = meta[metaKey];
		if (typeof value === "string" && value) {
			legacySecrets[key] = value;
		}
	}

	const seedSecrets = (fields?: ProviderField[]) => {
		const out: Record<string, string> = {};
		for (const field of fields || []) {
			if (legacySecrets[field.key]) {
				out[field.key] = legacySecrets[field.key];
			}
		}
		return out;
	};

	const handleChange = (newValue: any, _actionMeta: ActionMeta<DNSProviderOption>) => {
		if (newValue?.credentialId) {
			setFieldValue("meta.credentialId", newValue.credentialId);
			setFieldValue("meta.dnsProvider", undefined);
			setFieldValue("meta.dnsProviderCredentials", undefined);
			setFieldValue("meta.credentialSecrets", undefined);
		} else {
			setFieldValue("meta.credentialId", undefined);
			setFieldValue("meta.dnsProvider", newValue?.provider || newValue?.value);
			setFieldValue("meta.dnsProviderCredentials", "");
			setFieldValue("meta.credentialSecrets", seedSecrets(newValue?.fields));
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
			fields: p.fields,
		})) || [];
	const selectedOption =
		options.find((o) => o.credentialId && o.credentialId === (v.meta?.credentialId || v.meta?.credential_id)) ||
		options.find((o) => !o.credentialId && o.provider === v.meta?.dnsProvider) ||
		null;
	const isSavedCredentialSelected = !!selectedOption?.credentialId || dnsProviderId?.startsWith("saved:");
	const activeProvider = selectedOption?.provider || v.meta?.dnsProvider;
	const activeFields = selectedOption?.fields;
	const secretValues: Record<string, string> = {
		...seedSecrets(activeFields),
		...((meta.credentialSecrets as Record<string, string>) || {}),
	};

	// 按字段 schema 生成后端凭据文本（KEY=VALUE），与历史 dns_provider_credentials 契约一致。
	const buildCredentialsText = (fields: ProviderField[], secrets: Record<string, string>) => {
		return fields
			.map((field) => (field.env_name ? `${field.env_name}=${secrets[field.key] || ""}` : ""))
			.filter((line) => line !== "")
			.join("\n");
	};

	const updateCredentialField = (key: string, value: string) => {
		const next = { ...secretValues, [key]: value };
		setFieldValue("meta.credentialSecrets", next);
		if (activeFields) {
			setFieldValue("meta.dnsProviderCredentials", buildCredentialsText(activeFields, next));
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
					<ProviderCredentialFields
						fields={activeFields}
						values={secretValues}
						idPrefix="certificateDns"
						onChange={updateCredentialField}
					/>
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
