import { IconEye, IconEyeOff } from "@tabler/icons-react";
import { useState } from "react";
import type { ProviderField } from "src/api/backend";

interface Props {
	fields?: ProviderField[];
	values?: Record<string, string>;
	onChange: (key: string, value: string) => void;
	visibleSecrets?: Record<string, boolean>;
	onToggleSecret?: (key: string) => void;
	idPrefix?: string;
	columns?: number;
}

// ProviderCredentialFields 按后端下发的字段 schema 渲染凭据输入框。
// 新增 provider 时前端无需改动，只需后端在注册表里加字段。
export function ProviderCredentialFields({
	fields,
	values,
	onChange,
	visibleSecrets,
	onToggleSecret,
	idPrefix = "credentialField",
	columns = 1,
}: Props) {
	const [internalVisibility, setInternalVisibility] = useState<Record<string, boolean>>({});

	if (!fields || fields.length === 0) {
		return null;
	}

	return (
		<div className="row g-3">
			{fields.map((field) => {
				const inputId = `${idPrefix}${field.key}`;
				const value = values?.[field.key] || "";
				const visible = Boolean(visibleSecrets?.[field.key]) || Boolean(internalVisibility[field.key]);
				const isSecret = field.secret !== false;

				return (
					<div className={columns === 1 ? "col-md-12" : "col-md-6"} key={field.key}>
						<label className="form-label" htmlFor={inputId}>
							{field.label}
						</label>
						<div className="input-group">
							<input
								id={inputId}
								className="form-control"
								type={isSecret && !visible ? "password" : "text"}
								autoComplete="off"
								placeholder={field.placeholder}
								value={value}
								onChange={(event) => onChange(field.key, event.target.value)}
							/>
							{isSecret ? (
								<button
									type="button"
									className="btn btn-icon"
									aria-label={visible ? "隐藏明文" : "显示明文"}
									title={visible ? "隐藏明文" : "显示明文"}
									onClick={() => {
										if (onToggleSecret) {
											onToggleSecret(field.key);
											return;
										}
										setInternalVisibility({
											...internalVisibility,
											[field.key]: !visible,
										});
									}}
								>
									{visible ? <IconEyeOff size={18} /> : <IconEye size={18} />}
								</button>
							) : null}
						</div>
						{field.help ? <small className="text-muted">{field.help}</small> : null}
					</div>
				);
			})}
		</div>
	);
}
