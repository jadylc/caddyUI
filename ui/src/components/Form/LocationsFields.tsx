import cn from "classnames";
import { getIn, useFormikContext } from "formik";
import { useState } from "react";
import type { ProxyLocation } from "src/api/backend";
import { T } from "src/locale";
import styles from "./LocationsFields.module.css";
import { ReverseProxyAdvancedPanel, withReverseProxyDefaults } from "./ReverseProxyAdvancedPanel";
import { UpstreamsFields } from "./UpstreamsFields";

interface Props {
	initialValues: ProxyLocation[];
	name?: string;
}

type FieldErrors = Record<number, Record<string, string>>;

function validateLocation(loc: ProxyLocation): Record<string, string> {
	const errs: Record<string, string> = {};
	const upstreams = Array.isArray(loc.upstreams) ? loc.upstreams : [];
	const firstUpstream = upstreams[0];
	const forwardHost = firstUpstream?.forwardHost ?? loc.forwardHost;
	const forwardPort = firstUpstream?.forwardPort ?? loc.forwardPort;
	if (!forwardHost?.trim()) {
		errs.forwardHost = "location.error.forward-host-required";
	}
	if (!forwardPort || forwardPort < 1 || forwardPort > 65535) {
		errs.forwardPort = "location.error.forward-port-invalid";
	}
	const p = loc.path?.trim();
	const fp = loc.forwardPath?.trim();
	if (p === "/") {
		errs.path = "location.error.path-no-slash";
	}
	if (!p && fp) {
		errs.forwardPath = "location.error.forward-path-needs-path";
	}
	if (fp === "/") {
		errs.forwardPath = "location.error.forward-path-no-slash";
	}
	return errs;
}

function validateAll(locs: ProxyLocation[]): FieldErrors {
	const errors: FieldErrors = {};
	locs.forEach((loc, idx) => {
		const errs = validateLocation(loc);
		if (Object.keys(errs).length > 0) errors[idx] = errs;
	});
	return errors;
}

export function LocationsFields({ initialValues, name = "locations" }: Props) {
	const { values: formValues, setFieldValue } = useFormikContext<any>();
	const values = (getIn(formValues, name) as ProxyLocation[] | undefined) ?? initialValues ?? [];
	const [fieldErrors, setFieldErrors] = useState<FieldErrors>(() => validateAll(initialValues || []));

	const blankItem: ProxyLocation = {
		path: "",
		forwardScheme: "http",
		forwardHost: "",
		forwardPort: 80,
		forwardPath: "",
		upstreams: [{ forwardScheme: "http", forwardHost: "", forwardPort: 80, weight: 1 }],
		loadBalancingPolicy: "",
		reverseProxyMode: "inherit",
		reverseProxy: withReverseProxyDefaults(),
	};

	const setFormField = (newValues: ProxyLocation[]) => {
		setFieldValue(name, newValues);
	};

	const handleAdd = () => {
		const newValues = [...values, blankItem];
		setFormField(newValues);
	};

	const handleRemove = (idx: number) => {
		const newValues = values.filter((_: ProxyLocation, i: number) => i !== idx);
		setFieldErrors(validateAll(newValues));
		setFormField(newValues);
	};

	const handleChange = (idx: number, field: string, fieldValue: string) => {
		const parsed = field === "forwardPort" ? Number(fieldValue) : fieldValue;
		const newValues = values.map((v: ProxyLocation, i: number) => (i === idx ? { ...v, [field]: parsed } : v));
		setFormField(newValues);
		// Validate only the changed item for responsiveness
		const errs = validateLocation(newValues[idx]);
		setFieldErrors((prev) => {
			const next = { ...prev };
			if (Object.keys(errs).length > 0) {
				next[idx] = errs;
			} else {
				delete next[idx];
			}
			return next;
		});
	};

	const handleReverseProxyModeChange = (idx: number, mode: string) => {
		const nextMode: "inherit" | "custom" = mode === "custom" ? "custom" : "inherit";
		const newValues = values.map((v: ProxyLocation, i: number) =>
			i === idx
				? {
						...v,
						reverseProxyMode: nextMode,
						reverseProxy:
							nextMode === "custom"
								? withReverseProxyDefaults(v.reverseProxy)
								: withReverseProxyDefaults(),
					}
				: v,
		);
		setFormField(newValues);
	};

	const err = (idx: number, field: string) => fieldErrors[idx]?.[field];

	if (values.length === 0) {
		return (
			<div className="text-center">
				<button type="button" className="btn my-3" onClick={handleAdd}>
					<T id="action.add-location" />
				</button>
			</div>
		);
	}

	return (
		<>
			{values.map((item: ProxyLocation, idx: number) => (
				<div key={idx} className={cn("card", "card-active", "mb-3", styles.locationCard)}>
					<div className="card-body">
						<div className="row">
							<div className="col-md-12">
								<div className="input-group mb-3">
									<span className="input-group-text">Location</span>
									<input
										type="text"
										className={cn("form-control", err(idx, "path") && "is-invalid")}
										placeholder="/path"
										autoComplete="off"
										value={item.path}
										onChange={(e) => handleChange(idx, "path", e.target.value)}
									/>
									{err(idx, "path") && (
										<div className="invalid-feedback">
											<T id={err(idx, "path") || ""} />
										</div>
									)}
								</div>
							</div>
						</div>
						<UpstreamsFields
							name={`${name}.${idx}.upstreams`}
							policyName={`${name}.${idx}.loadBalancingPolicy`}
						/>
						<div className="row">
							<div className="col-md-12">
								<div className="mb-3">
									<label className="form-label" htmlFor="forwardPath">
										<T id="host.forward-path" />
									</label>
									<input
										id="forwardPath"
										type="text"
										className={cn("form-control", err(idx, "forwardPath") && "is-invalid")}
										placeholder="/target-path (留空则透传原路径)"
										autoComplete="off"
										value={item.forwardPath || ""}
										onChange={(e) => handleChange(idx, "forwardPath", e.target.value)}
									/>
									{err(idx, "forwardPath") && (
										<div className="invalid-feedback">
											<T id={err(idx, "forwardPath") || ""} />
										</div>
									)}
								</div>
							</div>
						</div>
						<div className="border-top pt-3">
							<div className="mb-3">
								<label className="form-label" htmlFor={`${name}-${idx}-reverse-proxy-mode`}>
									反代高级
								</label>
								<select
									id={`${name}-${idx}-reverse-proxy-mode`}
									className="form-control"
									value={item.reverseProxyMode === "custom" ? "custom" : "inherit"}
									onChange={(e) => handleReverseProxyModeChange(idx, e.target.value)}
								>
									<option value="inherit">继承站点级配置</option>
									<option value="custom">自定义当前路径</option>
								</select>
							</div>
							{item.reverseProxyMode === "custom" ? (
								<ReverseProxyAdvancedPanel
									name={`${name}.${idx}.reverseProxy`}
									compact
									inheritNote="当前路径会覆盖站点级反代高级配置。未填写的项按 Caddy 默认值处理。"
								/>
							) : null}
						</div>
						<div className="mt-1">
							<a
								href="#"
								onClick={(e) => {
									e.preventDefault();
									handleRemove(idx);
								}}
							>
								<T id="action.delete" />
							</a>
						</div>
					</div>
				</div>
			))}
			<div>
				<button type="button" className="btn btn-sm" onClick={handleAdd}>
					<T id="action.add-location" />
				</button>
			</div>
		</>
	);
}
