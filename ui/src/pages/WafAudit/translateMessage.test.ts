import { describe, expect, it } from "vitest";
import {
	classifyWafMessage,
	summarizeWafMessage,
	translateAction,
	translateSeverity,
	WAF_ATTACK_CATEGORIES,
} from "./translateMessage";

describe("summarizeWafMessage", () => {
	it("将多条同类攻击规则归纳为一句中文概述，含命中条数与异常分", () => {
		const msg =
			"XSS Attack Detected via libinjection | XSS Filter - Category 1: Script Tag Vector | NoScript XSS InjectionChecker: HTML Injection | Javascript method detected | Inbound Anomaly Score Exceeded (Total Score: 20)";
		expect(summarizeWafMessage(msg)).toBe("检测到XSS 注入攻击（5 条规则命中），异常分超限：总分 20");
	});

	it("同类攻击去重合并", () => {
		const msg =
			"SQL Injection Attack Detected via libinjection | SQL Injection Attack Detected via libinjection 2 | Cross-site Scripting (XSS) Attack Detected";
		expect(summarizeWafMessage(msg)).toBe("检测到SQL 注入攻击、XSS 注入攻击（3 条规则命中）");
	});

	it("未识别的规则归纳为可疑请求", () => {
		expect(summarizeWafMessage("Some Unknown Rule Message")).toBe("检测到可疑请求（1 条规则命中）");
	});

	it("空值返回占位符", () => {
		expect(summarizeWafMessage(undefined)).toBe("—");
		expect(summarizeWafMessage("")).toBe("—");
	});
});

describe("classifyWafMessage", () => {
	it("返回命中的攻击类别（去重、按定义顺序）", () => {
		const msg = "SQL Injection Attack Detected | Cross-site Scripting (XSS) Attack Detected | SQL Injection Attack 2";
		expect(classifyWafMessage(msg)).toEqual(["SQL 注入攻击", "XSS 注入攻击"]);
	});

	it("异常分汇总规则不计入类别", () => {
		expect(classifyWafMessage("Inbound Anomaly Score Exceeded (Total Score: 20)")).toEqual([]);
	});

	it("空值返回空数组", () => {
		expect(classifyWafMessage(undefined)).toEqual([]);
		expect(classifyWafMessage("")).toEqual([]);
	});
});

describe("WAF_ATTACK_CATEGORIES", () => {
	it("包含全部 8 个攻击类别", () => {
		expect(WAF_ATTACK_CATEGORIES).toEqual([
			"SQL 注入攻击",
			"XSS 注入攻击",
			"安全扫描器/爬虫探测",
			"远程命令/代码执行",
			"文件包含/访问攻击",
			"PHP 注入攻击",
			"LDAP 注入攻击",
			"HTTP 协议攻击",
		]);
	});
});

describe("translateSeverity", () => {
	it("将严重度映射为中文", () => {
		expect(translateSeverity("critical")).toBe("严重");
		expect(translateSeverity("error")).toBe("错误");
		expect(translateSeverity("warning")).toBe("警告");
		expect(translateSeverity("notice")).toBe("提示");
		expect(translateSeverity("info")).toBe("信息");
		expect(translateSeverity("debug")).toBe("调试");
	});

	it("大小写不敏感，未知值原样返回", () => {
		expect(translateSeverity("CRITICAL")).toBe("严重");
		expect(translateSeverity("weird")).toBe("weird");
		expect(translateSeverity(undefined)).toBe("—");
	});
});

describe("translateAction", () => {
	it("将动作映射为中文", () => {
		expect(translateAction("deny")).toBe("拦截");
		expect(translateAction("observe")).toBe("仅记录");
		expect(translateAction("log")).toBe("仅记录");
	});

	it("未知值原样返回，空值返回占位符", () => {
		expect(translateAction("allow")).toBe("allow");
		expect(translateAction(undefined)).toBe("—");
	});
});