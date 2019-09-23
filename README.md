[![codecov](https://codecov.io/gh/daemonl/cra-version-proxy.go/branch/master/graph/badge.svg)](https://codecov.io/gh/daemonl/cra-version-proxy.go)
[![Docker Cloud Build Status](https://img.shields.io/docker/cloud/build/daemonl/cra-version-proxy)](https://hub.docker.com/r/daemonl/cra-version-proxy)

Create React App Version Proxy
==============================

A reverse proxy for React apps.

- Set up a public source server, e.g. an S3 website, where different app
  versions are in /{version}/ prefixed directories, set as `CRA_PROXY_SOURCE`

- Set `CRA_PROXY_DEFAULT_VERSION` to the current 'live' version for normal users

Users can request any version with ?version={version}

The version will be stored in a cookie so that resources loaded by HTML pages
are also versioned (css, js etc)

All requests without an extension, including directories, are assumed to be the
app, so `/index.html` will be served

Docker
------

[daemonl/cra-version-proxy](https://hub.docker.com/r/daemonl/cra-version-proxy)




