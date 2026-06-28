import cn from "classnames";
import { useFormikContext } from "formik";
import { useCallback, useState } from "react";
import type { ProxyLocation } from "src/api/backend";
import { T } from "src/locale";
import styles from "./LocationsFields.module.css";

interface Props {
	initialValues: ProxyLocation[];
	name?: string;
}

type FieldErrors = Record<number, Record<string, string>>;

function validateLocation(loc: ProxyLocation): Record<string, string> {
	const errs: Record<string, string> = {};
	if (!loc.forwardHost?.trim()) {
		errs.forwardHost = "location.error.forward-host-required";
	}
	if (!loc.forwardPort || loc.forwardPort < 1 || loc.forwardPort > 65535) {
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
	const [values, setValues] = useState<ProxyLocation[]>(initialValues || []);
	const [fieldErrors, setFieldErrors] = useState<FieldErrors>(() => validateAll(initialValues || []));
	const { setFieldValue } = useFormikContext();

	const blankItem: ProxyLocation = {
		path: "",
		forwardScheme: "http",
		forwardHost: "",
		forwardPort: 80,
		forwardPath: "",
	};

	const setFormField = useCallback(
		(newValues: ProxyLocation[]) => {
			const filtered = newValues.filter((v: ProxyLocation) => v?.path?.trim() !== "");
			setFieldValue(name, filtered);
		},
		[setFieldValue, name],
	);

	const handleAdd = () => {
		const newValues = [...values, blankItem];
		setValues(newValues);
		setFormField(newValues);
	};

	const handleRemove = (idx: number) => {
		const newValues = values.filter((_: ProxyLocation, i: number) => i !== idx);
		setValues(newValues);
		setFieldErrors(validateAll(newValues));
		setFormField(newValues);
	};

	const handleChange = (idx: number, field: string, fieldValue: string) => {
		const parsed = field === "forwardPort" ? Number(fieldValue) : fieldValue;
		const newValues = values.map((v: ProxyLocation, i: number) => (i === idx ? { ...v, [field]: parsed } : v));
		setValues(newValues);
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
											<T id={err(idx, "path")!} />
										</div>
									)}
								</div>
							</div>
						</div>
						<div className="row">
							<div className="col-md-3">
								<div className="mb-3">
									<label className="form-label" htmlFor="forwardScheme">
										<T id="host.forward-scheme" />
									</label>
									<select
										id="forwardScheme"
										className="form-control"
										value={item.forwardScheme}
										onChange={(e) => handleChange(idx, "forwardScheme", e.target.value)}
									>
										<option value="http">http</option>
										<option value="https">https</option>
									</select>
								</div>
							</div>
							<div className="col-md-6">
								<div className="mb-3">
									<label className="form-label" htmlFor="forwardHost">
										<T id="proxy-host.forward-host" />
									</label>
									<input
										id="forwardHost"
										type="text"
										className={cn("form-control", err(idx, "forwardHost") && "is-invalid")}
										required
										placeholder="eg: 10.0.0.1"
										value={item.forwardHost}
										onChange={(e) => handleChange(idx, "forwardHost", e.target.value)}
									/>
									{err(idx, "forwardHost") && (
										<div className="invalid-feedback">
											<T id={err(idx, "forwardHost")!} />
										</div>
									)}
								</div>
							</div>
							<div className="col-md-3">
								<div className="mb-3">
									<label className="form-label" htmlFor="forwardPort">
										<T id="host.forward-port" />
									</label>
									<input
										id="forwardPort"
										type="number"
										min={1}
										max={65535}
										className={cn("form-control", err(idx, "forwardPort") && "is-invalid")}
										required
										placeholder="eg: 8081"
										value={item.forwardPort}
										onChange={(e) => handleChange(idx, "forwardPort", e.target.value)}
									/>
									{err(idx, "forwardPort") && (
										<div className="invalid-feedback">
											<T id={err(idx, "forwardPort")!} />
										</div>
									)}
								</div>
							</div>
						</div>
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
											<T id={err(idx, "forwardPath")!} />
										</div>
									)}
								</div>
							</div>
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
