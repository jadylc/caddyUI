import EasyModal, { type InnerModalProps } from "ez-modal-react";
import { useState } from "react";
import { Alert } from "react-bootstrap";
import Modal from "react-bootstrap/Modal";
import { useQuery } from "@tanstack/react-query";
import { Button, Loading } from "src/components";
import { downloadSiteAccessLog, getSiteAccessLogs, type SiteAccessLogKind } from "src/api/backend";
import { T } from "src/locale";

const showSiteAccessLogModal = (kind: SiteAccessLogKind, id: number, title: string) => {
	EasyModal.show(SiteAccessLogModal, { kind, id, title } as ShowProps);
};

interface Props extends InnerModalProps {
	kind: SiteAccessLogKind;
	id: number;
	title: string;
}

interface ShowProps {
	kind: SiteAccessLogKind;
	id: number;
	title: string;
}

const SiteAccessLogModal = EasyModal.create(({ kind, id, title, visible, remove }: Props) => {
	const [errorMsg, setErrorMsg] = useState("");
	const { data, isLoading, error, refetch, isFetching } = useQuery({
		queryKey: ["site-access-log", kind, id],
		queryFn: () => getSiteAccessLogs(kind, id, 500),
		enabled: visible,
	});

	const handleDownload = async () => {
		setErrorMsg("");
		try {
			await downloadSiteAccessLog(kind, id);
		} catch (err: any) {
			setErrorMsg(err?.message || "download failed");
		}
	};

	const lines = data?.lines ?? [];

	return (
		<Modal show={visible} onHide={remove} backdrop="static" keyboard size="lg">
			<Modal.Header closeButton>
				<Modal.Title>访问日志 - {title}</Modal.Title>
			</Modal.Header>
			<Modal.Body>
				<Alert variant="danger" show={!!error || !!errorMsg}>
					{errorMsg || error?.message}
				</Alert>
				{isLoading ? (
					<Loading noLogo />
				) : (
					<>
						<div className="d-flex align-items-center justify-content-between mb-2">
							<div className="text-secondary text-truncate">{data?.path || ""}</div>
							<div className="btn-list">
								<Button size="sm" onClick={() => refetch()} disabled={isFetching}>
									刷新
								</Button>
								<Button size="sm" onClick={handleDownload}>
									<T id="action.download" />
								</Button>
							</div>
						</div>
						<pre
							className="border rounded bg-dark text-white p-3 mb-0"
							style={{ maxHeight: "55vh", overflow: "auto", whiteSpace: "pre-wrap", fontSize: 12 }}
						>
							{lines.length ? lines.join("\n") : "暂无日志"}
						</pre>
					</>
				)}
			</Modal.Body>
			<Modal.Footer>
				<Button data-bs-dismiss="modal" onClick={remove}>
					<T id="cancel" />
				</Button>
			</Modal.Footer>
		</Modal>
	);
});

export { showSiteAccessLogModal };
