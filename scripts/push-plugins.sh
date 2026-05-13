#!/bin/bash

# Script to build and push SchemaHero plugins as OCI artifacts to DockerHub
# with multi-architecture support.
#
# Each plugin is pushed as a per-architecture OCI artifact:
#   {registry}/{namespace}/plugin-{driver}:{version}-{arch}
#
# A multi-arch OCI Image Index is created at:
#   {registry}/{namespace}/plugin-{driver}:{version}
#
# The downloader constructs refs using:
#   {registry}/{namespace}/plugin-{driver}:{major_version}-{arch}
#
# Usage: ./scripts/push-plugins.sh [plugin-name] [version]
# Example: ./scripts/push-plugins.sh postgres 0.0.1
# Example: ./scripts/push-plugins.sh all 0.0.1

set -euo pipefail

REGISTRY="${REGISTRY:-docker.io}"
REGISTRY_NAMESPACE="${REGISTRY_NAMESPACE:-schemahero}"
PLUGIN_VERSION="${2:-0.0.1}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"

# Derive major version from PLUGIN_VERSION (e.g. "0.0.1" → "0")
MAJOR_VERSION="${PLUGIN_VERSION%%.*}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if required tools are installed
check_requirements() {
    local missing_tools=()
    
    if ! command -v oras &> /dev/null; then
        missing_tools+=("oras")
    fi
    
    if ! command -v go &> /dev/null; then
        missing_tools+=("go")
    fi
    
    if [ ${#missing_tools[@]} -ne 0 ]; then
        print_error "Missing required tools: ${missing_tools[*]}"
        print_info "Please install missing tools:"
        
        if [[ " ${missing_tools[*]} " =~ " oras " ]]; then
            print_info "  Install ORAS: https://oras.land/docs/installation"
            print_info "  Or run: brew install oras (on macOS)"
        fi
        
        if [[ " ${missing_tools[*]} " =~ " go " ]]; then
            print_info "  Install Go: https://golang.org/doc/install"
        fi
        
        exit 1
    fi
}

# Authenticate to the target registry.
# For ghcr.io: uses `gh auth token` to log in via oras and docker.
# For docker.io: relies on existing `docker login`.
# For other registries: attempts oras login interactively.
ensure_auth() {
    local registry_host="${REGISTRY%%/*}"  # strip any path component

    case "$registry_host" in
        ghcr.io*)
            # Check if already authenticated
            if grep -q 'ghcr.io' ~/.docker/config.json 2>/dev/null; then
                print_info "Already authenticated to ghcr.io"
                return
            fi

            if ! command -v gh &>/dev/null; then
                print_error "gh CLI not found. Install it and run: gh auth login"
                exit 1
            fi

            local token
            token="$(gh auth token 2>/dev/null)" || {
                print_error "gh auth token failed. Run 'gh auth login' first."
                exit 1
            }

            # Authenticate oras (used for plugin push)
            echo "$token" | oras login ghcr.io --username "${REGISTRY_NAMESPACE%%/*}" --password-stdin 2>/dev/null || {
                print_error "oras login to ghcr.io failed. Ensure 'write:packages' scope is granted:
         gh auth refresh -h github.com -s write:packages"
                exit 1
            }

            # Also authenticate docker in case it's needed for manifest operations
            echo "$token" | docker login ghcr.io --username "${REGISTRY_NAMESPACE%%/*}" --password-stdin 2>/dev/null || true

            print_info "Authenticated to ghcr.io"
            ;;
        docker.io|registry-1.docker.io|index.docker.io)
            if ! docker login 2>/dev/null; then
                print_error "docker login to Docker Hub failed. Run: docker login"
                exit 1
            fi
            ;;
        *)
            # Generic registry — try oras login
            print_info "Attempting oras login to ${registry_host}..."
            oras login "${registry_host}" || {
                print_error "oras login to ${registry_host} failed. Please authenticate manually."
                exit 1
            }
            ;;
    esac
}

# Function to build a plugin for multiple platforms
build_plugin() {
    local plugin_name=$1
    local plugin_binary="schemahero-${plugin_name}"
    
    print_info "Building plugin: ${plugin_name}"
    
    # Create dist directory
    mkdir -p dist
    
    # Build for each platform
    IFS=',' read -ra PLATFORM_ARRAY <<< "$PLATFORMS"
    for platform in "${PLATFORM_ARRAY[@]}"; do
        OS=$(echo $platform | cut -d'/' -f1)
        ARCH=$(echo $platform | cut -d'/' -f2)
        
        print_info "  Building for ${platform}..."
        
        # Build the plugin
        (cd plugins/${plugin_name} && \
         CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build -o ../../dist/${plugin_binary}-${OS}-${ARCH} .)
        
        # Create tarball
        tar -czf dist/${plugin_binary}-${OS}-${ARCH}.tar.gz -C dist ${plugin_binary}-${OS}-${ARCH}
        
        # Create checksum
        (cd dist && sha256sum ${plugin_binary}-${OS}-${ARCH}.tar.gz > ${plugin_binary}-${OS}-${ARCH}.tar.gz.sha256)
        
        print_info "    ✓ Built ${plugin_binary}-${OS}-${ARCH}"
    done
    
    # Create manifest
    create_manifest $plugin_name
}

# Function to create a manifest for the plugin
create_manifest() {
    local plugin_name=$1
    local plugin_binary="schemahero-${plugin_name}"
    
    cat > dist/manifest.json <<EOF
{
  "plugin": "${plugin_name}",
  "version": "${PLUGIN_VERSION}",
  "platforms": "${PLATFORMS}",
  "artifacts": [
EOF
    
    IFS=',' read -ra PLATFORM_ARRAY <<< "$PLATFORMS"
    first=true
    for platform in "${PLATFORM_ARRAY[@]}"; do
        OS=$(echo $platform | cut -d'/' -f1)
        ARCH=$(echo $platform | cut -d'/' -f2)
        
        if [ "$first" = false ]; then
            echo "," >> dist/manifest.json
        fi
        echo -n "    {\"platform\": \"$platform\", \"file\": \"${plugin_binary}-${OS}-${ARCH}.tar.gz\"}" >> dist/manifest.json
        first=false
    done
    
    cat >> dist/manifest.json <<EOF

  ]
}
EOF
}

# Function to push plugin to registry as OCI artifact
push_plugin() {
    local plugin_name=$1
    local plugin_binary="schemahero-${plugin_name}"
    # Use "plugin-{driver}" repo format to match the downloader's GetPluginArtifactRef
    local oci_repo="${REGISTRY}/${REGISTRY_NAMESPACE}/plugin-${plugin_name}"
    
    print_info "Pushing plugin ${plugin_name} to ${oci_repo}"
    
    # Push each platform artifact
    local version_arch_tags=()
    local major_arch_tags=()
    local first_arch=""
    IFS=',' read -ra PLATFORM_ARRAY <<< "$PLATFORMS"
    for platform in "${PLATFORM_ARRAY[@]}"; do
        local OS=$(echo "$platform" | cut -d'/' -f1)
        local ARCH=$(echo "$platform" | cut -d'/' -f2)
        
        # Capture the first architecture for deterministic fallbacks
        if [ -z "$first_arch" ]; then
            first_arch="$ARCH"
        fi
        
        print_info "  Pushing ${platform} artifact..."
        
        # Tag format: {version}-{arch} (e.g. "0.0.1-amd64")
        # This matches the downloader's expected format:
        #   docker.io/schemahero/plugin-{driver}:{major_version}-{arch}
        oras push "${oci_repo}:${PLUGIN_VERSION}-${ARCH}" \
          --artifact-type application/vnd.schemahero.plugin.v1+tar \
          "dist/${plugin_binary}-${OS}-${ARCH}.tar.gz:application/gzip" \
          "dist/${plugin_binary}-${OS}-${ARCH}.tar.gz.sha256:text/plain" \
          "dist/manifest.json:application/json" \
          --annotation "org.opencontainers.image.title=${plugin_binary}" \
          --annotation "org.opencontainers.image.version=${PLUGIN_VERSION}" \
          --annotation "org.opencontainers.image.description=SchemaHero ${plugin_name} database plugin" \
          --annotation "org.opencontainers.image.source=https://github.com/schemahero/schemahero" \
          --annotation "org.opencontainers.image.platform=${platform}" \
          --annotation "org.opencontainers.image.os=${OS}" \
          --annotation "org.opencontainers.image.architecture=${ARCH}"
        
        version_arch_tags+=("${PLUGIN_VERSION}-${ARCH}")
        print_info "    ✓ Pushed ${plugin_binary}:${PLUGIN_VERSION}-${ARCH}"
        
        # Also tag with major version + arch for the downloader
        if [ "${MAJOR_VERSION}" != "${PLUGIN_VERSION}" ]; then
            print_info "  Tagging ${MAJOR_VERSION}-${ARCH} (major version)..."
            oras tag "${oci_repo}:${PLUGIN_VERSION}-${ARCH}" "${MAJOR_VERSION}-${ARCH}"
            major_arch_tags+=("${MAJOR_VERSION}-${ARCH}")
        fi
    done
    
    # Create a multi-arch OCI Image Index (manifest list) for the version tag.
    # This allows `oras copy` / `oras pull` to resolve the right arch automatically.
    print_info "  Creating multi-arch manifest index for ${PLUGIN_VERSION}..."
    oras manifest index create "${oci_repo}:${PLUGIN_VERSION}" \
        $(for t in "${version_arch_tags[@]}"; do echo "${oci_repo}:${t}"; done) || {
        # Fallback: if oras manifest index is not supported, tag the first arch
        print_warn "  oras manifest index not available — falling back to single-arch tag"
        oras tag "${oci_repo}:${PLUGIN_VERSION}-${first_arch}" "${PLUGIN_VERSION}"
    }
    
    # Also create a multi-arch index for the major version tag
    if [ "${MAJOR_VERSION}" != "${PLUGIN_VERSION}" ] && [ ${#major_arch_tags[@]} -gt 0 ]; then
        print_info "  Creating multi-arch manifest index for ${MAJOR_VERSION}..."
        oras manifest index create "${oci_repo}:${MAJOR_VERSION}" \
            $(for t in "${major_arch_tags[@]}"; do echo "${oci_repo}:${t}"; done) || {
            print_warn "  Falling back to single-arch tag for ${MAJOR_VERSION}"
            oras tag "${oci_repo}:${MAJOR_VERSION}-${first_arch}" "${MAJOR_VERSION}"
        }
    fi
    
    # Tag as latest
    print_info "  Tagging as latest..."
    oras tag "${oci_repo}:${PLUGIN_VERSION}" latest
    
    print_info "✓ Successfully pushed ${plugin_name} v${PLUGIN_VERSION} (${PLATFORMS})"
}

# Main script logic
main() {
    local plugin_name=$1
    
    if [ -z "$plugin_name" ]; then
        print_error "Usage: $0 <plugin-name|all> [version]"
        print_info "Available plugins: postgres, mysql, timescaledb, sqlite, rqlite, cassandra"
        exit 1
    fi
    
    check_requirements
    ensure_auth
    
    # Get list of plugins to build
    if [ "$plugin_name" = "all" ]; then
        plugins=("postgres" "mysql" "timescaledb" "sqlite" "rqlite" "cassandra")
    else
        plugins=("$plugin_name")
    fi
    
    # Clean dist directory
    rm -rf dist
    mkdir -p dist
    
    # Build and push each plugin
    for plugin in "${plugins[@]}"; do
        if [ ! -d "plugins/${plugin}" ]; then
            print_error "Plugin directory not found: plugins/${plugin}"
            exit 1
        fi
        
        build_plugin "$plugin"
        
        # Ask before pushing
        read -p "Push ${plugin} to registry? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            push_plugin "$plugin"
        else
            print_info "Skipping push for ${plugin}"
        fi
        
        # Clean up binaries but keep tarballs for inspection
        rm -f dist/schemahero-${plugin}-*[^.tar.gz]
    done
    
    print_info "✓ All done!"
    print_info "Artifacts are in dist/ directory"
}

# Run main function
main "$@"