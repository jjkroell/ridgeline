# MeshCore firmware builder.
#
# Deliberately a SMALL image: PlatformIO's toolchains and the MeshCore checkout
# live in mounted volumes, not baked in. The nRF52 + ESP32 toolchains come to
# roughly 6 GB, which would make an unwieldy image to rebuild and push; in a
# volume they are downloaded once and survive image rebuilds.
FROM python:3.12-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
      git curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir platformio==6.1.16

# Toolchains and package cache land here; mount a volume over it.
ENV PLATFORMIO_CORE_DIR=/pio
# MeshCore checkout; mount a volume over it too.
ENV SRC_DIR=/src
# PlatformIO's content-addressed object cache. Changing a -D flag invalidates the
# build environment and forces a full recompile, but most translation units are
# byte-identical across those rebuilds, so this turns minutes back into seconds.
# Lives under /pio so it shares the toolchain volume's lifetime.
ENV PLATFORMIO_BUILD_CACHE_DIR=/pio/build-cache

COPY patches /patches
COPY build.sh /usr/local/bin/build.sh
RUN chmod +x /usr/local/bin/build.sh

# Builds run as an unprivileged user; the compiler never needs root.
RUN useradd -m -u 1000 builder && mkdir -p /pio /src /out && chown builder:builder /pio /src /out
USER builder
WORKDIR /src

ENTRYPOINT ["/usr/local/bin/build.sh"]
