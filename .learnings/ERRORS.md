## [ERR-20260707-001] npm_run_build_windows_path

**Logged**: 2026-07-07T17:40:00+08:00
**Priority**: medium
**Status**: pending
**Area**: frontend

### Summary
`npm run build` can fail under the `C:\Users\Admin\projects\my-caddy-ui\ui` working tree because Vite/Rolldown emits `index.html` with an absolute Windows path.

### Error
```text
[plugin vite:build-html]
Error: The "fileName" or "name" properties of emitted chunks and assets must be strings that are neither absolute nor relative paths, received "D:/projects/my-caddy-ui/ui/index.html".
```

### Context
- Command attempted: `npm run build`
- Working directory: `C:\Users\Admin\projects\my-caddy-ui\ui`
- This appears to be a Vite/Rolldown Windows path handling issue rather than a TypeScript or feature-code failure.

### Suggested Fix
Use the known-good project path/worktree for build verification, or investigate the Vite/Rolldown config/path normalization so HTML asset names remain project-relative on Windows.

### Metadata
- Reproducible: yes
- Related Files: ui/package.json

---

## [ERR-20260707-002] remote_amd64_image_on_arm64_caddy_validate_panic

**Logged**: 2026-07-07T17:59:00+08:00
**Priority**: high
**Status**: resolved
**Area**: infra

### Summary
Running the `linux/amd64` `my-caddy-ui:dev` image on the arm64 cloud host can make `caddy validate` panic, which breaks UI/API save because the backend validates before reload.

### Error
```text
panic: runtime error: invalid memory address or nil pointer dereference
crypto/x509/pkix.(*Name).FillFromRDNSequence
golang.org/x/crypto/x509roots/fallback.newFallbackCertPool()
```

### Context
- Remote Docker host: `172.16.41.29:2375`
- Host architecture: arm64
- Failing image: `my-caddy-ui:dev` built for `linux/amd64`
- Symptom: UI POST to `/api/caddy/proxy-hosts` returned `reload failed: caddy validate failed: unexpected fault address ...`
- Reproduced by running `caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile` inside the container.

### Suggested Fix
Build and deploy a native `linux/arm64` image for arm64 hosts. Verified with `my-caddy-ui:dev-arm64`; `caddy validate` and UI save/reload both pass.

### Metadata
- Reproducible: yes
- Related Files: Dockerfile

### Resolution
- **Resolved**: 2026-07-07T17:59:00+08:00
- **Notes**: Rebuilt `my-caddy-ui:dev-arm64`, loaded it onto `172.16.41.29:2375`, replaced the remote `my-caddy-ui` container while preserving `/config` and `/data` volumes.

---
## [ERR-20260710-001] playwright_script_module_resolution

**Logged**: 2026-07-10T17:47:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
Temporary E2E scripts outside the `ui` package cannot resolve package-local ESM dependencies like `playwright`.

### Error
```text
Error [ERR_MODULE_NOT_FOUND]: Cannot find package 'playwright' imported from D:\projects\my-caddy-ui\_tmp\e2e-error-response-template.mjs
```

### Context
- Command attempted: `node ..\_tmp\e2e-error-response-template.mjs` from `ui`
- The script lived in repository `_tmp`, so Node ESM package resolution started outside `ui/node_modules`.

### Suggested Fix
Place temporary Playwright scripts under `ui/_tmp` or run them through a package entry that resolves dependencies from `ui`.

### Metadata
- Reproducible: yes
- Related Files: ui/package.json

### Resolution
- **Resolved**: 2026-07-10T17:47:00+08:00
- **Notes**: Move the temporary E2E script under `ui/_tmp` before running.

---

## [ERR-20260710-002] capability_gap_analysis_without_code_audit

**Logged**: 2026-07-10T18:05:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: docs

### Summary
Produced a Caddy UI feature-priority list without first auditing the current code coverage, causing already-implemented capabilities to be listed as recommendations.

### Error
```text
User correction: "你输出的大部分都已经有了"
```

### Context
- Task: compare current UI against upstream Caddy capabilities and prioritize missing UI features.
- The initial response overgeneralized from Caddy features instead of checking existing components such as `ReverseProxyAdvancedPanel`, `UpstreamsFields`, `ResponseOptimizationFields`, `AccessLogFields`, and error response templates.

### Suggested Fix
For gap analysis, first produce or mentally build an "already covered" map from the repository, then list only missing or partially covered features.

### Metadata
- Reproducible: yes
- Related Files: ui/src/components/Form/ReverseProxyAdvancedPanel.tsx, ui/src/components/Form/UpstreamsFields.tsx, ui/src/components/Form/ResponseOptimizationFields.tsx

### Resolution
- **Resolved**: 2026-07-10T18:05:00+08:00
- **Notes**: Re-audited the repository before issuing the corrected priority list.

---

## [ERR-20260710-003] frontend_scripts_live_under_ui

**Logged**: 2026-07-10T19:05:00+08:00
**Priority**: low
**Status**: resolved
**Area**: frontend

### Summary
The repository root does not contain `package.json`; frontend scripts must be run from `ui`.

### Error
```text
Cannot find path 'D:\projects\my-caddy-ui\package.json' because it does not exist.
```

### Context
- Command attempted: `Get-Content -Raw package.json`
- Working directory: `D:\projects\my-caddy-ui`
- The frontend package is `D:\projects\my-caddy-ui\ui\package.json`.

### Suggested Fix
Run frontend commands and dependency-resolution-sensitive E2E scripts from `D:\projects\my-caddy-ui\ui`.

### Metadata
- Reproducible: yes
- Related Files: ui/package.json

### Resolution
- **Resolved**: 2026-07-10T19:05:00+08:00
- **Notes**: Continued lint/build/E2E work from the `ui` package directory.

---

## [ERR-20260710-004] vite_e2e_child_process_holds_logs

**Logged**: 2026-07-10T21:56:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
Stopping the `npm run dev` wrapper process may leave the Vite `node` child process running, keeping redirected log files locked.

### Error
```text
The process cannot access the file 'D:\projects\my-caddy-ui\ui\_tmp\vite-e2e.out.log' because it is being used by another process.
```

### Context
- Command attempted: remove temporary Vite E2E stdout/stderr files after Playwright passed.
- `Start-Process npm.cmd ... -PassThru` returned the npm wrapper process, but the listening process on port 5173 was a child process.

### Suggested Fix
After E2E, stop the process owning the Vite listen port before removing redirected log files.

### Metadata
- Reproducible: yes
- Related Files: ui/package.json

### Resolution
- **Resolved**: 2026-07-10T21:56:00+08:00
- **Notes**: Used `Get-NetTCPConnection -LocalPort 5173` and stopped the owning process, then removed the temporary logs.

---

## [ERR-20260710-005] health_endpoint_path

**Logged**: 2026-07-10T23:51:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
The UI health API maps to `/api/`, not `/api/health`.

### Error
```text
remote service not ready
```

### Context
- Command attempted: remote deployment smoke test polling `http://172.16.41.29:8888/api/health`.
- `ui/src/api/backend/getHealth.ts` calls `api.get({ url: "/" })`, which becomes `/api/`.

### Suggested Fix
Use `/api/` when polling service readiness for this app.

### Metadata
- Reproducible: yes
- Related Files: ui/src/api/backend/getHealth.ts

### Resolution
- **Resolved**: 2026-07-10T23:51:00+08:00
- **Notes**: Re-ran smoke checks against `/api/`, which returned `{"status":"OK"}`.

---

## [ERR-20260710-006] playwright_strict_duplicate_text

**Logged**: 2026-07-10T23:54:00+08:00
**Priority**: low
**Status**: resolved
**Area**: tests

### Summary
Playwright `getByText` can fail in strict mode when a value appears in both a site name and a domain cell.

### Error
```text
locator.waitFor: Error: strict mode violation: getByText('a.b.c') resolved to 2 elements
```

### Context
- Command attempted: remote observability E2E smoke test.
- The page legitimately displayed `a.b.c` in both the site name and domain columns.

### Suggested Fix
Use role or scoped locators for table assertions when duplicate display text is expected.

### Metadata
- Reproducible: yes
- Related Files: ui/src/pages/Nginx/Observability/index.tsx

### Resolution
- **Resolved**: 2026-07-10T23:54:00+08:00
- **Notes**: Re-ran with `getByRole('cell', { name: 'a.b.c', exact: true })`.

---

## [ERR-20260710-007] observability_log_exposes_low_level_config_wording

**Logged**: 2026-07-10T23:57:00+08:00
**Priority**: medium
**Status**: resolved
**Area**: backend

### Summary
The observability page can surface low-level configuration wording through raw runtime warnings in the recent-errors table.

### Error
```text
Error: low-level config wording leaked
```

### Context
- Command attempted: remote Playwright smoke test asserting the observability page does not display `Caddyfile` or config-preview wording.
- The leaked text came from a Caddy startup warning: `Caddyfile input is not formatted; run 'caddy fmt --overwrite'...`.

### Suggested Fix
Filter low-level formatting warnings from the observability recent-errors aggregation; these are not actionable runtime health signals for UI users.

### Metadata
- Reproducible: yes
- Related Files: backend/main.go, ui/src/pages/Nginx/Observability/index.tsx

### Resolution
- **Resolved**: 2026-07-10T23:57:00+08:00
- **Notes**: Added `isLowLevelRuntimeLogMessage` filtering and a unit test for the Caddyfile formatting warning.

---

## [ERR-20260710-008] gofmt_path_from_backend_directory

**Logged**: 2026-07-10T23:59:00+08:00
**Priority**: low
**Status**: resolved
**Area**: backend

### Summary
Running `gofmt -w backend\main.go` from inside the `backend` directory targets a non-existent nested path.

### Error
```text
GetFileAttributesEx backend\main.go: The system cannot find the path specified.
```

### Context
- Command attempted from `D:\projects\my-caddy-ui\backend`: `gofmt -w backend\main.go backend\main_test.go; go test ./...`
- `go test` still passed because it was a separate command in the same PowerShell line.

### Suggested Fix
From the backend directory, run `gofmt -w main.go main_test.go`; from the repository root, run `gofmt -w backend\main.go backend\main_test.go`.

### Metadata
- Reproducible: yes
- Related Files: backend/main.go, backend/main_test.go

### Resolution
- **Resolved**: 2026-07-10T23:59:00+08:00
- **Notes**: Re-ran gofmt with paths relative to the current working directory.

---
