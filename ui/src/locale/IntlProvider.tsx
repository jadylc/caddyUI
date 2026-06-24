import langZh from "./src/zh.json";

const normalizeMessages = (messages: Record<string, { defaultMessage?: string } | string>) => {
	const out: Record<string, string> = {};
	Object.entries(messages).forEach(([key, value]) => {
		out[key] = typeof value === "string" ? value : value.defaultMessage || key;
	});
	return out;
};

const messages: Record<string, string> = normalizeMessages(langZh);

const t = (id: string, data?: Record<string, string | number | undefined>): string => {
	let msg = messages[id] || id;
	if (data) {
		Object.entries(data).forEach(([key, value]) => {
			msg = msg.replace(new RegExp(`\\{${key}\\}`, "g"), String(value));
		});
	}
	return msg;
};

const intl = {
	formatMessage: (descriptor: { id: string }, data?: Record<string, string | number | undefined>) => {
		return t(descriptor.id, data);
	},
};

const T = ({
	id,
	data,
	tData,
}: {
	id: string;
	data?: Record<string, string | number | undefined>;
	tData?: Record<string, string>;
}) => {
	const translatedData: Record<string, string> = {};
	if (tData) {
		Object.entries(tData).forEach(([key, value]) => {
			translatedData[key] = t(value);
		});
	}
	return (
		<span data-translation-id={id}>
			{t(id, { ...data, ...translatedData })}
		</span>
	);
};

export { intl, T, t };
