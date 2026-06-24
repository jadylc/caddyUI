import cn from "classnames";
import type { ReactNode } from "react";

interface Props {
	children: ReactNode;
	className?: string;
	type?: "button" | "submit";
	actionType?: "primary" | "secondary" | "success" | "warning" | "danger" | "info" | "light" | "dark";
	variant?: "ghost" | "outline" | "pill" | "square" | "action";
	size?: "sm" | "md" | "lg" | "xl";
	fullWidth?: boolean;
	isLoading?: boolean;
	disabled?: boolean;
	title?: string;
	ariaLabel?: string;
	color?:
		| "blue"
		| "azure"
		| "indigo"
		| "purple"
		| "pink"
		| "red"
		| "orange"
		| "yellow"
		| "lime"
		| "green"
		| "teal"
		| "cyan";
	onClick?: () => void;
}
function Button({
	children,
	className,
	onClick,
	type,
	actionType,
	variant,
	size,
	color,
	fullWidth,
	isLoading,
	disabled,
	title,
	ariaLabel,
}: Props) {
	const myOnClick = () => {
		!isLoading && onClick && onClick();
	};

	const cns = cn(
		"btn",
		"d-inline-flex",
		"align-items-center",
		"justify-content-center",
		className,
		actionType && `btn-${actionType}`,
		variant && `btn-${variant}`,
		size && `btn-${size}`,
		color && `btn-${color}`,
		fullWidth && "w-100",
		isLoading && "btn-loading",
	);

	return (
		<button
			type={type || "button"}
			className={cns}
			onClick={myOnClick}
			disabled={disabled}
			title={title}
			aria-label={ariaLabel}
		>
			{children}
		</button>
	);
}

export { Button };
