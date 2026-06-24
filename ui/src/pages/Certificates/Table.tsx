import { IconDotsVertical, IconDownload, IconEdit, IconRefresh, IconTrash } from "@tabler/icons-react";
import { createColumnHelper, getCoreRowModel, useReactTable } from "@tanstack/react-table";
import { useMemo } from "react";
import type { Certificate } from "src/api/backend";
import {
	CertificateInUseFormatter,
	CertificateProviderLabel,
	certificateProvider,
	CreatedOnFormatter,
	DateFormatter,
	DomainsFormatter,
	EmptyData,
	HasPermission,
} from "src/components";
import { TableLayout } from "src/components/Table/TableLayout";
import { intl, T } from "src/locale";
import { CERTIFICATES, MANAGE } from "src/modules/Permissions";

const certificateStatusLabel = (status?: string) => {
	switch (status) {
		case "issued":
			return (
				<span className="status status-green">
					<span className="status-dot" />
					<T id="certificate.status.issued" />
				</span>
			);
		case "expiring":
			return (
				<span className="status status-yellow">
					<span className="status-dot" />
					<T id="certificate.status.expiring" />
				</span>
			);
		case "expired":
			return (
				<span className="status status-red">
					<span className="status-dot" />
					<T id="certificate.status.expired" />
				</span>
			);
		case "failed":
			return (
				<span className="status status-red">
					<span className="status-dot" />
					<T id="certificate.status.failed" />
				</span>
			);
		case "pending":
			return (
				<span className="status status-yellow">
					<span className="status-dot" />
					<T id="certificate.status.pending" />
				</span>
			);
		case "disabled":
			return (
				<span className="status status-muted">
					<span className="status-dot" />
					<T id="disabled" />
				</span>
			);
		default:
			return (
				<span className="status status-green">
					<span className="status-dot" />
					<T id="certificate.status.issued" />
				</span>
			);
	}
};

const certificateFailureReason = (message?: string) => {
	if (!message) return null;
	return (
		<div
			className="text-danger small mt-1"
			style={{ maxWidth: 420, whiteSpace: "normal", wordBreak: "break-word" }}
		>
			{message}
		</div>
	);
};

const isFailedCertificate = (certificate: Certificate) => certificate.meta?.status === "failed";

const certificateValidationMethod = (certificate: Certificate) => {
	const method = String(certificate.meta?.signMethod || certificate.meta?.sign_method || "").toUpperCase();
	if (method.includes("DNS")) return "DNS";
	if (method.includes("HTTP")) return "HTTP";
	if (certificate.meta?.dnsChallenge) return "DNS";
	if (certificate.provider === "other") return "";
	return "HTTP";
};

interface Props {
	data: Certificate[];
	isFiltered?: boolean;
	isFetching?: boolean;
	onDelete?: (id: number) => void;
	onEdit?: (certificate: Certificate) => void;
	onRenew?: (id: number) => void;
	onDownload?: (id: number) => void;
}
export default function Table({ data, isFetching, onDelete, onEdit, onRenew, onDownload, isFiltered }: Props) {
	const columnHelper = createColumnHelper<Certificate>();
	const columns = useMemo(
		() => [
			columnHelper.accessor((row: any) => row, {
				id: "domainNames",
				header: intl.formatMessage({ id: "column.name" }),
				cell: (info: any) => {
					const value = info.getValue();
					return (
						<DomainsFormatter
							domains={value.domainNames}
							niceName={value.niceName}
							provider={value.provider || ""}
							linkDomains={false}
						/>
					);
				},
			}),
			columnHelper.accessor((row: any) => row.createdOn, {
				id: "createdOn",
				header: "创建时间",
				cell: (info: any) => <CreatedOnFormatter value={info.getValue()} />,
				meta: { className: "text-nowrap" },
			}),
			columnHelper.accessor((row: any) => row, {
				id: "provider",
				header: intl.formatMessage({ id: "column.provider" }),
				cell: (info: any) => {
					const r = info.getValue();
					const provider = certificateProvider(r);
					if (provider === "auto") {
						return <>Caddy 自动</>;
					}
					if (provider === "letsencrypt") {
						if (r.meta?.dnsChallenge && r.meta?.dnsProvider) {
							return (
								<>
									<T id="lets-encrypt" /> &ndash; {r.meta?.dnsProvider}
								</>
							);
						}
						return <T id="lets-encrypt" />;
					}
					if (provider === "other") {
						return <T id="certificates.custom" />;
					}
					return <CertificateProviderLabel provider={provider} />;
				},
			}),
			columnHelper.accessor((row: any) => row, {
				id: "validationMethod",
				header: "验证方式",
				cell: (info: any) => {
					const method = certificateValidationMethod(info.getValue());
					if (!method) {
						return <span className="text-muted">-</span>;
					}
					return <span className="badge bg-secondary-lt">{method}</span>;
				},
			}),
			columnHelper.accessor((row: any) => row.expiresOn, {
				id: "expiresOn",
				header: intl.formatMessage({ id: "column.expires" }),
				cell: (info: any) => {
					return <DateFormatter value={info.getValue()} highlightPast />;
				},
			}),
			columnHelper.accessor((row: any) => row, {
				id: "status",
				header: intl.formatMessage({ id: "column.status" }),
				cell: (info: any) => {
					const r = info.getValue();
					const status = r.meta?.status;
					return (
						<>
							{certificateStatusLabel(status)}
							{status === "failed"
								? certificateFailureReason(r.meta?.lastError || r.meta?.last_error)
								: null}
						</>
					);
				},
			}),
			columnHelper.accessor((row: any) => row, {
				id: "proxyHosts",
				header: intl.formatMessage({ id: "column.used-by" }),
				cell: (info: any) => {
					const r = info.getValue();
					return (
						<CertificateInUseFormatter
							proxyHosts={r.proxyHosts}
							redirectionHosts={r.redirectionHosts}
							deadHosts={r.deadHosts}
							streams={r.streams}
						/>
					);
				},
			}),
			columnHelper.display({
				id: "id",
				cell: (info: any) => {
					return (
						<span className="dropdown">
							<button
								type="button"
								className="btn dropdown-toggle btn-action btn-sm px-1"
								data-bs-boundary="viewport"
								data-bs-toggle="dropdown"
							>
								<IconDotsVertical />
							</button>
							<div className="dropdown-menu dropdown-menu-end">
								<span className="dropdown-header">
									<T
										id="object.actions-title"
										tData={{ object: "certificate" }}
										data={{ id: info.row.original.id }}
									/>
								</span>
								<a
									className="dropdown-item"
									href="#"
									onClick={(e) => {
										e.preventDefault();
										onRenew?.(info.row.original.id);
									}}
								>
									<IconRefresh size={16} />
									<T id={isFailedCertificate(info.row.original) ? "action.retry" : "action.renew"} />
								</a>
								<HasPermission section={CERTIFICATES} permission={MANAGE} hideError>
									<a
										className="dropdown-item"
										href="#"
										onClick={(e) => {
											e.preventDefault();
											onEdit?.(info.row.original);
										}}
									>
										<IconEdit size={16} />
										<T id="certificates.request.title" />
									</a>
									<a
										className="dropdown-item"
										href="#"
										onClick={(e) => {
											e.preventDefault();
											onDownload?.(info.row.original.id);
										}}
									>
										<IconDownload size={16} />
										<T id="action.download" />
									</a>
									<div className="dropdown-divider" />
									<a
										className="dropdown-item"
										href="#"
										onClick={(e) => {
											e.preventDefault();
											onDelete?.(info.row.original.id);
										}}
									>
										<IconTrash size={16} />
										<T id="action.delete" />
									</a>
								</HasPermission>
							</div>
						</span>
					);
				},
				meta: {
					className: "text-end w-1",
				},
			}),
		],
		[columnHelper, onDelete, onEdit, onRenew, onDownload],
	);

	const tableInstance = useReactTable<Certificate>({
		columns,
		data,
		getCoreRowModel: getCoreRowModel(),
		rowCount: data.length,
		meta: {
			isFetching,
		},
		enableSortingRemoval: false,
	});

	return (
		<TableLayout
			tableInstance={tableInstance}
			emptyState={
				<EmptyData
					object="certificate"
					objects="certificates"
					tableInstance={tableInstance}
					isFiltered={isFiltered}
					color="pink"
					permissionSection={CERTIFICATES}
				/>
			}
		/>
	);
}
