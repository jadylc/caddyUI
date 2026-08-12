import { describe, expect, it } from "vitest";
import { validateRootRedirectPath } from "./RootRedirectValidation";

describe("validateRootRedirectPath", () => {
	it("accepts an empty value or a local non-root path", () => {
		expect(validateRootRedirectPath("")).toBeUndefined();
		expect(validateRootRedirectPath("/d")).toBeUndefined();
		expect(validateRootRedirectPath("/d?from=home")).toBeUndefined();
	});

	it.each([
		["/", "host.root-redirect.error.loop"],
		["/d/..", "host.root-redirect.error.loop"],
		["//example.com/d", "host.root-redirect.error.path-prefix"],
		["https://example.com/d", "host.root-redirect.error.path-prefix"],
		["/bad path", "host.root-redirect.error.invalid"],
		["/{uri}", "host.root-redirect.error.invalid"],
	])("rejects unsafe or looping target %s", (target, errorId) => {
		expect(validateRootRedirectPath(target)).toBe(errorId);
	});
});
