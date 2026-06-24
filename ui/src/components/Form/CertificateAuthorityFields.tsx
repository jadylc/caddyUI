import { Field, useFormikContext } from "formik";
import { useEffect } from "react";
import type { ChangeEvent } from "react";
import { useCertificateAuthorities } from "src/hooks";

interface Props {
	name?: string;
	metaPrefix?: string;
}

export function CertificateAuthorityFields({ name = "provider", metaPrefix = "meta" }: Props) {
	const { values, setFieldValue } = useFormikContext();
	const { data: authorities = [] } = useCertificateAuthorities();
	const v: any = values || {};
	const current = name.split(".").reduce((acc: any, key) => acc?.[key], v) || "auto";
	const meta = v?.meta || {};
	const caField = `${metaPrefix}.caDirectory`;
	const eabKeyIDField = `${metaPrefix}.eabKeyId`;
	const eabMACKeyField = `${metaPrefix}.eabMacKey`;
	const eabKeyIDValue = meta.eabKeyId ?? meta.eab_key_id ?? "";
	const eabMACKeyValue = meta.eabMacKey ?? meta.eab_mac_key ?? "";
	const selectedAuthority = authorities.find((item) => item.id === current || item.issuerProvider === current);
	const savedEABKeyID = selectedAuthority?.eabKeyId ?? "";
	const savedEABMACKey = selectedAuthority?.eabMacKey ?? "";
	const savedCADirectory = selectedAuthority?.caDirectory ?? "";

	useEffect(() => {
		if (current !== "google" && current !== "custom") {
			return;
		}
		if (!eabKeyIDValue && savedEABKeyID) {
			setFieldValue(eabKeyIDField, savedEABKeyID, false);
		}
		if (!eabMACKeyValue && savedEABMACKey) {
			setFieldValue(eabMACKeyField, savedEABMACKey, false);
		}
		if (current === "custom" && !meta.caDirectory && savedCADirectory) {
			setFieldValue(caField, savedCADirectory, false);
		}
	}, [
		caField,
		current,
		eabKeyIDField,
		eabKeyIDValue,
		eabMACKeyField,
		eabMACKeyValue,
		meta.caDirectory,
		savedCADirectory,
		savedEABKeyID,
		savedEABMACKey,
		setFieldValue,
	]);

	const handleChange = (e: ChangeEvent<HTMLSelectElement>) => {
		const provider = e.target.value;
		setFieldValue(name, provider);
		setFieldValue(`${metaPrefix}.issuerProvider`, provider);
		if (provider !== "custom") {
			setFieldValue(caField, undefined);
		}
		if (provider !== "google" && provider !== "custom") {
			setFieldValue(eabKeyIDField, undefined);
			setFieldValue(eabMACKeyField, undefined);
		} else {
			const authority = authorities.find((item) => item.id === provider || item.issuerProvider === provider);
			setFieldValue(eabKeyIDField, eabKeyIDValue || authority?.eabKeyId || "");
			setFieldValue(eabMACKeyField, eabMACKeyValue || authority?.eabMacKey || "");
			if (provider === "custom" && !meta.caDirectory && authority?.caDirectory) {
				setFieldValue(caField, authority.caDirectory);
			}
		}
	};

	return (
		<div className="mb-3">
			<Field name={name}>
				{({ field }: any) => (
					<>
						<label htmlFor="certificateAuthority" className="form-label">
							签发机构
						</label>
						<select
							id="certificateAuthority"
							className="form-select"
							{...field}
							value={current}
							onChange={handleChange}
						>
							<option value="auto">Caddy 自动 (Let's Encrypt / ZeroSSL)</option>
							<option value="letsencrypt">Let's Encrypt</option>
							<option value="letsencrypt-staging">Let's Encrypt Staging (测试)</option>
							<option value="zerossl">ZeroSSL ACME (自动 EAB)</option>
							<option value="google">Google Trust Services (首次需 EAB)</option>
							<option value="internal">Caddy Internal (内网适用)</option>
							<option value="custom">自定义 ACME Directory URL</option>
						</select>
					</>
				)}
			</Field>
			{current === "custom" ? (
				<Field name={caField}>
					{({ field }: any) => (
						<div className="mt-3">
							<label htmlFor="acmeDirectoryUrl" className="form-label">
								ACME Directory URL
							</label>
							<input
								id="acmeDirectoryUrl"
								className="form-control"
								placeholder="https://example.com/acme/directory"
								{...field}
							/>
						</div>
					)}
				</Field>
			) : null}
			{current === "google" || current === "custom" ? (
				<div className="row mt-3">
					<div className="col-6">
						<Field name={eabKeyIDField}>
							{({ field }: any) => (
								<>
									<label htmlFor="eabKeyId" className="form-label">
										EAB Key ID
									</label>
									<input
										id="eabKeyId"
										className="form-control"
										{...field}
										value={field.value ?? eabKeyIDValue}
									/>
								</>
							)}
						</Field>
					</div>
					<div className="col-6">
						<Field name={eabMACKeyField}>
							{({ field }: any) => (
								<>
									<label htmlFor="eabMacKey" className="form-label">
										EAB MAC Key
									</label>
									<input
										id="eabMacKey"
										className="form-control"
										type="password"
										{...field}
										value={field.value ?? eabMACKeyValue}
									/>
								</>
							)}
						</Field>
					</div>
					<small className="text-muted mt-2">Google 首次填写 EAB 后会保存，后续申请可留空复用。</small>
				</div>
			) : null}
		</div>
	);
}
