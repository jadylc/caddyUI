import { IconHelp } from "@tabler/icons-react";
import { Field, Form, Formik } from "formik";
import { type ReactNode, useState } from "react";
import Alert from "react-bootstrap/Alert";
import { Button, LoadingPage } from "src/components";
import { useSystemSettings, useSetSystemSettings } from "src/hooks";
import { T } from "src/locale";
import { showHelpModal } from "src/modals";
import { showObjectSuccess } from "src/notifications";

export default function SystemSettingsPage() {
	const query = useSystemSettings();
	const { mutate: setSettings } = useSetSystemSettings();
	const [errorMsg, setErrorMsg] = useState<ReactNode | null>(null);
	const [isSubmitting, setIsSubmitting] = useState(false);
	const { data, isLoading, isError, error } = query;

	if (isLoading) return <LoadingPage />;
	if (isError) return <Alert variant="danger">{error?.message || "Unknown error"}</Alert>;
	if (!data) return null;

	return (
		<div className="card mt-4">
			<div className="card-status-top bg-lime" />
			<Formik
				initialValues={data}
				onSubmit={(values: any, { setSubmitting }: any) => {
					setIsSubmitting(true);
					setErrorMsg(null);
					setSettings(values, {
						onError: (err: any) => setErrorMsg(err.message),
						onSuccess: () => showObjectSuccess("system-settings", "saved"),
						onSettled: () => {
							setIsSubmitting(false);
							setSubmitting(false);
						},
					});
				}}
			>
				{() => (
					<Form>
						<div className="card-header">
							<div className="row w-full align-items-center">
								<div className="col">
									<h2 className="card-title mb-0">
										<T id="system-settings" />
									</h2>
								</div>
								<div className="col-auto">
									<Button
										size="sm"
										onClick={() => showHelpModal("SystemSettings", "lime")}
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
							<Field name="acmeContactEmail">
								{({ field }: any) => (
									<div className="mb-3">
										<label className="form-label" htmlFor="acmeContactEmail">
											<T id="settings.acme-email" />
											<IconHelp
												size={16}
												className="ms-1 text-secondary cursor-pointer"
												onClick={() =>
													showHelpModal(
														"ACMEEmail",
														"lime",
													)
												}
											/>
										</label>
										<input
											id="acmeContactEmail"
											className="form-control"
											placeholder="noreply-random@localhost"
											{...field}
										/>
										<small className="text-secondary">
											<T id="settings.acme-email.help" />
										</small>
									</div>
								)}
							</Field>
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
	);
}
