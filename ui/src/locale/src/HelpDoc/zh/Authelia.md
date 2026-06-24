## 什么是 Authelia 设置？

Authelia 设置用于配置全局 forward_auth 入口。

启用后，代理服务可以选择使用 Authelia 进行统一登录、双因素认证和访问控制。

请填写 Caddy 容器或进程可以访问到的 Authelia 地址；如果开启故障放行，Authelia 不可用时会临时绕过认证。
