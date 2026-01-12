package docker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// GetLocalImageDigest returns the digest of a local image if it exists
func (d *Docker) GetLocalImageDigest(image string) (string, error) {
	start := time.Now()
	defer func() {
		if time.Since(start) > 5*time.Second {
			d.logger.Warn("GetLocalImageDigest took longer than expected: %v", time.Since(start))
		}
	}()

	// First check if the image exists locally
	output, err := d.RunCommand("images", "--format", "{{.Repository}}:{{.Tag}}", image)
	if err != nil || strings.TrimSpace(output) == "" {
		d.logger.Debug("Image %s not found locally", image)
		return "", fmt.Errorf("image not found locally: %s", image)
	}

	// Get the image digest directly from Docker's image inspection
	// First try to get the digest from the manifest
	output, err = d.RunCommand("inspect", "--format", "{{.RepoDigests}}", image)
	if err != nil {
		return "", fmt.Errorf("failed to inspect local image: %w", err)
	}

	d.logger.Debug("Raw RepoDigests for %s: %s", image, strings.TrimSpace(output))
	
	// Extract the digest from RepoDigests
	// Format is typically [repo@sha256:digest]
	repoDigests := strings.TrimSpace(output)
	if repoDigests != "[]" && strings.Contains(repoDigests, "sha256:") {
		// Extract the sha256 part
		parts := strings.Split(repoDigests, "sha256:")
		if len(parts) > 1 {
			// Get the digest part and remove any trailing characters
			digestPart := strings.Split(parts[1], "]")
			if len(digestPart) > 0 {
				digest := "sha256:" + strings.TrimSpace(digestPart[0])
				d.logger.Debug("Local digest (from RepoDigests) for %s: %s", image, digest)
				return digest, nil
			}
		}
	}

	// If we couldn't get the digest from RepoDigests, try to get it from the remote registry
	// This is a workaround for the fact that local and remote digests can differ
	// even for the same image content
	d.logger.Debug("Could not extract digest from RepoDigests, trying to get from remote registry")
	
	// Parse the image reference
	ref, err := name.ParseReference(image)
	if err != nil {
		return "", fmt.Errorf("failed to parse image reference: %w", err)
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get the digest from the remote registry
	desc, err := remote.Get(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		d.logger.Debug("Failed to get digest from remote registry: %v", err)
		
		// As a last resort, use the image ID
		output, err = d.RunCommand("inspect", "--format", "{{.Id}}", image)
		if err != nil {
			return "", fmt.Errorf("failed to get image ID: %w", err)
		}
		
		digest := strings.TrimSpace(output)
		if digest == "" {
			return "", fmt.Errorf("empty digest returned for local image: %s", image)
		}
		
		d.logger.Debug("Local digest (from ID) for %s: %s", image, digest)
		return digest, nil
	}

	digestStr := desc.Digest.String()
	d.logger.Debug("Local digest (from remote registry) for %s: %s", image, digestStr)
	return digestStr, nil
}

// Cache structure to store image digests with expiration
type digestCacheEntry struct {
	digest    string
	expiresAt time.Time
}

// Cache to store image digests
var (
	digestCache     = make(map[string]digestCacheEntry)
	digestCacheMux  sync.RWMutex
	digestCacheTTL  = 5 * time.Minute // Cache entries expire after 5 minutes
)

// GetRemoteImageDigest fetches the digest of a remote image without pulling it
// Uses go-containerregistry to properly handle multi-architecture images
func (d *Docker) GetRemoteImageDigest(image string) (string, error) {
	// Check cache first
	digestCacheMux.RLock()
	if entry, found := digestCache[image]; found && time.Now().Before(entry.expiresAt) {
		digestCacheMux.RUnlock()
		d.logger.Debug("Using cached digest for %s: %s", image, entry.digest)
		return entry.digest, nil
	}
	digestCacheMux.RUnlock()

	d.logger.Debug("Getting remote digest for %s using go-containerregistry", image)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Parse the image reference
	ref, err := name.ParseReference(image)
	if err != nil {
		return "", fmt.Errorf("failed to parse image reference: %w", err)
	}

	// Get the image descriptor with timeout context
	desc, err := remote.Get(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		// Handle specific error types
		if strings.Contains(err.Error(), "unauthorized") {
			d.logger.Warn("Authentication error for %s: %v", image, err)
			return "", fmt.Errorf("authentication failed for image %s: %w", image, err)
		} else if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("timeout while fetching image digest: %w", err)
		}
		return "", fmt.Errorf("failed to get image descriptor: %w", err)
	}

	// Get the digest
	digest := desc.Digest.String()
	d.logger.Debug("Remote digest for %s: %s", image, digest)

	// Cache the result
	digestCacheMux.Lock()
	digestCache[image] = digestCacheEntry{
		digest:    digest,
		expiresAt: time.Now().Add(digestCacheTTL),
	}
	digestCacheMux.Unlock()

	return digest, nil
}

// ShouldPullImage checks if the remote image is different from the local one
// Returns true if the image should be pulled, false otherwise, and any error encountered
func (d *Docker) ShouldPullImage(image string) (bool, error) {
	start := time.Now()
	defer func() {
		d.logger.Debug("ShouldPullImage check for %s took %v", image, time.Since(start))
	}()

	// Parse the image to ensure it's valid
	_, err := name.ParseReference(image)
	if err != nil {
		return true, fmt.Errorf("invalid image reference %s: %w", image, err)
	}

	// Try to get local digest
	localDigest, localErr := d.GetLocalImageDigest(image)
	
	// If local image doesn't exist, we definitely need to pull
	if localErr != nil {
		d.logger.Info("Local image %s not found, will pull", image)
		return true, nil
	}
	
	// Try to get remote digest
	remoteDigest, remoteErr := d.GetRemoteImageDigest(image)
	if remoteErr != nil {
		// Check for specific error types
		if strings.Contains(remoteErr.Error(), "not found") {
			d.logger.Error("Image %s not found in remote registry", image)
			return false, fmt.Errorf("image not found in registry: %w", remoteErr)
		} else if strings.Contains(remoteErr.Error(), "unauthorized") {
			d.logger.Warn("Authentication error for %s: %v", image, remoteErr)
			// For auth errors, we might want to retry with credentials, but for now just pull
			return true, fmt.Errorf("authentication error, will pull anyway: %w", remoteErr)
		} else {
			// For other errors, log warning but proceed with pull to be safe
			d.logger.Warn("Could not get remote digest for %s: %v, will pull anyway", image, remoteErr)
			return true, nil
		}
	}
	
	// Clean up digests to ensure proper comparison
	// Extract just the hash part if it's a full digest with algorithm prefix
	cleanDigest := func(digest string) string {
		// If it contains a repo reference, extract just the digest part
		if strings.Contains(digest, "@") {
			parts := strings.Split(digest, "@")
			if len(parts) > 1 {
				digest = parts[1]
			}
		}
		
		// If it has a sha256: prefix, extract just the hash
		if strings.HasPrefix(digest, "sha256:") {
			digest = strings.TrimPrefix(digest, "sha256:")
		}
		
		return digest
	}
	
	localDigestClean := cleanDigest(localDigest)
	remoteDigestClean := cleanDigest(remoteDigest)
	
	// Log all digest formats for debugging
	d.logger.Debug("Local digest (original): %s", localDigest)
	d.logger.Debug("Local digest (cleaned): %s", localDigestClean)
	d.logger.Debug("Remote digest (original): %s", remoteDigest)
	d.logger.Debug("Remote digest (cleaned): %s", remoteDigestClean)
	
	// Compare cleaned digests
	shouldPull := localDigestClean != remoteDigestClean
	
	// If we're using the remote registry method for local digest, they should match
	// This is a special case where we know the digests should be the same
	if strings.Contains(localDigest, "(from remote registry)") && shouldPull {
		d.logger.Info("Local digest was obtained from remote registry but still differs from current remote digest")
		d.logger.Info("This suggests the remote image has been updated since the local image was pulled")
	}
	
	if shouldPull {
		d.logger.Info("Remote image %s has different digest, will pull", image)
		d.logger.Info("Local digest: %s", localDigestClean)
		d.logger.Info("Remote digest: %s", remoteDigestClean)
	} else {
		d.logger.Info("Image %s is up to date, skipping pull", image)
		d.logger.Info("Digest: %s", localDigestClean)
	}
	
	return shouldPull, nil
}
