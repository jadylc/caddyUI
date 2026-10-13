import CodeEditor from "@uiw/react-textarea-code-editor";
import EasyModal, { type InnerModalProps } from "ez-modal-react";
import Modal from "react-bootstrap/Modal";
import { Button } from "src/components";
import type { WafAuditEntry } from "src/api/backend";
import { T } from "src/locale";

const showWafAuditDetailsModal = (entry: WafAuditEntry) => {
	EasyModal.show(WafAuditDetailsModal, { entry });
};

interface Props extends InnerModalProps {
	entry: WafAuditEntry;
}
const WafAuditDetailsModal = EasyModal.create(({ entry, visible, remove }: Props) => {
	return (
		<Modal show={visible} onHide={remove} backdrop="static" keyboard size="lg">
			<Modal.Header closeButton>
				<Modal.Title>
					<T id="action.view-details" />
				</Modal.Title>
			</Modal.Header>
			<Modal.Body>
				<div className="row mb-3">
					<div className="col-6">
						<span className="text-secondary">主机：</span>
						{entry.host || "—"}
					</div>
					<div className="col-6">
						<span className="text-secondary">时间：</span>
						{entry.timestamp || "—"}
					</div>
				</div>
				<div className="row mb-3">
					<div className="col-6">
						<span className="text-secondary">客户端 IP：</span>
						{entry.remoteAddr || "—"}
					</div>
					<div className="col-6">
						<span className="text-secondary">事务 ID：</span>
						{entry.transactionId || "—"}
					</div>
				</div>
				<CodeEditor
					language="json"
					padding={15}
					data-color-mode="dark"
					minHeight={200}
					indentWidth={2}
					style={{
						fontFamily: "ui-monospace,SFMono-Regular,SF Mono,Consolas,Liberation Mono,Menlo,monospace",
						borderRadius: "0.3rem",
						minHeight: "200px",
						backgroundColor: "var(--tblr-bg-surface-dark)",
					}}
					readOnly
					value={JSON.stringify(entry, null, 2)}
				/>
			</Modal.Body>
			<Modal.Footer>
				<Button data-bs-dismiss="modal" onClick={remove}>
					<T id="action.close" />
				</Button>
			</Modal.Footer>
		</Modal>
	);
});

export { showWafAuditDetailsModal };