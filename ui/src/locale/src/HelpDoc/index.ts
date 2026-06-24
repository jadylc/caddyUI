import * as zh from "./zh/index";

const items: any = { zh };

export const getHelpFile = (section: string): string => {
	if (typeof items.zh !== "undefined" && typeof items.zh[section] !== "undefined") {
		return items.zh[section].default;
	}
	throw new Error(`Cannot load help doc for zh-${section}`);
};

export default items;
