import type { ReactNode } from "react";

interface Props {
	section?: string;
	permission?: string;
	hideError?: boolean;
	children?: ReactNode;
	pageLoading?: boolean;
	loadingNoLogo?: boolean;
}
function HasPermission({ children }: Props) {
	return <>{children}</>;
}

export { HasPermission };
