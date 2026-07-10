import cn from "classnames";
import { Field, getIn } from "formik";
import type { AccessLogConfig } from "src/api/backend";

const defaultAccessLog: Required<AccessLogConfig> = {
	enabled: false,
	format: "json",
	outputFile: "",
	rollSize: "10MiB",
	rollKeep: 10,
	rollKeepDays: 30,
};

export const withAccessLogDefaults = (value?: AccessLogConfig): AccessLogConfig => ({
	...defaultAccessLog,
	...(value ?? {}),
	format: value?.format === "console" ? "console" : "json",
	outputFile: String(value?.outputFile || ""),
	rollSize: String(value?.rollSize || defaultAccessLog.rollSize),
	rollKeep: Number(value?.rollKeep || defaultAccessLog.rollKeep),
	rollKeepDays: Number(value?.rollKeepDays || defaultAccessLog.rollKeepDays),
});

export const normalizeAccessLog = (value?: AccessLogConfig): AccessLogConfig => {
	const cfg = withAccessLogDefaults(value);
	return {
		enabled: !!cfg.enabled,
		format: cfg.format === "console" ? "console" : "json",
		outputFile: String(cfg.outputFile || "").trim(),
		rollSize: String(cfg.rollSize || defaultAccessLog.rollSize).trim(),
		rollKeep: Math.max(1, Number(cfg.rollKeep || defaultAccessLog.rollKeep)),
		rollKeepDays: Math.max(1, Number(cfg.rollKeepDays || defaultAccessLog.rollKeepDays)),
	};
};

const validateOutputFile = (value: any) => {
	const raw = String(value || "").trim();
	if (!raw) return undefined;
	if (!raw.startsWith("/config/access-logs/")) return "输出文件必须位于 /config/access-logs";
	const file = raw.replace("/config/access-logs/", "");
	if (!file || file.includes("/") || file.includes("\\")) return "输出文件必须是直接文件";
	if (/[\r\n\t]/.test(raw)) return "输出文件不能包含控制字符";
	return undefined;
};

const validateRollSize = (value: any) => {
	const raw = String(value || "").trim();
	if (!raw) return "单文件大小不能为空";
	if (!/^[1-9][0-9]*(B|KB|MB|GB|KiB|MiB|GiB)?$/.test(raw)) return "格式类似 10MiB、100MB";
	return undefined;
};

const validatePositiveNumber = (label: string) => (value: any) => {
	const n = Number(value);
	if (!Number.isInteger(n) || n <= 0) return `${label}必须大于 0`;
	return undefined;
};

interface Props {
	color?: string;
}

export function AccessLogFields({ color = "bg-lime" }: Props) {
	return (
		<div className="divide-y">
			<div>
				<label className="row" htmlFor="accessLogEnabled">
					<span className="col">访问日志</span>
					<span className="col-auto">
						<Field name="accessLog.enabled" type="checkbox">
							{({ field }: any) => (
								<label className="form-check form-check-single form-switch">
									<input
										{...field}
										id="accessLogEnabled"
										className={cn("form-check-input", { [color]: field.checked })}
										type="checkbox"
									/>
								</label>
							)}
						</Field>
					</span>
				</label>
			</div>
			<Field name="accessLog.enabled">
				{({ field: enabledField }: any) =>
					enabledField.value ? (
						<div className="pt-3">
							<div className="row g-3">
								<Field name="accessLog.format">
									{({ field }: any) => (
										<div className="col-md-4">
											<label className="form-label" htmlFor="accessLogFormat">
												格式
											</label>
											<select id="accessLogFormat" className="form-control" {...field}>
												<option value="json">JSON</option>
												<option value="console">Console</option>
											</select>
										</div>
									)}
								</Field>
								<Field name="accessLog.outputFile" validate={validateOutputFile}>
									{({ field, form }: any) => {
										const error = getIn(form.errors, field.name);
										const touched = getIn(form.touched, field.name);
										return (
											<div className="col-md-8">
												<label className="form-label" htmlFor="accessLogOutputFile">
													输出文件
												</label>
												<input
													{...field}
													id="accessLogOutputFile"
													type="text"
													className={cn("form-control", { "is-invalid": error && touched })}
													placeholder="/config/access-logs/site.log"
												/>
												{error && touched ? <div className="invalid-feedback">{error}</div> : null}
											</div>
										);
									}}
								</Field>
								<Field name="accessLog.rollSize" validate={validateRollSize}>
									{({ field, form }: any) => {
										const error = getIn(form.errors, field.name);
										const touched = getIn(form.touched, field.name);
										return (
											<div className="col-md-4">
												<label className="form-label" htmlFor="accessLogRollSize">
													单文件大小
												</label>
												<input
													{...field}
													id="accessLogRollSize"
													type="text"
													className={cn("form-control", { "is-invalid": error && touched })}
												/>
												{error && touched ? <div className="invalid-feedback">{error}</div> : null}
											</div>
										);
									}}
								</Field>
								<Field name="accessLog.rollKeep" validate={validatePositiveNumber("保留文件数")}>
									{({ field, form }: any) => {
										const error = getIn(form.errors, field.name);
										const touched = getIn(form.touched, field.name);
										return (
											<div className="col-md-4">
												<label className="form-label" htmlFor="accessLogRollKeep">
													保留文件数
												</label>
												<input
													{...field}
													id="accessLogRollKeep"
													type="number"
													min={1}
													className={cn("form-control", { "is-invalid": error && touched })}
												/>
												{error && touched ? <div className="invalid-feedback">{error}</div> : null}
											</div>
										);
									}}
								</Field>
								<Field name="accessLog.rollKeepDays" validate={validatePositiveNumber("保留天数")}>
									{({ field, form }: any) => {
										const error = getIn(form.errors, field.name);
										const touched = getIn(form.touched, field.name);
										return (
											<div className="col-md-4">
												<label className="form-label" htmlFor="accessLogRollKeepDays">
													保留天数
												</label>
												<input
													{...field}
													id="accessLogRollKeepDays"
													type="number"
													min={1}
													className={cn("form-control", { "is-invalid": error && touched })}
												/>
												{error && touched ? <div className="invalid-feedback">{error}</div> : null}
											</div>
										);
									}}
								</Field>
							</div>
						</div>
					) : null
				}
			</Field>
		</div>
	);
}
