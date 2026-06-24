import cn from "classnames";
import type { ReactNode } from "react";

interface Props {
	domains: string[];
	niceName?: string;
	provider?: string;
	color?: string;
	linkDomains?: boolean;
	linkScheme?: "http" | "https";
}

const DomainLink = ({
	domain,
	color,
	linkDomains = true,
	linkScheme = "http",
}: {
	domain?: string;
	color?: string;
	linkDomains?: boolean;
	linkScheme?: "http" | "https";
}) => {
	// when domain contains a wildcard, make the link go nowhere.
	// Apparently the domain can be null or undefined sometimes.
	// This try is just a safeguard to prevent the whole formatter from breaking.
	if (!domain) return null;
	try {
		const className = cn("badge", color ? `bg-${color}-lt` : null, "domain-name", "me-2");
		if (!linkDomains) {
			return (
				<span key={domain} className={className}>
					{domain}
				</span>
			);
		}
		let onClick: ((e: React.MouseEvent) => void) | undefined;
		if (domain.includes("*")) {
			onClick = (e: React.MouseEvent) => e.preventDefault();
		}
		return (
			<a
				key={domain}
				href={`${linkScheme}://${domain}`}
				target="_blank"
				rel="noopener"
				onClick={onClick}
				className={className}
			>
				{domain}
			</a>
		);
	} catch {
		return null;
	}
};

const isCustomNiceName = (niceName?: string, domains?: string[]) => {
	const name = niceName?.trim().toLowerCase();
	if (!name) return false;
	if (!domains || domains.length === 0) return true;
	return !domains.some((domain) => domain.trim().toLowerCase() === name);
};

export function DomainsFormatter({ domains, niceName, color, linkDomains = true, linkScheme = "http" }: Props) {
	const elms: ReactNode[] = [];

	if ((!domains || domains.length === 0) && !niceName) {
		elms.push(
			<span key="nice-name" className="badge bg-danger-lt me-2">
				Unknown
			</span>,
		);
	}
	if (isCustomNiceName(niceName, domains)) {
		elms.push(
			<span key="nice-name" className="badge bg-info-lt me-2">
				{niceName}
			</span>,
		);
	}

	if (domains) {
		domains.map((domain: string) =>
			elms.push(
				<DomainLink
					key={domain}
					domain={domain}
					color={color}
					linkDomains={linkDomains}
					linkScheme={linkScheme}
				/>,
			),
		);
	}

	return (
		<div className="flex-fill">
			<div className="font-weight-medium">{...elms}</div>
		</div>
	);
}
