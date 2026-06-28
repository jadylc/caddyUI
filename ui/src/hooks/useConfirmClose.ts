import type { RefObject } from "react";
import { useCallback, useRef, useState } from "react";

/**
 * Manages the "ESC to close with unsaved changes" flow.
 * @param onClose - the actual close/remove callback
 * @param dirtyRef - optional external ref for dirty state (used with Formik DirtySync)
 *   If not provided, one is created internally (for non-Formik modals).
 */
function useConfirmClose(onClose: () => void, dirtyRef?: RefObject<boolean>) {
	const internalRef = useRef(false);
	const ref = dirtyRef || internalRef;

	const [showConfirm, setShowConfirm] = useState(false);

	const handleClose = useCallback(() => {
		if (ref.current) {
			setShowConfirm(true);
		} else {
			onClose();
		}
	}, [ref, onClose]);

	const handleConfirm = useCallback(() => {
		setShowConfirm(false);
		onClose();
	}, [onClose]);

	const handleCancel = useCallback(() => {
		setShowConfirm(false);
	}, []);

	return { handleClose, showConfirm, handleConfirm, handleCancel, dirtyRef: ref };
}

export { useConfirmClose };
