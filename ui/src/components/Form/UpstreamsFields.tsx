import cn from "classnames";
import { getIn, useFormikContext } from "formik";
import type { ProxyUpstream } from "src/api/backend";
import { T } from "src/locale";

export const loadBalancingPolicies = [
	{ value: "", labelId: "load-balancing.policy.default" },
	{ value: "random", labelId: "load-balancing.policy.random" },
	{ value: "round_robin", labelId: "load-balancing.policy.round-robin" },
	{ value: "weighted_round_robin", labelId: "load-balancing.policy.weighted-round-robin" },
	{ value: "least_conn", labelId: "load-balancing.policy.least-conn" },
	{ value: "first", labelId: "load-balancing.policy.first" },
	{ value: "ip_hash", labelId: "load-balancing.policy.ip-hash" },
	{ value: "client_ip_hash", labelId: "load-balancing.policy.client-ip-hash" },
	{ value: "uri_hash", labelId: "load-balancing.policy.uri-hash" },
];

interface Props {
	name?: string;
	policyName?: string;
	showPolicy?: boolean;
}

const blankUpstream = (): ProxyUpstream => ({
	forwardScheme: "http",
	forwardHost: "",
	forwardPort: 80,
	weight: 1,
});

const normalizeUpstreams = (value: any): ProxyUpstream[] => {
	if (!Array.isArray(value) || value.length === 0) {
		return [blankUpstream()];
	}
	return value.map((item) => ({
		forwardScheme: item?.forwardScheme === "https" ? "https" : "http",
		forwardHost: item?.forwardHost || "",
		forwardPort: Number(item?.forwardPort || (item?.forwardScheme === "https" ? 443 : 80)),
		weight: Number(item?.weight || 1),
	}));
};

const upstreamHasError = (item: ProxyUpstream) => {
	if (!String(item.forwardHost || "").trim()) return true;
	if (!Number.isInteger(Number(item.forwardPort)) || Number(item.forwardPort) < 1 || Number(item.forwardPort) > 65535) {
		return true;
	}
	if (item.weight !== undefined && (!Number.isInteger(Number(item.weight)) || Number(item.weight) < 0)) {
		return true;
	}
	return false;
};

export function UpstreamsFields({ name = "upstreams", policyName = "loadBalancingPolicy", showPolicy = true }: Props) {
	const { values, setFieldValue, setFieldTouched, submitCount } = useFormikContext<any>();
	const upstreams = normalizeUpstreams(getIn(values, name));
	const policy = String(getIn(values, policyName) || "");
	const showErrors = submitCount > 0;

	const setUpstreams = (next: ProxyUpstream[]) => {
		setFieldValue(name, next);
	};

	const handleChange = (idx: number, field: keyof ProxyUpstream, value: string) => {
		const parsed =
			field === "forwardPort" || field === "weight"
				? value === ""
					? undefined
					: Number(value)
				: value;
		const next = upstreams.map((item, itemIdx) => (itemIdx === idx ? { ...item, [field]: parsed } : item));
		if (field === "forwardScheme" && (value === "http" || value === "https")) {
			const currentPort = Number(next[idx].forwardPort || 0);
			if ((value === "http" && currentPort === 443) || (value === "https" && currentPort === 80) || currentPort <= 0) {
				next[idx].forwardPort = value === "https" ? 443 : 80;
			}
		}
		setUpstreams(next);
		setFieldTouched(name, true, false);
	};

	const handleAdd = () => {
		setUpstreams([...upstreams, blankUpstream()]);
		setFieldTouched(name, true, false);
	};

	const handleRemove = (idx: number) => {
		const next = upstreams.filter((_, itemIdx) => itemIdx !== idx);
		setUpstreams(next.length ? next : [blankUpstream()]);
		setFieldTouched(name, true, false);
	};

	return (
		<div className="mb-3">
			{showPolicy ? (
				<div className="mb-3">
					<label className="form-label" htmlFor={policyName}>
						<T id="load-balancing.policy" />
					</label>
					<select
						id={policyName}
						className="form-control"
						value={policy}
						onChange={(event) => setFieldValue(policyName, event.target.value)}
					>
						{loadBalancingPolicies.map((item) => (
							<option key={item.value || "default"} value={item.value}>
								<T id={item.labelId} />
							</option>
						))}
					</select>
				</div>
			) : null}
			<div className="d-flex flex-column gap-2">
				{upstreams.map((item, idx) => {
					const hasError = showErrors && upstreamHasError(item);
					return (
						<div className="row g-2 align-items-start" key={idx}>
							<div className="col-md-2">
								<label className="form-label" htmlFor={`${name}-${idx}-scheme`}>
									<T id="host.forward-scheme" />
								</label>
								<select
									id={`${name}-${idx}-scheme`}
									className="form-control"
									value={item.forwardScheme}
									onChange={(event) => handleChange(idx, "forwardScheme", event.target.value)}
								>
									<option value="http">http</option>
									<option value="https">https</option>
								</select>
							</div>
							<div className="col-md-5">
								<label className="form-label" htmlFor={`${name}-${idx}-host`}>
									<T id="proxy-host.forward-host" />
								</label>
								<input
									id={`${name}-${idx}-host`}
									type="text"
									className={cn("form-control", hasError && !String(item.forwardHost || "").trim() && "is-invalid")}
									required
									placeholder="eg: 10.0.0.1"
									value={item.forwardHost}
									onChange={(event) => handleChange(idx, "forwardHost", event.target.value)}
								/>
							</div>
							<div className="col-md-2">
								<label className="form-label" htmlFor={`${name}-${idx}-port`}>
									<T id="host.forward-port" />
								</label>
								<input
									id={`${name}-${idx}-port`}
									type="number"
									min={1}
									max={65535}
									className={cn(
										"form-control",
										hasError &&
											(!Number.isInteger(Number(item.forwardPort)) ||
												Number(item.forwardPort) < 1 ||
												Number(item.forwardPort) > 65535) &&
											"is-invalid",
									)}
									required
									value={item.forwardPort ?? ""}
									onChange={(event) => handleChange(idx, "forwardPort", event.target.value)}
								/>
							</div>
							<div className="col-md-2">
								<label className="form-label" htmlFor={`${name}-${idx}-weight`}>
									<T id="load-balancing.weight" />
								</label>
								<input
									id={`${name}-${idx}-weight`}
									type="number"
									min={0}
									className={cn(
										"form-control",
										hasError &&
											item.weight !== undefined &&
											(!Number.isInteger(Number(item.weight)) || Number(item.weight) < 0) &&
											"is-invalid",
									)}
									value={item.weight ?? ""}
									onChange={(event) => handleChange(idx, "weight", event.target.value)}
								/>
							</div>
							<div className="col-md-1 d-flex align-items-end">
								<button
									type="button"
									className="btn btn-icon"
									onClick={() => handleRemove(idx)}
									disabled={upstreams.length === 1}
									aria-label="delete upstream"
								>
									&times;
								</button>
							</div>
							{hasError ? (
								<div className="col-12 text-danger small">
									<T id="load-balancing.upstream.invalid" />
								</div>
							) : null}
						</div>
					);
				})}
			</div>
			<button type="button" className="btn btn-sm mt-2" onClick={handleAdd}>
				<T id="load-balancing.add-upstream" />
			</button>
		</div>
	);
}
