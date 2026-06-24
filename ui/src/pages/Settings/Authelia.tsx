import { IconHelp } from "@tabler/icons-react";
import { Field, Form, Formik } from "formik";
import { type ReactNode, useState } from "react";
import Alert from "react-bootstrap/Alert";
import { useLocation } from "react-router-dom";
import type { AuthentikSettings, AutheliaSettings } from "src/api/backend";
import { Button, LoadingPage } from "src/components";
import { forwardAuthProviders, type ForwardAuthProviderID } from "src/forwardAuthProviders";
import { useForwardAuthSettings, useSetForwardAuthSettings } from "src/hooks";
import { intl, T } from "src/locale";
import { showHelpModal } from "src/modals";
import { showObjectSuccess } from "src/notifications";

const validateAutheliaUrl = (value: string) => {
	try {
		const url = new URL(value);
		return url.protocol === "http:" || url.protocol === "https:"
			? undefined
			: intl.formatMessage({ id: "error.invalid-url" });
	} catch {
		return intl.formatMessage({ id: "error.invalid-url" });
	}
};

const validateAuthUri = (value: string) =>
	value?.startsWith("/") ? undefined : intl.formatMessage({ id: "error.invalid-uri" });

export default function AutheliaSettingsPage() {
	const { pathname } = useLocation();
	const provider = (pathname.includes("authentik") ? "authentik" : "authelia") as ForwardAuthProviderID;
	const spec = forwardAuthProviders[provider];
	const query = useForwardAuthSettings<AutheliaSettings | AuthentikSettings>(provider);
	const { mutate: setSettings } = useSetForwardAuthSettings<AutheliaSettings | AuthentikSettings>(provider);
	const [errorMsg, setErrorMsg] = useState<ReactNode | null>(null);
	const [isSubmitting, setIsSubmitting] = useState(false);
	const { data, isLoading, isError, error } = query;
	const objectName = provider;

	if (isLoading) return <LoadingPage />;
	if (isError) return <Alert variant="danger">{error?.message || "Unknown error"}</Alert>;
	if (!data) return null;

	return (
		<>
			<div className="card mt-4">
				<div className="card-body">
					<p className="text-secondary mb-2">
						<T id={spec.descriptionId} />
					</p>
					<a href={spec.docsUrl} target="_blank" rel="noopener noreferrer">
						{spec.docsUrl}
					</a>
				</div>
			</div>
			<div className="card mt-3">
				<div className="card-status-top bg-lime" />
				<Formik
					initialValues={{
						...data,
						copyHeaders: data.copyHeaders.join("\n"),
					}}
					onSubmit={(values: any, { setSubmitting }: any) => {
						setIsSubmitting(true);
						setErrorMsg(null);
						setSettings(
							{
								...values,
								copyHeaders: String(values.copyHeaders || "")
									.split(/[\n,]+/)
									.map((header) => header.trim())
									.filter(Boolean),
							},
							{
								onError: (err: any) => setErrorMsg(err.message),
								onSuccess: () => showObjectSuccess(objectName, "saved"),
								onSettled: () => {
									setIsSubmitting(false);
									setSubmitting(false);
								},
							},
						);
					}}
				>
					{() => (
						<Form>
							<div className="card-header">
								<div className="row w-full align-items-center">
									<div className="col">
										<h2 className="card-title mb-0">
											<T id={spec.settingsLabelId} />
										</h2>
									</div>
									<div className="col-auto">
										<Button
											size="sm"
											onClick={() =>
												showHelpModal(
													provider === "authentik" ? "Authentik" : "Authelia",
													"lime",
												)
											}
											title="功能说明"
											ariaLabel="功能说明"
										>
											<IconHelp size={20} />
										</Button>
									</div>
								</div>
							</div>
							<div className="card-body">
								<Alert variant="danger" show={!!errorMsg} onClose={() => setErrorMsg(null)} dismissible>
									{errorMsg}
								</Alert>
								<div className="mb-3">
									<label className="row" htmlFor={`${provider}Enabled`}>
										<span className="col">
											<T id={spec.enabledId} />
										</span>
										<span className="col-auto">
											<Field name="enabled" type="checkbox">
												{({ field }: any) => (
													<label className="form-check form-check-single form-switch">
														<input
															{...field}
															id={`${provider}Enabled`}
															className="form-check-input bg-lime"
															type="checkbox"
														/>
													</label>
												)}
											</Field>
										</span>
									</label>
								</div>
								<Field name="upstream" validate={validateAutheliaUrl}>
									{({ field, form }: any) => (
										<div className="mb-3">
											<label className="form-label" htmlFor={`${provider}Upstream`}>
												<T id="forward-auth.upstream" />
											</label>
											<input
												id={`${provider}Upstream`}
												className={`form-control ${form.errors.upstream && form.touched.upstream ? "is-invalid" : ""}`}
												placeholder={spec.upstreamPlaceholder}
												{...field}
											/>
											<small className="text-secondary">
												<T id={spec.upstreamHelpId} />
											</small>
											{form.errors.upstream && form.touched.upstream ? (
												<div className="invalid-feedback">{form.errors.upstream}</div>
											) : null}
										</div>
									)}
								</Field>
								<Field name="uri" validate={validateAuthUri}>
									{({ field, form }: any) => (
										<div className="mb-3">
											<label className="form-label" htmlFor={`${provider}Uri`}>
												<T id="forward-auth.uri" />
											</label>
											<input
												id={`${provider}Uri`}
												className={`form-control ${form.errors.uri && form.touched.uri ? "is-invalid" : ""}`}
												placeholder={spec.uriPlaceholder}
												{...field}
											/>
											{form.errors.uri && form.touched.uri ? (
												<div className="invalid-feedback">{form.errors.uri}</div>
											) : null}
										</div>
									)}
								</Field>
								<Field name="copyHeaders">
									{({ field }: any) => (
										<div className="mb-3">
											<label className="form-label" htmlFor={`${provider}CopyHeaders`}>
												<T id="forward-auth.copy-headers" />
											</label>
											<textarea
												id={`${provider}CopyHeaders`}
												className="form-control"
												rows={4}
												{...field}
											/>
										</div>
									)}
								</Field>
								{spec.supportsFailOpen ? (
									<>
										<div className="mb-3">
											<label className="row" htmlFor="autheliaFailOpen">
												<span className="col">
													<T id="authelia.fail-open" />
													<div className="text-secondary small">
														<T id="authelia.fail-open.help" />
													</div>
												</span>
												<span className="col-auto">
													<Field name="failOpen" type="checkbox">
														{({ field }: any) => (
															<label className="form-check form-check-single form-switch">
																<input
																	{...field}
																	id="autheliaFailOpen"
																	className="form-check-input bg-lime"
																	type="checkbox"
																/>
															</label>
														)}
													</Field>
												</span>
											</label>
										</div>
										<Alert variant="warning">
											<T id="authelia.fail-open.warning" />
										</Alert>
									</>
								) : (
									<Alert variant="info">
										<T
											id={spec.summaryEnabledId}
											tData={{
												endpoint: `${String(data.upstream || "").replace(/\/$/, "")}${data.uri || ""}`,
											}}
										/>
									</Alert>
								)}
							</div>
							<div className="card-footer text-end">
								<Button
									type="submit"
									actionType="primary"
									className="bg-lime"
									isLoading={isSubmitting}
									disabled={isSubmitting}
								>
									<T id="save" />
								</Button>
							</div>
						</Form>
					)}
				</Formik>
			</div>
		</>
	);
}
