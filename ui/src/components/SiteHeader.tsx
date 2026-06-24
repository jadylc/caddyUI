import { LogPanel, NavLink, ThemeSwitcher } from "src/components";
import styles from "./SiteHeader.module.css";

export function SiteHeader() {
	return (
		<header className="navbar navbar-expand-md d-print-none">
			<div className="container-xl">
				<button
					className="navbar-toggler"
					type="button"
					data-bs-toggle="collapse"
					data-bs-target="#navbar-menu"
					aria-controls="navbar-menu"
					aria-expanded="false"
					aria-label="Toggle navigation"
				>
					<span className="navbar-toggler-icon" />
				</button>
				<div className="navbar-brand pe-0 pe-md-3">
					<NavLink to="/">
						<div className={styles.logo}>
							<img
								src="/images/caddy.svg"
								width={40}
								height={40}
								className="navbar-brand-image"
								alt="Logo"
							/>
						</div>
						Caddy UI
					</NavLink>
				</div>
				<div className="navbar-nav flex-row order-md-last">
					<div className="d-none d-md-flex">
						<div className="nav-item d-flex flex-row align-items-center">
							<LogPanel />
							<ThemeSwitcher />
						</div>
					</div>
					<div className="nav-item d-flex d-md-none flex-row align-items-center">
						<LogPanel />
						<ThemeSwitcher />
					</div>
				</div>
			</div>
		</header>
	);
}
