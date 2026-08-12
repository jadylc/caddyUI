import { Field } from "formik";
import { T } from "src/locale";
import { validateRootRedirectPath } from "./RootRedirectValidation";

const RootRedirectFields = () => (
	<div className="mt-3">
		<h4 className="py-2 mb-1">
			<T id="host.root-redirect.title" />
		</h4>
		<div className="row">
			<div className="col-md-8">
				<Field name="rootRedirectPath" validate={validateRootRedirectPath}>
					{({ field, form }: any) => (
						<div className="mb-3">
							<label className="form-label" htmlFor="rootRedirectPath">
								<T id="host.root-redirect.target" />
							</label>
							<input
								{...field}
								id="rootRedirectPath"
								type="text"
								className={`form-control ${form.errors.rootRedirectPath && form.touched.rootRedirectPath ? "is-invalid" : ""}`}
								placeholder="/d"
							/>
							<small className="text-secondary">
								<T id="host.root-redirect.help" />
							</small>
							{form.errors.rootRedirectPath && form.touched.rootRedirectPath ? (
								<div className="invalid-feedback">
									<T id={form.errors.rootRedirectPath} />
								</div>
							) : null}
						</div>
					)}
				</Field>
			</div>
			<div className="col-md-4">
				<Field name="rootRedirectCode">
					{({ field }: any) => (
						<div className="mb-3">
							<label className="form-label" htmlFor="rootRedirectCode">
								<T id="host.root-redirect.status-code" />
							</label>
							<select {...field} id="rootRedirectCode" className="form-control">
								<option value="302">
									<T id="host.root-redirect.status.302" />
								</option>
								<option value="301">
									<T id="host.root-redirect.status.301" />
								</option>
								<option value="307">
									<T id="host.root-redirect.status.307" />
								</option>
								<option value="308">
									<T id="host.root-redirect.status.308" />
								</option>
							</select>
						</div>
					)}
				</Field>
			</div>
		</div>
		<div className="border-start border-3 ps-3 small text-secondary">
			<T id="host.root-redirect.example" />
		</div>
	</div>
);

export { RootRedirectFields };
