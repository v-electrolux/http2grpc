# HTTP2Grpc plugin for traefik

[![Build Status](https://github.com/v-electrolux/http2grpc/workflows/Main/badge.svg?branch=master)](https://github.com/v-electrolux/http2grpc/actions)

This plugin is an attempt to implement converter of HTTP protocol to gRPC as a middleware plugin for Traefik.
It is useful when you declare router to gRPC backend for gRPC client,
and additionally you want to intercept connection with middleware,
that do HTTP request to third party services and forward it back to gRPC client.
Forward Auth for example.

## Configuration

### Flags meaning
- `bodyAsStatusMessage`: if true, middleware try set body (as utf8 string) to grpc status message,
  if false, grpc status message will be empty. Default is false
- `logLevel`: `info` or `debug`. Default is `info`

### Static config examples

- cli as local plugin
```
--experimental.localplugins.http2grpc=true
--experimental.localplugins.http2grpc.modulename=github.com/v-electrolux/http2grpc
```

- envs as local plugin
```
TRAEFIK_EXPERIMENTAL_LOCALPLUGINS_http2grpc=true
TRAEFIK_EXPERIMENTAL_LOCALPLUGINS_http2grpc_MODULENAME=github.com/v-electrolux/http2grpc
```

- yaml as local plugin
```yaml
experimental:
  localplugins:
    http2grpc:
      modulename: github.com/v-electrolux/http2grpc
```

- toml as local plugin
```toml
[experimental.localplugins.http2grpc]
    modulename = "github.com/v-electrolux/http2grpc"
```

### Dynamic config examples

- docker labels
```
traefik.http.middlewares.checkAuth.basicauth.users=test:$$apr1$$H6uskkkW$$IgXLP6ewTrSuBkTrqE8wj/
traefik.http.middlewares.http2grpcMiddleware.plugin.http2grpc.bodyAsStatusMessage=true
traefik.http.middlewares.http2grpcMiddleware.plugin.http2grpc.logLevel=info
traefik.http.routers.http2grpcRouter.middlewares=http2grpcMiddleware,checkAuth
```

- yaml
```yml
http:

  routers:
    http2grpcRouter:
      rule: host(`demo.localhost`)
      service: grpcBackend
      entryPoints:
        - web
      middlewares:
        - http2grpcMiddleware
        - someForwardAuthMiddleware

  services:
    grpcBackend:
      loadBalancer:
        servers:
          - url: 127.0.0.1:5000

  middlewares:
    http2grpcMiddleware:
      plugin:
        http2grpc:
          bodyAsStatusMessage: true
          logLevel: info
    checkAuth:
      basicauth:
        users:
          - "test:$apr1$H6uskkkW$IgXLP6ewTrSuBkTrqE8wj/"
```
