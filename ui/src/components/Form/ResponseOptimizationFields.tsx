import cn from "classnames";
import { Field, getIn, useFormikContext } from "formik";
import { useState } from "react";
import type { ResponseOptimizationConfig } from "src/api/backend";
import { useHeaderPolicyTemplates } from "src/hooks";

const fallbackCacheControl = "public, max-age=3600";

const cacheControlOptions = [
	{ label: "短缓存 5 分钟", value: "public, max-age=300" },
	{ label: "静态缓存 1 小时", value: "public, max-age=3600" },
	{ label: "长期缓存 30 天", value: "public, max-age=2592000, immutable" },
	{ label: "禁止缓存", value: "no-store" },
];

export const withResponseOptimizationDefaults = (
	value?: ResponseOptimizationConfig,
	defaultCacheControl = fallbackCacheControl,
): ResponseOptimizationConfig => ({
	compressionEnabled: !!value?.compressionEnabled,
	cacheControlEnabled: !!value?.cacheControlEnabled,
	cacheControlValue: String(value?.cacheControlValue || defaultCacheControl),
	securityHeadersEnabled: !!value?.securityHeadersEnabled,
	policyTemplateId: Number(value?.policyTemplateId || 0),
});

export const normalizeResponseOptimization = (
	value?: ResponseOptimizationConfig,
	defaultCacheControl = fallbackCacheControl,
): ResponseOptimizationConfig => {
	const cfg = withResponseOptimizationDefaults(value, defaultCacheControl);
	return {
		compressionEnabled: !!cfg.compressionEnabled,
		cacheControlEnabled: !!cfg.cacheControlEnabled,
		cacheControlValue: cfg.cacheControlEnabled ? String(cfg.cacheControlValue || defaultCacheControl).trim() : "",
		securityHeadersEnabled: !!cfg.securityHeadersEnabled,
		policyTemplateId: Number(cfg.policyTemplateId || 0),
	};
};

interface Props {
	color?: string;
	defaultCacheControl?: string;
}

export function ResponseOptimizationFields({ color = "bg-lime", defaultCacheControl = fallbackCacheControl }: Props) {
	const { values, setFieldValue } = useFormikContext<any>();
	const { data: policyTemplates } = useHeaderPolicyTemplates();
	const [useCustomCacheControl, setUseCustomCacheControl] = useState(false);
	const response = values.response ?? {};
	const cacheValue = String(response.cacheControlValue || defaultCacheControl);
	const knownCacheControl = cacheControlOptions.some((item) => item.value === cacheValue);
	const selectedCacheValue = useCustomCacheControl || !knownCacheControl ? "custom" : cacheValue;

	const validateCacheControl = (value: any) => {
		if (!response.cacheControlEnabled) return undefined;
		const raw = String(value || "").trim();
		if (!raw) return "Cache-Control 不能为空";
		if (/[\r\n\t]/.test(raw)) return "Cache-Control 不能包含控制字符";
		return undefined;
	};

	return (
		<div className="divide-y">
			<div>
				<label className="form-label" htmlFor="responsePolicyTemplateId">
					策略模板
				</label>
				<Field as="select" id="responsePolicyTemplateId" name="response.policyTemplateId" className="form-select">
					<option value={0}>不绑定模板</option>
					{(policyTemplates || [])
						.filter((template) => template.enabled)
						.map((template) => (
							<option key={template.id} value={template.id}>
								{template.name}
								{template.builtin ? "（内置）" : ""}
							</option>
						))}
				</Field>
				<div className="form-hint">绑定后会复用模板中的 CORS、CSP、缓存和安全响应头。</div>
			</div>
			<div>
				<label className="row" htmlFor="responseCompressionEnabled">
					<span className="col">压缩</span>
					<span className="col-auto">
						<Field name="response.compressionEnabled" type="checkbox">
							{({ field }: any) => (
								<label className="form-check form-check-single form-switch">
									<input
										{...field}
										id="responseCompressionEnabled"
										className={cn("form-check-input", { [color]: field.checked })}
										type="checkbox"
									/>
								</label>
							)}
						</Field>
					</span>
				</label>
			</div>
			<div>
				<label className="row" htmlFor="responseCacheControlEnabled">
					<span className="col">Cache-Control</span>
					<span className="col-auto">
						<Field name="response.cacheControlEnabled" type="checkbox">
							{({ field, form }: any) => (
								<label className="form-check form-check-single form-switch">
									<input
										{...field}
										id="responseCacheControlEnabled"
										className={cn("form-check-input", { [color]: field.checked })}
										type="checkbox"
										onChange={(event) => {
											const checked = event.target.checked;
											form.setFieldValue(field.name, checked);
											if (checked && !String(response.cacheControlValue || "").trim()) {
												form.setFieldValue("response.cacheControlValue", defaultCacheControl);
											}
										}}
									/>
								</label>
							)}
						</Field>
					</span>
				</label>
				{response.cacheControlEnabled ? (
					<div className="mt-2">
						<select
							className="form-select"
							value={selectedCacheValue}
							onChange={(event) => {
								const value = event.target.value;
								if (value === "custom") {
									setUseCustomCacheControl(true);
									return;
								}
								setUseCustomCacheControl(false);
								setFieldValue("response.cacheControlValue", value);
							}}
						>
							{cacheControlOptions.map((item) => (
								<option key={item.value} value={item.value}>
									{item.label}
								</option>
							))}
							<option value="custom">自定义</option>
						</select>
						<Field name="response.cacheControlValue" validate={validateCacheControl}>
							{({ field, form }: any) => {
								const error = getIn(form.errors, field.name);
								const touched = getIn(form.touched, field.name);
								if (selectedCacheValue !== "custom") return null;
								return (
									<>
										<input
											{...field}
											type="text"
											className={cn("form-control mt-2", { "is-invalid": error && touched })}
										/>
										{error && touched ? <div className="invalid-feedback">{error}</div> : null}
									</>
								);
							}}
						</Field>
					</div>
				) : null}
			</div>
			<div>
				<label className="row" htmlFor="responseSecurityHeadersEnabled">
					<span className="col">安全响应头</span>
					<span className="col-auto">
						<Field name="response.securityHeadersEnabled" type="checkbox">
							{({ field }: any) => (
								<label className="form-check form-check-single form-switch">
									<input
										{...field}
										id="responseSecurityHeadersEnabled"
										className={cn("form-check-input", { [color]: field.checked })}
										type="checkbox"
									/>
								</label>
							)}
						</Field>
					</span>
				</label>
			</div>
		</div>
	);
}
