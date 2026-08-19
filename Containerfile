FROM ubuntu:22.04

# 系统依赖：GTK/WebKit 开发库 + 构建工具
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    ca-certificates curl xz-utils build-essential pkg-config git \
    libgtk-4-dev libwebkitgtk-6.0-dev \
    libgtk-3-dev libwebkit2gtk-4.1-dev \
    libwebkit2gtk-4.0-dev \
    libsecret-1-dev libglib2.0-dev \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# 代理构建参数（默认空）：同时导出大小写形式。
# apt-get 只认小写 http_proxy，curl 忽略大写 HTTP_PROXY，Go 两者都认。
ARG HTTP_PROXY=
ARG HTTPS_PROXY=
ARG http_proxy=
ARG https_proxy=
ENV HTTP_PROXY=${HTTP_PROXY} \
    HTTPS_PROXY=${HTTPS_PROXY} \
    http_proxy=${http_proxy} \
    https_proxy=${https_proxy}

# 安装 Go 1.25.0
RUN curl -fsSL "https://go.dev/dl/go1.25.0.linux-amd64.tar.gz" -o /tmp/go.tar.gz && \
    tar -C /usr/local -xzf /tmp/go.tar.gz && \
    rm /tmp/go.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /workspace
