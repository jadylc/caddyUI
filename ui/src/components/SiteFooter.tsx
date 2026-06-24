import styles from "./SiteFooter.module.css";

export function SiteFooter() {
	return (
		<footer className={styles.footer}>
			<div className="container-xl d-flex flex-wrap justify-content-between align-items-center py-3">
				<span className="text-secondary small">
					&copy; {new Date().getFullYear()} Caddy UI
				</span>
				<a
					href="https://github.com/jadylc/caddyUI"
					target="_blank"
					rel="noopener noreferrer"
					className="text-secondary small text-decoration-none"
				>
					<i className="ti ti-brand-github me-1" />
					GitHub
				</a>
			</div>
		</footer>
	);
}
