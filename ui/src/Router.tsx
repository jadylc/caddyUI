import { lazy, Suspense } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import {
	ErrorNotFound,
	LoadingPage,
	Page,
	SiteContainer,
	SiteFooter,
	SiteHeader,
	SiteMenu,
	Unhealthy,
} from "src/components";
import { useHealth } from "src/hooks";

const Dashboard = lazy(() => import("src/pages/Dashboard"));
const Certificates = lazy(() => import("src/pages/Certificates"));
const Access = lazy(() => import("src/pages/Access"));
const AuditLog = lazy(() => import("src/pages/AuditLog"));
const ProxyHosts = lazy(() => import("src/pages/Nginx/ProxyHosts"));
const RedirectionHosts = lazy(() => import("src/pages/Nginx/RedirectionHosts"));
const DeadHosts = lazy(() => import("src/pages/Nginx/DeadHosts"));
const Streams = lazy(() => import("src/pages/Nginx/Streams"));
const DynamicDNS = lazy(() => import("src/pages/Nginx/DynamicDNS"));
const DomainMonitor = lazy(() => import("src/pages/Nginx/DomainMonitor"));
const WakeOnLan = lazy(() => import("src/pages/WakeOnLan"));
const Settings = lazy(() => import("src/pages/Settings"));
const CredentialSettings = lazy(() => import("src/pages/Settings/Credentials"));
const NotificationSettings = lazy(() => import("src/pages/Settings/Notifications"));
const SystemSettings = lazy(() => import("src/pages/Settings/SystemSettings"));

function Router() {
	const health = useHealth();

	if (health.isLoading) {
		return <LoadingPage />;
	}

	if (health.isError || health.data?.status !== "OK") {
		return <Unhealthy />;
	}

	return (
		<BrowserRouter>
			<Page>
				<div>
					<SiteHeader />
					<SiteMenu />
				</div>
				<SiteContainer>
					<Suspense fallback={<LoadingPage noLogo />}>
						<Routes>
							<Route path="*" element={<ErrorNotFound />} />
							<Route path="/certificates" element={<Certificates />} />
							<Route path="/access" element={<Access />} />
							<Route path="/wake-on-lan" element={<WakeOnLan />} />
							<Route path="/audit-log" element={<AuditLog />} />
							<Route path="/settings/authelia" element={<Settings />} />
							<Route path="/settings/authentik" element={<Settings />} />
							<Route path="/settings/credentials" element={<CredentialSettings />} />
							<Route path="/settings/notifications" element={<NotificationSettings />} />
							<Route path="/settings/system" element={<SystemSettings />} />
							<Route path="/caddy/proxy" element={<ProxyHosts />} />
							<Route path="/caddy/redirection" element={<RedirectionHosts />} />
							<Route path="/caddy/404" element={<DeadHosts />} />
							<Route path="/caddy/stream" element={<Streams />} />
							<Route path="/caddy/dynamic-dns" element={<DynamicDNS />} />
							<Route path="/caddy/domain-monitor" element={<DomainMonitor />} />
							<Route path="/nginx/proxy" element={<Navigate to="/caddy/proxy" replace />} />
							<Route path="/nginx/redirection" element={<Navigate to="/caddy/redirection" replace />} />
							<Route path="/nginx/404" element={<Navigate to="/caddy/404" replace />} />
							<Route path="/nginx/stream" element={<Navigate to="/caddy/stream" replace />} />
							<Route path="/nginx/dynamic-dns" element={<Navigate to="/caddy/dynamic-dns" replace />} />
							<Route
								path="/nginx/domain-monitor"
								element={<Navigate to="/caddy/domain-monitor" replace />}
							/>
							<Route path="/" element={<Dashboard />} />
						</Routes>
					</Suspense>
				</SiteContainer>
				<SiteFooter />
			</Page>
		</BrowserRouter>
	);
}

export default Router;
