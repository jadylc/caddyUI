## [FEAT-20260812-001] root_path_redirect

**Logged**: 2026-08-12T00:00:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: frontend

### Requested Capability
Allow a managed domain such as `a.b.c` to redirect only its root path `/` to a local path such as `/d` without creating a same-host redirect loop.

### User Context
The existing redirection-host form models a whole-host redirect and cannot safely express a same-domain landing-path redirect.

### Complexity Estimate
medium

### Suggested Implementation
Add an optional root redirect path and status code to proxy/static hosts, validate it as a non-root local path, and render an exact-path Caddy redirect before the normal site handler.

### Metadata
- Frequency: first_time
- Related Features: proxy_hosts, static_hosts, redirection_hosts

### Resolution
- **Resolved**: 2026-08-12T09:43:00+08:00
- **Notes**: Added root-only local-path redirects to proxy and static hosts, with safe validation, status selection, persistence, and render tests.

---
