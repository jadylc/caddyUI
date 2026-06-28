import { IconX } from "@tabler/icons-react";
import { useFormikContext } from "formik";
import { useState } from "react";
import type { AccessListItem } from "src/api/backend";
import { T } from "src/locale";

type FieldErrors = Record<number, Record<string, string>>;

function validateAuth(_idx: number, item: AccessListItem, isExisting: boolean): Record<string, string> {
	const errs: Record<string, string> = {};
	if (item.password?.trim() && !item.username?.trim()) {
		errs.username = "access-list.error.username-required";
	}
	if (!isExisting && item.username?.trim() && !item.password?.trim()) {
		errs.password = "access-list.error.password-required";
	}
	return errs;
}

interface Props {
	initialValues: AccessListItem[];
	name?: string;
}
export function BasicAuthFields({ initialValues, name = "items" }: Props) {
	const [values, setValues] = useState<AccessListItem[]>(initialValues || []);
	const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
	const { setFieldValue } = useFormikContext();

	const blankItem: AccessListItem = { username: "", password: "" };

	if (values?.length === 0) {
		setValues([blankItem]);
	}

	const handleAdd = () => {
		setValues([...values, blankItem]);
	};

	const handleRemove = (idx: number) => {
		const newValues = values.filter((_: AccessListItem, i: number) => i !== idx);
		if (newValues.length === 0) {
			newValues.push(blankItem);
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
		const newValues = values.map((v: AccessListItem, i: number) => (i === idx ? { ...v, [field]: fieldValue } : v));
		setValues(newValues);
		setFormField(newValues);
		const isExisting = initialValues.some((iv) => iv.username === newValues[idx]?.username && iv.username?.trim());
		const errs = validateAuth(idx, newValues[idx], isExisting);
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

	const setFormField = (newValues: AccessListItem[]) => {
		const filtered = newValues.filter((v: AccessListItem) => v?.username?.trim() !== "");
		setFieldValue(name, filtered);
	};

	const err = (idx: number, field: string) => fieldErrors[idx]?.[field];

	return (
		<>
			<div className="row">
				<div className="col-6">
					<label className="form-label" htmlFor="...">
						<T id="username" />
					</label>
				</div>
				<div className="col-6">
					<label className="form-label" htmlFor="...">
						<T id="password" />
					</label>
				</div>
			</div>
			{values.map((item: AccessListItem, idx: number) => (
				<div className="row mb-3" key={idx}>
					<div className="col-6">
						<input
							type="text"
							autoComplete="off"
							className={`form-control input-sm ${err(idx, "username") ? "is-invalid" : ""}`}
							value={item.username}
							onChange={(e) => handleChange(idx, "username", e.target.value)}
						/>
						{err(idx, "username") && (
							<div className="invalid-feedback d-block">
								<T id={err(idx, "username")!} />
							</div>
						)}
					</div>
					<div className="col-5">
						<input
							type="password"
							autoComplete="off"
							className={`form-control ${err(idx, "password") ? "is-invalid" : ""}`}
							value={item.password}
							placeholder={
								initialValues.filter((iv: AccessListItem) => iv.username === item.username).length > 0
									? "••••••••"
									: ""
							}
							onChange={(e) => handleChange(idx, "password", e.target.value)}
						/>
						{err(idx, "password") && (
							<div className="invalid-feedback d-block">
								<T id={err(idx, "password")!} />
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
			<div>
				<button type="button" className="btn btn-sm" onClick={handleAdd}>
					<T id="action.add" />
				</button>
			</div>
		</>
	);
}
