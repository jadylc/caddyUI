import { IconRefresh, IconTerminal2, IconX } from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState } from "react";
import { getLogs } from "src/api/backend";
import { T } from "src/locale";
import { Button } from "./Button";
import styles from "./LogPanel.module.css";

export function LogPanel() {
	const [open, setOpen] = useState(false);
	const [autoScroll, setAutoScroll] = useState(true);
	const bodyRef = useRef<HTMLDivElement | null>(null);
	const { data, isFetching, refetch } = useQuery({
		queryKey: ["runtime-logs"],
		queryFn: () => getLogs(220),
		enabled: open,
		refetchInterval: open ? 5000 : false,
	});
	const lines = useMemo(() => [...(data || [])].reverse(), [data]);
	const latestLogLine = data?.[data.length - 1] ?? "";

	useEffect(() => {
		if (autoScroll && (latestLogLine || lines.length === 0)) {
			requestAnimationFrame(() => {
				bodyRef.current?.scrollTo({ top: 0 });
			});
		}
	}, [autoScroll, latestLogLine, lines.length]);

	const handleAutoScrollChange = (checked: boolean) => {
		setAutoScroll(checked);
		if (checked) {
			requestAnimationFrame(() => {
				bodyRef.current?.scrollTo({ top: 0, behavior: "smooth" });
			});
		}
	};

	return (
		<>
			<Button
				className={styles.toggle}
				size="sm"
				variant="ghost"
				data-bs-toggle="tooltip"
				data-bs-placement="bottom"
				aria-label="Logs"
				data-bs-original-title="Logs"
				onClick={() => setOpen(true)}
			>
				<IconTerminal2 size={24} />
			</Button>
			{open ? (
				<div className={styles.panel}>
					<div className="card-header py-2">
						<h3 className="card-title">
							<T id="logs" />
						</h3>
						<div className="card-actions">
							<label className={`form-check form-switch ${styles.autoScroll}`}>
								<input
									className="form-check-input"
									type="checkbox"
									checked={autoScroll}
									onChange={(e) => handleAutoScrollChange(e.target.checked)}
								/>
								<span className="form-check-label">
									<T id="logs.auto-scroll" />
								</span>
							</label>
							<Button size="sm" variant="ghost" isLoading={isFetching} onClick={() => refetch()}>
								<IconRefresh size={18} />
							</Button>
							<Button size="sm" variant="ghost" onClick={() => setOpen(false)}>
								<IconX size={18} />
							</Button>
						</div>
					</div>
					<div ref={bodyRef} className={`card-body bg-dark text-light ${styles.body}`}>
						{lines.length ? (
							lines.map((line, index) => (
								<div className={styles.line} key={`${line}-${index}`}>
									{line}
								</div>
							))
						) : (
							<T id="logs.empty" />
						)}
					</div>
				</div>
			) : null}
		</>
	);
}
