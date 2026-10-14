/**
 * WAF 审计表格列的展示辅助：消息归纳总结中文概述、严重度/动作中文化。
 * 仅用于表格单元渲染；后端返回的 message 等字段原文保持不变（详情弹窗展示原文）。
 */

// 攻击类别分组：按优先级顺序匹配规则消息文本，命中即归入该类别
const categoryGroups: { label: string; patterns: RegExp[] }[] = [
	{
		label: "SQL 注入攻击",
		patterns: [/sql injection/i, /\bsqli\b/i, /\bmysql\b/i, /\bmssql\b/i, /\boracle\b/i, /\bsqllfi\b/i],
	},
	{
		label: "XSS 注入攻击",
		patterns: [/xss/i, /javascript/i, /script tag/i, /noscript/i, /injectionchecker/i, /html injection/i, /attribute injection/i],
	},
	{
		label: "安全扫描器/爬虫探测",
		patterns: [/security scanner/i, /\bscanner\b/i, /generic http client/i, /\bscripting\b/i, /user-agent/i],
	},
	{
		label: "远程命令/代码执行",
		patterns: [/command injection/i, /code execution/i, /webshell/i, /remote command/i, /\brce\b/i],
	},
	{
		label: "文件包含/访问攻击",
		patterns: [/file inclusion/i, /\blfi\b/i, /\brfi\b/i, /restricted file/i, /file access/i],
	},
	{
		label: "PHP 注入攻击",
		patterns: [/php injection/i],
	},
	{
		label: "LDAP 注入攻击",
		patterns: [/ldap injection/i],
	},
	{
		label: "HTTP 协议攻击",
		patterns: [/request smuggling/i, /parameter pollution/i, /\bhpp\b/i, /\bcswsh\b/i],
	},
];

// 全部攻击类别（供筛选器选项使用，顺序即展示顺序）
export const WAF_ATTACK_CATEGORIES: string[] = categoryGroups.map((g) => g.label);

// 异常分汇总（coraza 尾部的 949110 汇总规则，如 "Inbound Anomaly Score Exceeded (Total Score: 20)"）
const anomalyScoreRe = /Total (?:Inbound )?Score: (\d+)/;

/**
 * 解析 message（同一请求的多条规则命中，以 " | " 分隔），
 * 返回命中的攻击类别（按 categoryGroups 顺序去重）；异常分汇总规则不计入类别。
 * 与 summarizeWafMessage 的分类逻辑保持一致，供筛选器复用。
 */
export const classifyWafMessage = (message?: string): string[] => {
	if (!message) {
		return [];
	}
	const parts = message
		.split(" | ")
		.map((p) => p.trim())
		.filter(Boolean);

	const matchedLabels: string[] = [];
	for (const part of parts) {
		if (anomalyScoreRe.test(part)) {
			continue;
		}
		for (const group of categoryGroups) {
			if (group.patterns.some((re) => re.test(part))) {
				if (!matchedLabels.includes(group.label)) {
					matchedLabels.push(group.label);
				}
				break;
			}
		}
	}
	return matchedLabels;
};

/**
 * 将后端返回的 message 归纳总结为一句中文概述：
 * 同类攻击去重合并、提取异常分汇总，未识别的规则不计入类别但保留命中条数。
 */
export const summarizeWafMessage = (message?: string): string => {
	const parts = (message ?? "")
		.split(" | ")
		.map((p) => p.trim())
		.filter(Boolean);
	if (parts.length === 0) {
		return "—";
	}

	const matchedLabels = classifyWafMessage(message);
	let totalScore: string | null = null;
	for (const part of parts) {
		const score = part.match(anomalyScoreRe);
		if (score) {
			totalScore = score[1];
		}
	}

	const tail = totalScore ? `，异常分超限：总分 ${totalScore}` : "";
	if (matchedLabels.length > 0) {
		return `检测到${matchedLabels.join("、")}（${parts.length} 条规则命中）${tail}`;
	}
	return `检测到可疑请求（${parts.length} 条规则命中）${tail}`;
};

// 严重度中文化
const severityDict: Record<string, string> = {
	critical: "严重",
	error: "错误",
	warning: "警告",
	notice: "提示",
	info: "信息",
	debug: "调试",
};

export const translateSeverity = (value?: string): string => {
	if (!value) return "—";
	return severityDict[value.toLowerCase()] || value;
};

// 动作中文化
const actionDict: Record<string, string> = {
	deny: "拦截",
	observe: "仅记录",
	log: "仅记录",
};

export const translateAction = (value?: string): string => {
	if (!value) return "—";
	return actionDict[value.toLowerCase()] || value;
};