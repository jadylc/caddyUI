import cn from "classnames";
import { useLocation, useNavigate } from "react-router-dom";

interface Props {
	children: React.ReactNode;
	to?: string;
	isDropdownItem?: boolean;
	onClick?: () => void;
	active?: boolean;
}
export function NavLink({ children, to, isDropdownItem, onClick, active }: Props) {
	const navigate = useNavigate();
	const location = useLocation();
	const isActive = active ?? (to === "/" ? location.pathname === "/" : location.pathname === to);

	return (
		<a
			className={cn(isDropdownItem ? "dropdown-item" : "nav-link", isActive && "active")}
			href={to}
			aria-current={isActive ? "page" : undefined}
			onClick={(e) => {
				e.preventDefault();
				if (onClick) {
					onClick();
				}
				if (to) {
					navigate(to);
				}
			}}
		>
			{children}
		</a>
	);
}
