FROM node:lts-alpine AS templating

ENV WORKDIR=/template

RUN mkdir -p ${WORKDIR}
WORKDIR ${WORKDIR}

COPY theme-build ${WORKDIR}/
COPY resources ${WORKDIR}/resources

RUN yarn install
RUN node template-colors.js  ${WORKDIR}/resources/var/www/html/styles/default.css.tpl ${WORKDIR}/build/default.css
RUN node template-error-pages.js ${WORKDIR}/resources/var/www/html/errors/error-page.html.tpl ${WORKDIR}/build/errors


FROM alpine:3.20 AS builder

# dockerfile is based on https://github.com/dockerfile/nginx and https://github.com/bellycard/docker-loadbalancer
ENV NGINX_VERSION 1.26.3
ENV NGINX_TAR_SHA256="69ee2b237744036e61d24b836668aad3040dda461fe6f570f1787eab570c75aa"

COPY nginx-build /

RUN set -x -o errexit \
    && set -o nounset \
    && set -o pipefail \
    && apk update \
    && apk upgrade \
    && apk --update add bash openssl-dev pcre-dev zlib-dev wget build-base \
    && mkdir /build \
    && cd /build \
    && wget http://nginx.org/download/nginx-${NGINX_VERSION}.tar.gz \
    && echo "${NGINX_TAR_SHA256} *nginx-${NGINX_VERSION}.tar.gz" | sha256sum -c - \
    && tar -zxvf nginx-${NGINX_VERSION}.tar.gz \
    && cd /build/nginx-${NGINX_VERSION} \
    && /build.sh \
    && rm -rf /var/cache/apk/* /build

FROM alpine:3.20

LABEL maintainer="hello@cloudogu.com"

ENV WARP_MENU_VERSION=2.0.3 \
    WARP_MENU_TAR_SHA256="8dfd023579728b6786bdb4664fb6d3e629717d9d2d27cdd4b365f9a844f1858c" \
    CES_ABOUT_VERSION="0.7.0" \
    CES_ABOUT_TAR_SHA256="fcfdfb86dac75d5ae751cc0e8c3436ecee12f0d5ed830897c4f61029ae1df27e" \
    # Used in template to invalidate caches - do not remove. The release script will auto update this line
    VERSION="1.0.3"

# Install required packages
RUN apk upgrade \
    && apk --update add \
        bash \
        openssl \
        pcre \
        musl \
        zlib

 # add nginx user
RUN adduser nginx -D

# prepare folders
RUN set -x \
 && mkdir -p /var/www/html \
 && mkdir -p /var/www/html/customhtml \
 && mkdir -p /var/log/nginx \
 && mkdir -p /etc/nginx/include.d \
 && mkdir -p /etc/nginx/conf.d

# install ces-about page
RUN wget -O /tmp/ces-about-v${CES_ABOUT_VERSION}.tar.gz https://github.com/cloudogu/ces-about/releases/download/v${CES_ABOUT_VERSION}/ces-about_v${CES_ABOUT_VERSION}.tar.gz \
    && echo "${CES_ABOUT_TAR_SHA256} */tmp/ces-about-v${CES_ABOUT_VERSION}.tar.gz" | sha256sum -c - \
    && tar -xzvf /tmp/ces-about-v${CES_ABOUT_VERSION}.tar.gz -C /var/www/html \
    && mkdir -p /etc/nginx/include.d/ \
    && cp /var/www/html/routes/ces-about-routes.conf /etc/nginx/include.d/ \
    && rm -rf /var/www/html/routes

# install warp menu
RUN wget https://github.com/cloudogu/warp-menu/releases/download/v${WARP_MENU_VERSION}/warp-v${WARP_MENU_VERSION}.zip -q -O /tmp/warp.zip \
 && echo "${WARP_MENU_TAR_SHA256} */tmp/warp.zip" | sha256sum -c - \
 && unzip /tmp/warp.zip -d /var/www/html

 # redirect logs
RUN ln -sf /dev/stdout /var/log/nginx/access.log \
 && ln -sf /dev/stderr /var/log/nginx/error.log

# cleanup apk cache
RUN rm -rf /var/cache/apk/*

# copy files
COPY --from=builder /usr/sbin/nginx /usr/sbin/nginx

# copy files
COPY resources /

# copy templated files
COPY --from=templating /template/build/default.css /var/www/html/styles/default.css
COPY --from=templating /template/build/errors /var/www/html/errors

# Volumes are used to avoid writing to containers writable layer https://docs.docker.com/storage/
# Compared to the bind mounted volumes we declare in the dogu.json,
# the volumes declared here are not mounted to the dogu if the container is destroyed/recreated,
# e.g. after a dogu upgrade
VOLUME ["/etc/nginx/app.conf.d", "/var/www/html"]

# Define working directory.
WORKDIR /etc/nginx

# Expose ports.
EXPOSE 80

# Define default command.
ENTRYPOINT ["/startup.sh"]
