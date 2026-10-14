import { describe, expect, it } from "vitest";
import type { WafAuditEntry } from "src/api/backend";
import { filterWafAudits } from "./wafAuditFilter";

const entry = (overrides: Partial<WafAuditEntry>): WafAuditEntry => ({
	timestamp: "2026-10-14T10:00:00Z",
	host: "example.com",
	method: "GET",
	uri: "/",
	severity: "critical",
	action: "deny",
	message: "XSS Attack Detected via libinjection",
	...overrides,
});

const seed: WafAuditEntry[] = [
	entry({ host: "a.com", severity: "critical", action: "deny", message: "XSS Attack Detected via libinjection" }),
	entry({ host: "a.com", severity: "warning", action: "observe", message: "SQL Injection Attack Detected via libinjection" }),
	entry({ host: "b.com", severity: "notice", action: "log", message: "Inbound Anomaly Score Exceeded (Total Score: 20)" }),
	entry({ host: "b.com", severity: "critical", action: "deny", message: "Command Injection Detected" }),
];

describe("filterWafAudits", () => {
	it("全部未选时展示全部数据", () => {
		expect(filterWafAudits(seed, {})).toEqual(seed);
		expect(filterWafAudits(seed, { hosts: [], severities: [], actions: [], types: [] })).toEqual(seed);
	});

	it("单维度内多选取 OR", () => {
		const result = filterWafAudits(seed, { hosts: ["a.com", "b.com"] });
		expect(result).toHaveLength(4);
		const onlyA = filterWafAudits(seed, { hosts: ["a.com"] });
		expect(onlyA.map((i) => i.host)).toEqual(["a.com", "a.com"]);
	});

	it("多维度之间取 AND", () => {
		const result = filterWafAudits(seed, { hosts: ["a.com"], severities: ["critical"] });
		expect(result).toHaveLength(1);
		expect(result[0].message).toContain("XSS");
	});

	it("按严重度与动作过滤", () => {
		expect(filterWafAudits(seed, { severities: ["critical"] })).toHaveLength(2);
		expect(filterWafAudits(seed, { actions: ["observe", "log"] })).toHaveLength(2);
		expect(filterWafAudits(seed, { actions: ["deny"] })).toHaveLength(2);
	});

	it("按攻击类型过滤（消息归纳分类）", () => {
		const xss = filterWafAudits(seed, { types: ["XSS 注入攻击"] });
		expect(xss).toHaveLength(1);
		expect(xss[0].message).toContain("XSS");

		const sqlAndCmd = filterWafAudits(seed, { types: ["SQL 注入攻击", "远程命令/代码执行"] });
		expect(sqlAndCmd.map((i) => i.message).sort()).toEqual(["Command Injection Detected", "SQL Injection Attack Detected via libinjection"]);

		// 异常分汇总规则不计入任何类别
		expect(filterWafAudits(seed, { types: ["HTTP 协议攻击"] })).toHaveLength(0);
	});

	it("维度组合且无匹配时返回空数组", () => {
		expect(filterWafAudits(seed, { hosts: ["a.com"], types: ["SQL 注入攻击"] })).toHaveLength(1);
		expect(filterWafAudits(seed, { hosts: ["zzz.com"] })).toEqual([]);
	});
});