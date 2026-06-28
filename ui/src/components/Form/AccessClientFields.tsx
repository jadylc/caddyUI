import { IconX } from "@tabler/icons-react";
import cn from "classnames";
import { useFormikContext } from "formik";
import { useState } from "react";
import type { AccessListClient } from "src/api/backend";
import { intl, T } from "src/locale";

type FieldErrors = Record<number, Record<string, string>>;

function validateClient(_idx: number, item: AccessListClient): Record<string, string> {
	const errs: Record<string, string> = {};
	if (!item.address?.trim()) {
		errs.address = "access-list.error.address-required";
	}
	return errs;
}

interface Props {
	initialValues: AccessListClient[];
	name?: string;
}
export function AccessClientFields({ initialValues, name = "clients" }: Props) {
	const [values, setValues] = useState<AccessListClient[]>(initialValues || []);
	const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
	const { setFieldValue } = useFormikContext();

	const blankClient: AccessListClient = { directive: "allow", address: "" };

	if (values?.length === 0) {
		setValues([blankClient]);
	}

	const handleAdd = () => {
		setValues([...values, blankClient]);
	};

	const handleRemove = (idx: number) => {
		const newValues = values.filter((_: AccessListClient, i: number) => i !== idx);
		if (newValues.length === 0) {
			newValues.push(blankClient);
		}
		setValues(newValues);
		setFormField(newValues);
		setFieldErrors((prev) => {
			const next: FieldErrors = {};
			Object.entries(prev).forEach(([k, v]) => {
				const numKey = Number(k);
				if (numKey < idx) next[numKey] = v;
				else if (numKey > idx) next[numKey - 1] = v;
			});
			return next;
		});
	};

	const handleChange = (idx: number, field: string, fieldValue: string) => {
		const newValues = values.map((v: AccessListClient, i: number) =>
			i === idx ? { ...v, [field]: fieldValue } : v,
		);
		setValues(newValues);
		setFormField(newValues);
		const errs = validateClient(idx, newValues[idx]);
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

	const setFormField = (newValues: AccessListClient[]) => {
		const filtered = newValues.filter((v: AccessListClient) => v?.address?.trim() !== "");
		setFieldValue(name, filtered);
	};

	const err = (idx: number, field: string) => fieldErrors[idx]?.[field];

	return (
		<>
			<p className="text-muted">
				<T id="access-list.help.rules-order" />
			</p>
			{values.map((client: AccessListClient, idx: number) => (
				<div className="row mb-1" key={idx}>
					<div className="col-11">
						<div className="input-group mb-2">
							<span className="input-group-select">
								<select
									className={cn(
										"form-select",
										"m-0",
										client.directive === "allow" ? "bg-lime-lt" : "bg-orange-lt",
									)}
									name={`clients[${idx}].directive`}
									value={client.directive}
									onChange={(e) => handleChange(idx, "directive", e.target.value)}
								>
									<option value="allow">
										<T id="action.allow" />
									</option>
									<option value="deny">
										<T id="action.deny" />
									</option>
								</select>
							</span>
							<input
								name={`clients[${idx}].address`}
								type="text"
								className={`form-control ${err(idx, "address") ? "is-invalid" : ""}`}
								autoComplete="off"
								value={client.address}
								onChange={(e) => handleChange(idx, "address", e.target.value)}
								placeholder={intl.formatMessage({ id: "access-list.rule-source.placeholder" })}
							/>
						</div>
						{err(idx, "address") && (
							<div className="text-danger mt-n1 mb-1" style={{ fontSize: "0.875em" }}>
								<T id={err(idx, "address")!} />
							</div>
						)}
					</div>
					<div className="col-1">
						<a
							role="button"
							className="btn btn-ghost btn-danger p-0"
							onClick={(e) => {
								e.preventDefault();
								handleRemove(idx);
							}}
						>
							<IconX size={16} />
						</a>
					</div>
				</div>
			))}
			<div className="mb-3">
				<button type="button" className="btn btn-sm" onClick={handleAdd}>
					<T id="action.add" />
				</button>
			</div>
			<div className="row mb-3">
				<p className="text-muted">
					<T id="access-list.help-rules-last" />
				</p>
				<div className="col-11">
					<div className="input-group mb-2">
						<span className="input-group-select">
							<select
								className="form-select m-0 bg-orange-lt"
								name="clients[last].directive"
								value="deny"
								disabled
							>
								<option value="deny">
									<T id="action.deny" />
								</option>
							</select>
						</span>
						<input
							name="clients[last].address"
							type="text"
							className="form-control"
							autoComplete="off"
							value="all"
							disabled
						/>
					</div>
				</div>
			</div>
		</>
	);
}
