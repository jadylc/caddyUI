import { useFormikContext } from "formik";
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import Modal from "react-bootstrap/Modal";
import { Button } from "src/components";
import { T } from "src/locale";

/** Renders nothing; syncs Formik `dirty` to the provided ref each render. */
function DirtySync({ dirtyRef }: { dirtyRef: React.MutableRefObject<boolean> }) {
	const { dirty } = useFormikContext();
	dirtyRef.current = dirty;
	return null;
}

interface ConfirmDiscardProps {
	show: boolean;
	onConfirm: () => void;
	onCancel: () => void;
}

function ConfirmDiscardModal({ show, onConfirm, onCancel }: ConfirmDiscardProps) {
	const [visible, setVisible] = useState(false);

	useEffect(() => {
		if (show) {
			requestAnimationFrame(() => setVisible(true));
		} else {
			setVisible(false);
		}
	}, [show]);

	if (!show) return null;

	return createPortal(
		<Modal show={visible} onHide={onCancel} backdrop="static" keyboard centered>
			<Modal.Header closeButton>
				<Modal.Title>
					<T id="confirm.discard.title" />
				</Modal.Title>
			</Modal.Header>
			<Modal.Body>
				<div className="text-center">
					<svg
						role="img"
						aria-label="warning icon"
						xmlns="http://www.w3.org/2000/svg"
						className="icon mb-2 text-warning icon-lg"
						width="24"
						height="24"
						viewBox="0 0 24 24"
						strokeWidth="2"
						stroke="currentColor"
						fill="none"
						strokeLinecap="round"
						strokeLinejoin="round"
					>
						<path stroke="none" d="M0 0h24v24H0z" fill="none" />
						<path d="M12 9v2m0 4v.01" />
						<path d="M5 19h14a2 2 0 0 0 1.84 -2.75l-7.1 -12.25a2 2 0 0 0 -3.5 0l-7.1 12.25a2 2 0 0 0 1.75 2.75" />
					</svg>
					<p className="mb-0">
						<T id="confirm.discard.message" />
					</p>
				</div>
			</Modal.Body>
			<Modal.Footer>
				<Button onClick={onCancel}>
					<T id="cancel" />
				</Button>
				<Button actionType="primary" className="ms-auto btn-red" onClick={onConfirm}>
					<T id="confirm.discard.confirm" />
				</Button>
			</Modal.Footer>
		</Modal>,
		document.body,
	);
}

export { ConfirmDiscardModal, DirtySync };
