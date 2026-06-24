import {
	IconDeviceDesktop,
	IconDeviceDesktopBolt,
	IconGlobe,
	IconHome,
	IconLock,
	IconSettings,
} from "@tabler/icons-react";
import cn from "classnames";
import React, { useCallback, useEffect, useRef, useState } from "react";
import { useLocation } from "react-router-dom";
import { NavLink } from "src/components";
import { T } from "src/locale";

interface MenuItem {
	label: string;
	icon?: React.ElementType;
	to?: string;
	items?: MenuItem[];
}

function useHoverClickDropdown() {
	const [pinned, setPinned] = useState(false);
	const [open, setOpen] = useState(false);
	const ref = useRef<HTMLLIElement>(null);
	const hoverTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
	const leaveTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

	const openDropdown = useCallback(() => {
		clearTimeout(leaveTimer.current);
		setOpen(true);
	}, []);

	const closeDropdown = useCallback(() => {
		clearTimeout(hoverTimer.current);
		setOpen(false);
		setPinned(false);
	}, []);

	const onMouseEnter = useCallback(() => {
		if (pinned) return;
		clearTimeout(leaveTimer.current);
		hoverTimer.current = setTimeout(openDropdown, 100);
	}, [pinned, openDropdown]);

	const onMouseLeave = useCallback(() => {
		if (pinned) return;
		clearTimeout(hoverTimer.current);
		leaveTimer.current = setTimeout(closeDropdown, 150);
	}, [pinned, closeDropdown]);

	const onToggleClick = useCallback(
		(e: React.MouseEvent) => {
			e.preventDefault();
			e.stopPropagation();
			if (pinned) {
				closeDropdown();
			} else {
				setPinned(true);
				setOpen(true);
			}
		},
		[pinned, closeDropdown],
	);

	useEffect(() => {
		if (!pinned) return;
		const handler = (e: MouseEvent) => {
			if (ref.current && !ref.current.contains(e.target as Node)) {
				closeDropdown();
			}
		};
		document.addEventListener("click", handler, true);
		return () => document.removeEventListener("click", handler, true);
	}, [pinned, closeDropdown]);

	useEffect(() => {
		return () => {
			clearTimeout(hoverTimer.current);
			clearTimeout(leaveTimer.current);
		};
	}, []);

	return { ref, open, pinned, onMouseEnter, onMouseLeave, onToggleClick, closeDropdown };
}

const menuItems: MenuItem[] = [
	{
		to: "/",
		icon: IconHome,
		label: "dashboard",
	},
	{
		icon: IconDeviceDesktop,
		label: "site-management",
		items: [
			{
				to: "/caddy/proxy",
				label: "proxy-hosts",
			},
			{
				to: "/caddy/redirection",
				label: "redirection-hosts",
			},
			{
				to: "/caddy/stream",
				label: "streams",
			},
			{
				to: "/caddy/404",
				label: "dead-hosts",
			},
		],
	},
	{
		icon: IconGlobe,
		label: "domain-certificates",
		items: [
			{
				to: "/caddy/domain-monitor",
				label: "domain-monitor",
			},
			{
				to: "/caddy/dynamic-dns",
				label: "dynamic-dns",
			},
			{
				to: "/certificates",
				label: "certificates",
			},
			{
				to: "/settings/credentials",
				label: "credentials-settings",
			},
		],
	},
	{
		icon: IconLock,
		label: "access-control",
		items: [
			{
				to: "/access",
				label: "access-lists",
			},
			{
				to: "/settings/authelia",
				label: "authelia-settings",
			},
			{
				to: "/settings/authentik",
				label: "authentik-settings",
			},
		],
	},
	{
		to: "/wake-on-lan",
		icon: IconDeviceDesktopBolt,
		label: "wake-on-lan",
	},
	{
		icon: IconSettings,
		label: "system-management",
		items: [
			{
				to: "/settings/system",
				label: "system-settings",
			},
			{
				to: "/settings/notifications",
				label: "notifications-settings",
			},
			{
				to: "/audit-log",
				label: "auditlogs",
			},
		],
	},
];

const isMenuActive = (item: MenuItem, pathname: string): boolean => {
	if (item.to) {
		return item.to === "/" ? pathname === "/" : pathname === item.to;
	}
	return item.items?.some((subitem) => isMenuActive(subitem, pathname)) || false;
};

interface MenuItemViewProps {
	item: MenuItem;
	pathname: string;
	onClick?: () => void;
}

function MenuItemView({ item, pathname, onClick }: MenuItemViewProps) {
	if (item.items && item.items.length > 0) {
		return <MenuDropdown item={item} pathname={pathname} onClick={onClick} />;
	}

	const active = isMenuActive(item, pathname);
	return (
		<li className={cn("nav-item", active && "active")} key={`item-${item.label}`}>
			<NavLink to={item.to} onClick={onClick} active={active}>
				<span className="nav-link-icon d-md-none d-lg-inline-block">
					{item.icon && React.createElement(item.icon, { height: 24, width: 24 })}
				</span>
				<span className="nav-link-title">
					<T id={item.label} />
				</span>
			</NavLink>
		</li>
	);
}

function MenuDropdown({ item, pathname, onClick }: MenuItemViewProps) {
	const active = isMenuActive(item, pathname);
	const cns = cn("nav-item", "dropdown", active && "active");
	const { ref, open, onMouseEnter, onMouseLeave, onToggleClick, closeDropdown } = useHoverClickDropdown();
	return (
		<li
			ref={ref}
			className={cns}
			key={`item-${item.label}`}
			onMouseEnter={onMouseEnter}
			onMouseLeave={onMouseLeave}
		>
			<a
				className={cn("nav-link dropdown-toggle", active && "active", open && "show")}
				href={item.to}
				aria-expanded={open}
				role="button"
				onClick={onToggleClick}
			>
				<span className="nav-link-icon d-md-none d-lg-inline-block">
					{item.icon && React.createElement(item.icon, { height: 24, width: 24 })}
				</span>
				<span className="nav-link-title">
					<T id={item.label} />
				</span>
			</a>
			<div className={cn("dropdown-menu", "site-menu-dropdown-menu", open && "show")}>
				{item.items?.map((subitem, idx) => (
					<NavLink
						key={`${idx}-${subitem.to}`}
						to={subitem.to}
						isDropdownItem
						onClick={() => {
							closeDropdown();
							onClick?.();
						}}
						active={isMenuActive(subitem, pathname)}
					>
						<T id={subitem.label} />
					</NavLink>
				))}
			</div>
		</li>
	);
}

export function SiteMenu() {
	const location = useLocation();
	const closeMenu = () =>
		setTimeout(() => {
			const navbarToggler = document.querySelector<HTMLElement>(".navbar-toggler");
			const navbarMenu = document.querySelector("#navbar-menu");
			if (navbarToggler && navbarMenu?.classList.contains("show")) {
				navbarToggler.click();
			}
		}, 300);

	return (
		<header className="navbar-expand-md">
			<div className="collapse navbar-collapse" id="navbar-menu">
				<div className="navbar">
					<div className="container-xl">
						<div className="row flex-column flex-md-row flex-fill align-items-center">
							<div className="col">
								<ul className="navbar-nav">
									{menuItems.length > 0 &&
										menuItems.map((item) => (
											<MenuItemView
												key={`item-${item.label}`}
												item={item}
												pathname={location.pathname}
												onClick={closeMenu}
											/>
										))}
								</ul>
							</div>
						</div>
					</div>
				</div>
			</div>
		</header>
	);
}
