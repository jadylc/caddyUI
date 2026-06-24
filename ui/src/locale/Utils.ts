import { fromUnixTime, type IntlFormatFormatOptions, intlFormat, parseISO } from "date-fns";

const isUnixTimestamp = (value: unknown): boolean => {
	if (typeof value !== "number" && typeof value !== "string") return false;
	const num = Number(value);
	if (!Number.isFinite(num)) return false;
	if (num > 0 && num < 10000000000) return true;
	if (num >= 10000000000 && num < 32503680000000) return true;
	return false;
};

const parseDate = (value: string | number): Date | null => {
	if (typeof value !== "number" && typeof value !== "string") return null;
	try {
		return isUnixTimestamp(value) ? fromUnixTime(+value) : parseISO(`${value}`);
	} catch {
		return null;
	}
};

const formatDateTime = (value: string | number): string => {
	const d = parseDate(value);
	if (!d) return `${value}`;
	try {
		return intlFormat(
			d,
			{
				dateStyle: "medium",
				timeStyle: "medium",
				hourCycle: "h23",
				timeZone: "Asia/Shanghai",
			} as IntlFormatFormatOptions,
			{ locale: "zh-CN" },
		);
	} catch {
		return `${value}`;
	}
};

export { formatDateTime, parseDate, isUnixTimestamp };
