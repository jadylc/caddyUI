const validateRootRedirectPath = (value: unknown) => {
	const target = String(value || "").trim();
	if (!target) return undefined;
	if (!target.startsWith("/") || target.startsWith("//")) {
		return "host.root-redirect.error.path-prefix";
	}
	const pathParts = target
		.split(/[?#]/, 1)[0]
		.split("/")
		.reduce<string[]>((parts, segment) => {
			if (!segment || segment === ".") return parts;
			if (segment === "..") parts.pop();
			else parts.push(segment);
			return parts;
		}, []);
	if (pathParts.length === 0) {
		return "host.root-redirect.error.loop";
	}
	if (/\s|[{}\\]/.test(target)) {
		return "host.root-redirect.error.invalid";
	}
	return undefined;
};

export { validateRootRedirectPath };
