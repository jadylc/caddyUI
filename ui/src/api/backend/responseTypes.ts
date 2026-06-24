import type { AppVersion, Certificate } from "./models";

export interface HealthResponse {
	status: string;
	setup: boolean;
	version: AppVersion;
}

export interface VersionCheckResponse {
	current: string;
	latest: string;
	updateAvailable: boolean;
}

export interface ValidatedCertificateResponse {
	certificate: Certificate;
}
