#!/usr/bin/env bash
set -euo pipefail

# ╔══════════════════════════════════════════════════════════════════╗
# ║  SchemaHero Dev Image Builder                                    ║
# ║  Push dev images to GitHub Container Registry (ghcr.io)         ║
# ╚══════════════════════════════════════════════════════════════════╝
#
# WHAT THIS DOES
#
#   Cross-compiles SchemaHero components on your Mac using native Go
#   cross-compilation, packages them into minimal container images,
#   and pushes multi-arch manifests to your ghcr.io repo.
#
#   Uses `docker buildx` to create proper multi-platform images from
#   pre-compiled binaries — no QEMU, no in-Docker Go builds, no fat VMs.
#
#   Manager image  → multi-arch Docker manifest (amd64 + arm64)
#   Plugin binaries → pushed as OCI artifacts per-arch via `oras`
#   CLI binary     → local dev binary with ghcr.io refs baked in
#
# USAGE
#
#   scripts/push-dev-images.sh                  # manager + mysql plugin
#   scripts/push-dev-images.sh manager          # manager image only
#   scripts/push-dev-images.sh mysql            # mysql plugin only
#   scripts/push-dev-images.sh manager mysql    # both (same as default)
#   scripts/push-dev-images.sh all              # manager + every plugin
#   scripts/push-dev-images.sh cli              # just build the CLI binary
#
#   Combine freely: scripts/push-dev-images.sh manager mysql cli
#
# CONFIGURATION (override via environment)
#
#   GHCR_OWNER    ghcr.io username/namespace        [default: esafak]
#   GHCR_REPO     repository name                   [default: schemahero]
#   TAG           image tag                          [default: dev]
#   PLATFORMS     target architectures               [default: linux/amd64,linux/arm64]
#
# PREREQUISITES
#
#   - docker buildx — for multi-arch image builds
#   - gh            — GitHub CLI, logged in with `write:packages` scope
#                     If missing:  gh auth refresh -h github.com -s write:packages
#   - oras          — for plugin push (optional, prints binary path if missing)
#                     Install with:  brew install oras
#
# EXAMPLES
#
#   # Push defaults (manager + mysql) with both architectures:
#   scripts/push-dev-images.sh
#
#   # Single arch only (faster):
#   PLATFORMS=linux/amd64 scripts/push-dev-images.sh
#
#   # Custom tag:
#   TAG=enum-support scripts/push-dev-images.sh manager mysql
#
#   # Push to a different org:
#   GHCR_OWNER=myorg scripts/push-dev-images.sh all
#
#   # Build CLI, then install to cluster:
#   scripts/push-dev-images.sh cli
#   ./bin/kubectl-schemahero-dev install
#
# OUTPUT IMAGES
#
#   Manager:  ghcr.io/<GHCR_OWNER>/schemahero-manager:<TAG>        (multi-arch manifest)
#   Plugins:  ghcr.io/<GHCR_OWNER>/schemahero/plugin-<driver>:<TAG>-<arch>  (per-arch OCI artifact)
#   CLI:      ./bin/kubectl-schemahero-dev  (local binary only)

# ── Configuration ─────────────────────────────────────────────
GHCR_OWNER="${GHCR_OWNER:-esafak}"
GHCR_REPO="${GHCR_REPO:-schemahero}"
TAG="${TAG:-dev}"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
GIT_SHA="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_TIME="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

MANAGER_IMAGE="ghcr.io/${GHCR_OWNER}/${GHCR_REPO}-manager"
PLUGIN_REGISTRY="ghcr.io/${GHCR_OWNER}/${GHCR_REPO}"

# Multi-arch by default. Override to build a single arch for speed.
# Go cross-compiles natively — no QEMU or emulation needed.
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"
# ──────────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }

# Parse PLATFORMS into arrays for iteration
IFS=',' read -ra PLATFORM_LIST <<< "$PLATFORMS"

# ── Builder setup ─────────────────────────────────────────────
ensure_buildx_builder() {
    # Ensure a buildx builder instance exists. Uses the Docker-native
    # "docker" driver which can push multi-arch manifests by shipping
    # pre-built binaries via --provenance=false and custom Dockerfiles.
    if docker buildx inspect schemahero-dev-builder &>/dev/null; then
        return
    fi

    info "Creating buildx builder 'schemahero-dev-builder'..."
    docker buildx create --name schemahero-dev-builder --driver docker-container --use
}

# ── Login ─────────────────────────────────────────────────────
ensure_ghcr_auth() {
    # Authenticate Docker to ghcr.io using the gh CLI token.
    # Requires `write:packages` scope for push.
    if grep -q 'ghcr.io' ~/.docker/config.json 2>/dev/null; then
        info "Already authenticated to ghcr.io"
        return
    fi

    local token
    token="$(gh auth token 2>/dev/null)" \
        || error "gh auth token failed. Run 'gh auth login' first."

    echo "$token" | docker login ghcr.io --username "${GHCR_OWNER}" --password-stdin 2>/dev/null \
        || error "docker login to ghcr.io failed. Ensure 'write:packages' scope is granted:
         gh auth refresh -h github.com -s write:packages"

    info "Authenticated to ghcr.io as ${GHCR_OWNER}"
}

# ── Manager ───────────────────────────────────────────────────
push_manager() {
    # Cross-compiles the Kubernetes manager binary for each target arch
    # on the host (native Go cross-compilation — fast), then uses
    # `docker buildx` to assemble a multi-arch image from pre-built
    # binaries and push a single manifest.
    #
    # Produces two tags: :<TAG> and :<git-sha>

    info "Building multi-arch manager image"
    info "  image:     ${MANAGER_IMAGE}:${TAG}"
    info "  platforms: ${PLATFORMS}"
    info "  version:   ${VERSION} (${GIT_SHA})"

    local tmpdir
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' RETURN

    # Cross-compile manager binary for each target architecture.
    # Go does this natively — no QEMU or emulation needed.
    for plat in "${PLATFORM_LIST[@]}"; do
        local goarch="${plat##*/}"  # e.g. "amd64" from "linux/amd64"
        info "  Compiling linux/${goarch}..."

        CGO_ENABLED=0 GOOS=linux GOARCH="${goarch}" go build \
            -tags netgo -installsuffix netgo \
            -ldflags "\
                -X github.com/schemahero/schemahero/pkg/version.version=${VERSION} \
                -X github.com/schemahero/schemahero/pkg/version.gitSHA=${GIT_SHA} \
                -X github.com/schemahero/schemahero/pkg/version.buildTime=${BUILD_TIME} \
            " \
            -o "${tmpdir}/manager-${goarch}" \
            ./cmd/manager
    done

    # Minimal Dockerfile that copies the right binary for each platform.
    # Uses TARGETARCH (set by buildx per-platform) to pick the correct binary.
    cat > "${tmpdir}/Dockerfile" <<'DOCKERFILE'
FROM --platform=$BUILDPLATFORM ubuntu:latest
ARG TARGETARCH
RUN apt-get update -y && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY manager-${TARGETARCH} /manager
RUN useradd -c 'schemahero-manager user' -m -d /home/schemahero-manager -s /bin/bash -u 1001 schemahero-manager
USER schemahero-manager
ENV HOME=/home/schemahero-manager
ENTRYPOINT ["/manager", "run"]
DOCKERFILE

    ensure_buildx_builder

    # Build multi-arch image and push directly to registry.
    # --provenance=false avoids attestation metadata that can cause issues.
    docker buildx build \
        --platform "${PLATFORMS}" \
        --provenance=false \
        -t "${MANAGER_IMAGE}:${TAG}" \
        -t "${MANAGER_IMAGE}:${GIT_SHA}" \
        -f "${tmpdir}/Dockerfile" \
        --push \
        "${tmpdir}"

    info "  Pushed ${MANAGER_IMAGE}:${TAG} (${PLATFORMS})"
    info "  Pushed ${MANAGER_IMAGE}:${GIT_SHA}"
}

# ── Plugins ───────────────────────────────────────────────────
push_plugin() {
    # Cross-compiles a database plugin binary for each target arch and
    # pushes each as an OCI artifact to ghcr.io using `oras`.
    #
    # Plugins are NOT Docker images — they're standalone binaries that
    # the manager downloads at runtime via the plugin registry.
    #
    # Per-arch tag:  plugin-{driver}:{TAG}-{arch}   (downloaded by the manager)
    # Multi-arch:    plugin-{driver}:{TAG}           (OCI Image Index manifest)
    #
    # If oras is not installed, prints the binary paths and skips push.

    local driver="$1"
    local plugin_dir="./plugins/${driver}"

    if [[ ! -d "$plugin_dir" ]]; then
        error "Plugin directory not found: ${plugin_dir}"
    fi

    local plugin_image="${PLUGIN_REGISTRY}/plugin-${driver}"

    info "Building plugin: ${driver} (${PLATFORMS})"

    local tmpdir
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' RETURN

    local arch_tags=()

    # Cross-compile for each target architecture
    for plat in "${PLATFORM_LIST[@]}"; do
        local goarch="${plat##*/}"
        info "  Compiling linux/${goarch}..."

        pushd "$plugin_dir" >/dev/null
        CGO_ENABLED=0 GOOS=linux GOARCH="${goarch}" go build \
            -o "${tmpdir}/schemahero-${driver}-${goarch}" .
        popd >/dev/null

        info "  Built linux/${goarch} ($(du -h "${tmpdir}/schemahero-${driver}-${goarch}" | cut -f1))"

        if ! command -v oras &>/dev/null; then
            warn "  oras not found — skipping push for ${goarch}"
            warn "  Binary at: ${tmpdir}/schemahero-${driver}-${goarch}"
            continue
        fi

        pushd "$tmpdir" >/dev/null
        oras push "${plugin_image}:${TAG}-${goarch}" "schemahero-${driver}-${goarch}"
        popd >/dev/null

        arch_tags+=("${TAG}-${goarch}")
        info "  Pushed ${plugin_image}:${TAG}-${goarch}"
    done

    if ! command -v oras &>/dev/null; then
        warn "  Install oras to push automatically:  brew install oras"
        return
    fi

    # Create a multi-arch OCI Image Index (manifest list) for the base tag.
    # This allows `oras pull` / `oras copy` with the base tag to resolve
    # the correct architecture automatically.
    if [[ ${#arch_tags[@]} -gt 0 ]]; then
        info "  Creating multi-arch manifest index for ${TAG}..."
        oras manifest index create "${plugin_image}:${TAG}" \
            $(for t in "${arch_tags[@]}"; do echo "${plugin_image}:${t}"; done) || {
            # Fallback: if oras manifest index is not supported, tag first arch
            warn "  oras manifest index not available — falling back to single-arch tag"
            local first_arch="${PLATFORM_LIST[0]##*/}"
            oras tag "${plugin_image}:${TAG}-${first_arch}" "${TAG}"
        }
        info "  Created multi-arch index: ${plugin_image}:${TAG} (${#arch_tags[@]} architectures)"
    fi
}

# ── CLI ───────────────────────────────────────────────────────
build_cli() {
    # Builds a local kubectl-schemahero binary with the dev manager
    # image and plugin registry URLs baked in via ldflags.
    #
    # Use this binary to install SchemaHero into a cluster that pulls
    # your dev images from ghcr.io:
    #
    #   ./bin/kubectl-schemahero-dev install

    info "Building kubectl-schemahero-dev with embedded ghcr.io refs"

    local ldflags="-X github.com/schemahero/schemahero/pkg/version.version=${VERSION} \
        -X github.com/schemahero/schemahero/pkg/version.gitSHA=${GIT_SHA} \
        -X github.com/schemahero/schemahero/pkg/version.buildTime=${BUILD_TIME} \
        -X github.com/schemahero/schemahero/pkg/version.managerImage=${MANAGER_IMAGE} \
        -X github.com/schemahero/schemahero/pkg/version.pluginRegistry=${PLUGIN_REGISTRY}"

    CGO_ENABLED=0 go build \
        -tags netgo -installsuffix netgo \
        -ldflags "$ldflags" \
        -o ./bin/kubectl-schemahero-dev \
        ./cmd/kubectl-schemahero

    info "  Binary:  ./bin/kubectl-schemahero-dev"
    info "  Manager: ${MANAGER_IMAGE}:${TAG}"
    info "  Plugins: ${PLUGIN_REGISTRY}/plugin-{driver}:${TAG}"
    echo ""
    info "  Install to cluster:"
    info "    ./bin/kubectl-schemahero-dev install"
}

# ── Main ──────────────────────────────────────────────────────
if [[ $# -eq 0 ]]; then
    TARGETS=(manager mysql)
else
    TARGETS=("$@")
fi

ensure_ghcr_auth

for target in "${TARGETS[@]}"; do
    case "$target" in
        manager)  push_manager ;;
        mysql|postgres|timescaledb|sqlite|rqlite|cassandra) push_plugin "$target" ;;
        cli)      build_cli ;;
        all)      push_manager; for p in mysql postgres timescaledb sqlite rqlite cassandra; do push_plugin "$p"; done ;;
        *)        error "Unknown target: '${target}'.

  Available targets:
    manager      Kubernetes controller image
    mysql        MySQL database plugin
    postgres     PostgreSQL database plugin
    timescaledb  TimescaleDB database plugin
    sqlite       SQLite database plugin
    rqlite       rqlite database plugin
    cassandra    Cassandra database plugin
    cli          Local kubectl-schemahero-dev binary
    all          Manager + all plugins

  Examples:
    scripts/push-dev-images.sh              # manager + mysql
    scripts/push-dev-images.sh all          # everything
    scripts/push-dev-images.sh cli          # just the CLI binary
    scripts/push-dev-images.sh manager cli  # manager image + CLI binary" ;;
    esac
done

echo ""
info "Done! Pushed images:"
info "  ${MANAGER_IMAGE}:${TAG}  (${PLATFORMS})"
info "  ${PLUGIN_REGISTRY}/plugin-{driver}:${TAG}  (multi-arch manifest)"
info "  ${PLUGIN_REGISTRY}/plugin-{driver}:${TAG}-<arch>  (per-arch OCI artifact)"
echo ""
info "Next step — build the CLI and install:"
info "  scripts/push-dev-images.sh cli && ./bin/kubectl-schemahero-dev install"
