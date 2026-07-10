import cn from "classnames";
import { Field, getIn, useFormikContext } from "formik";
import type { ErrorHandlingConfig, ErrorResponseTemplate } from "src/api/backend";
import { useErrorResponseTemplates } from "src/hooks";

export const withErrorHandlingDefaults = (value?: ErrorHandlingConfig): ErrorHandlingConfig => ({
	enabled: !!value?.enabled,
	templateId: value?.templateId || 0,
});

export const normalizeErrorHandling = (value?: ErrorHandlingConfig): ErrorHandlingConfig => {
	const cfg = withErrorHandlingDefaults(value);
	return {
		enabled: !!cfg.enabled,
		templateId: cfg.enabled ? Number(cfg.templateId || 0) : 0,
	};
};

const templateSummary = (template?: ErrorResponseTemplate) => {
	if (!template) return "";
	return (template.rules || [])
		.map((rule) => `${rule.statuses?.[0] || "default"} -> ${rule.action}`)
		.join("，");
};

interface Props {
	color?: string;
}

export function ErrorHandlingFields({ color = "bg-lime" }: Props) {
	const { values, setFieldValue } = useFormikContext<any>();
	const { data: templates, isLoading } = useErrorResponseTemplates();
	const errorHandling = values.errorHandling ?? {};
	const selected = templates?.find((item) => item.id === Number(errorHandling.templateId || 0));

	const validateTemplate = (value: any) => {
		if (!errorHandling.enabled) return undefined;
		if (!Number(value || 0)) return "请选择错误响应模板";
		return undefined;
	};

	return (
		<div className="divide-y">
			<div>
				<label className="row" htmlFor="errorHandlingEnabled">
					<span className="col">错误处理</span>
					<span className="col-auto">
						<Field name="errorHandling.enabled" type="checkbox">
							{({ field, form }: any) => (
								<label className="form-check form-check-single form-switch">
									<input
										{...field}
										id="errorHandlingEnabled"
										className={cn("form-check-input", { [color]: field.checked })}
										type="checkbox"
										onChange={(event) => {
											const checked = event.target.checked;
											form.setFieldValue(field.name, checked);
											if (!checked) {
												form.setFieldValue("errorHandling.templateId", 0);
											}
										}}
									/>
								</label>
							)}
						</Field>
					</span>
				</label>
				{errorHandling.enabled ? (
					<Field name="errorHandling.templateId" validate={validateTemplate}>
						{({ field, form }: any) => {
							const error = getIn(form.errors, field.name);
							const touched = getIn(form.touched, field.name);
							return (
								<div className="mt-2">
									<select
										className={cn("form-select", { "is-invalid": error && touched })}
										value={field.value || 0}
										disabled={isLoading}
										onBlur={field.onBlur}
										onChange={(event) => setFieldValue(field.name, Number(event.target.value || 0))}
									>
										<option value={0}>选择模板</option>
										{(templates || []).map((template) => (
											<option key={template.id} value={template.id}>
												{template.name}
												{template.enabled ? "" : "（停用）"}
											</option>
										))}
									</select>
									{error && touched ? <div className="invalid-feedback">{error}</div> : null}
									{selected ? (
										<div className="text-muted small mt-2">
											{templateSummary(selected)}
											{selected.staticRoot ? ` · ${selected.staticRoot}` : ""}
										</div>
									) : null}
								</div>
							);
						}}
					</Field>
				) : null}
			</div>
		</div>
	);
}
