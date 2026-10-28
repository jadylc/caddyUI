import * as api from "./base";

export interface HealthCheckTestInput {
	upstream: string;
	uri?: string;
	status?: string;
	timeout?: string;
	host?: string;
}

export interface HealthCheckTestResult {
	ok: boolean;
	url: string;
	status?: number;
	expected?: string;
	timeout?: string;
	message?: string;
	error?: string;
}

// testHealthCheck 在保存前探测上游的健康检查路径。
// 主动健康检查一旦指向后端不返回 2xx 的路径，会按检查间隔持续产生错误日志，
// 这个接口让界面能在保存前看到真实状态码。
export async function testHealthCheck(data: HealthCheckTestInput): Promise<HealthCheckTestResult> {
	return await api.post({
		url: "/caddy/proxy-hosts/health-check/test",
		data,
	});
}