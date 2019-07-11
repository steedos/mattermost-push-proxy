FROM ubuntu:18.04

ENV APP_VERSION=5.9.0
ENV PATH="/mattermost-push-proxy/bin:${PATH}"

# Install some needed packages
RUN apt-get update \
    && apt-get -y --no-install-recommends install \
      curl \
      wget \
      netcat \
      ca-certificates \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*.deb /var/cache/apt/archives/partial/*.deb /var/cache/apt/*.bin

RUN mkdir -p /mattermost-push-proxy \
    && cd /mattermost-push-proxy \
    && wget https://www-steedos-com.oss-cn-beijing.aliyuncs.com/apps/messenger/push-proxy/mattermost-push-proxy-$APP_VERSION.tar.gz \
    && tar -xvzf mattermost-push-proxy-$APP_VERSION.tar.gz \
    && rm mattermost-push-proxy-$APP_VERSION.tar.gz \
    && mv mattermost-push-proxy/* ./ \
    && rm -rf mattermost-push-proxy

WORKDIR /mattermost-push-proxy

RUN chmod 755 /mattermost-push-proxy/bin/mattermost-push-proxy

COPY entrypoint.sh /

EXPOSE 8066

VOLUME ["/mattermost-push-proxy", "/config", "/certs"]

ENTRYPOINT ["/entrypoint.sh"]
