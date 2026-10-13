import { IconHelp } from "@tabler/icons-react";
import { useMemo, useState } from "react";
import Alert from "react-bootstrap/Alert";
import { Button, LoadingPage } from "src/components";
import { useWafAudits } from "src/hooks";
import { T } from "src/locale";
import { showHelpModal, showWafAuditDetailsModal } from "src/modals";
import Table from "./Table";

export default function TableWrapper() {
	const [hostFilter, setHostFilter] = useState("");
	const { isFetching, isLoading, isError, error, data } = useWafAudits();

	const filtered = useMemo(() => {
		const items = data?.items ?? [];
		const keyword = hostFilter.trim().toLowerCase();
		if (!keyword) {
			return items;
		}
		return items.filter((item) => String(item.host || "").toLowerCase().includes(keyword));
	}, [data, hostFilter]);

	if (isLoading) {
		return <LoadingPage />;
	}

	if (isError) {
		return <Alert variant="danger">{error?.message || "Unknown error"}</Alert>;
	}

	return (
		<div className="card mt-4">
			<div className="card-status-top bg-lime" />
			<div className="card-table">
				<div className="card-header">
					<div className="row w-full">
						<div className="col">
							<h2 className="mt-1 mb-0">
								<T id="waf-audit" />
							</h2>
							<small className="text-secondary">
								记录 WAF 检测到的可疑请求；仅检测模式下不会被拦截
							</small>
						</div>
						<div className="col-auto d-flex align-items-center gap-2">
							<input
								type="text"
								className="form-control"
								placeholder="按主机过滤"
								value={hostFilter}
								onChange={(e) => setHostFilter(e.target.value)}
								style={{ width: 220 }}
							/>
							<Button
								size="sm"
								onClick={() => showHelpModal("WafAudit", "lime")}
								title="功能说明"
								ariaLabel="功能说明"
							>
								<IconHelp size={20} />
							</Button>
						</div>
					</div>
				</div>
				<Table data={filtered} isFetching={isFetching} onSelectItem={showWafAuditDetailsModal} />
			</div>
		</div>
	);
}