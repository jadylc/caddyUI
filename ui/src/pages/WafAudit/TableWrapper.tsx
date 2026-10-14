import { IconHelp } from "@tabler/icons-react";
import { useMemo, useState } from "react";
import Alert from "react-bootstrap/Alert";
import Select from "react-select";
import { Button, LoadingPage } from "src/components";
import { useWafAudits } from "src/hooks";
import { T } from "src/locale";
import { showHelpModal, showWafAuditDetailsModal } from "src/modals";
import Table from "./Table";
import { translateAction, translateSeverity, WAF_ATTACK_CATEGORIES } from "./translateMessage";
import { filterWafAudits, type WafAuditFilters } from "./wafAuditFilter";

interface FilterOption {
	readonly value: string;
	readonly label: string;
}

// 从数据中提取去重后的值作为选项；label 可应用中文化映射
const toOptions = (values: (string | undefined)[], labelOf?: (value: string) => string): FilterOption[] =>
	[...new Set(values.filter((v): v is string => Boolean(v)))].sort().map((value) => ({
		value,
		label: labelOf ? labelOf(value) : value,
	}));

const selectStyles = {
	control: (base: any) => ({
		...base,
		minHeight: "calc(1.5em + 0.75rem + 2px)",
	}),
};

export default function TableWrapper() {
	const [selectedHosts, setSelectedHosts] = useState<FilterOption[]>([]);
	const [selectedSeverities, setSelectedSeverities] = useState<FilterOption[]>([]);
	const [selectedActions, setSelectedActions] = useState<FilterOption[]>([]);
	const [selectedTypes, setSelectedTypes] = useState<FilterOption[]>([]);
	const { isFetching, isLoading, isError, error, data } = useWafAudits();

	const items = data?.items ?? [];

	// 选项来源：主机/严重度/动作取自当前数据出现的值；类型取自消息归纳分类
	const options = useMemo(
		() => ({
			hosts: toOptions(items.map((item) => item.host)),
			severities: toOptions(items.map((item) => item.severity), translateSeverity),
			actions: toOptions(items.map((item) => item.action), translateAction),
			types: WAF_ATTACK_CATEGORIES.map((label) => ({ value: label, label })),
		}),
		[items],
	);

	// 组合过滤：不同维度取 AND、同一维度内取 OR；全部未选时展示全部
	const filtered = useMemo(() => {
		const filters: WafAuditFilters = {
			hosts: selectedHosts.map((o) => o.value),
			severities: selectedSeverities.map((o) => o.value),
			actions: selectedActions.map((o) => o.value),
			types: selectedTypes.map((o) => o.value),
		};
		return filterWafAudits(items, filters);
	}, [items, selectedHosts, selectedSeverities, selectedActions, selectedTypes]);

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
				<div className="card-body border-bottom py-3">
					<div className="row g-2">
						<div className="col-6 col-sm-4 col-lg-3">
							<label className="form-label mb-1">主机</label>
							<Select
								className="react-select-container"
								classNamePrefix="react-select"
								isMulti
								options={options.hosts}
								value={selectedHosts}
								onChange={(value) => setSelectedHosts(value as FilterOption[])}
								placeholder="全部主机"
								styles={selectStyles}
								menuPortalTarget={document.body}
								isClearable
							/>
						</div>
						<div className="col-6 col-sm-4 col-lg-3">
							<label className="form-label mb-1">严重度</label>
							<Select
								className="react-select-container"
								classNamePrefix="react-select"
								isMulti
								options={options.severities}
								value={selectedSeverities}
								onChange={(value) => setSelectedSeverities(value as FilterOption[])}
								placeholder="全部严重度"
								styles={selectStyles}
								menuPortalTarget={document.body}
								isClearable
							/>
						</div>
						<div className="col-6 col-sm-4 col-lg-3">
							<label className="form-label mb-1">动作</label>
							<Select
								className="react-select-container"
								classNamePrefix="react-select"
								isMulti
								options={options.actions}
								value={selectedActions}
								onChange={(value) => setSelectedActions(value as FilterOption[])}
								placeholder="全部动作"
								styles={selectStyles}
								menuPortalTarget={document.body}
								isClearable
							/>
						</div>
						<div className="col-6 col-sm-4 col-lg-3">
							<label className="form-label mb-1">类型</label>
							<Select
								className="react-select-container"
								classNamePrefix="react-select"
								isMulti
								options={options.types}
								value={selectedTypes}
								onChange={(value) => setSelectedTypes(value as FilterOption[])}
								placeholder="全部类型"
								styles={selectStyles}
								menuPortalTarget={document.body}
								isClearable
							/>
						</div>
					</div>
				</div>
				<Table
					data={filtered}
					isFetching={isFetching}
					onSelectItem={showWafAuditDetailsModal}
					emptyState={
						<tr>
							<td colSpan={8} className="text-center text-secondary py-4">
								没有匹配的记录
							</td>
						</tr>
					}
				/>
			</div>
		</div>
	);
}