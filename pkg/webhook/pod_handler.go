package webhook

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/common"
	"github.com/kube-zen/zen-lock/pkg/controller/metrics"
	"github.com/kube-zen/zen-lock/pkg/crypto"
)

const (
	// Annotation keys
	annotationInject    = "zen-lock/inject"
	annotationMountPath = "zen-lock/mount-path"

	// Default values
	defaultMountPath  = "/zen-lock/secrets"
	defaultVolumeName = "zen-secrets"
)

// GenerateSecretName generates a stable secret name from namespace and pod name
// This function is exported for testing purposes
func GenerateSecretName(namespace, podName string) string {
	secretNameBase := fmt.Sprintf("zen-lock-inject-%s-%s", namespace, podName)
	hash := sha256.Sum256([]byte(secretNameBase))
	hashStr := hex.EncodeToString(hash[:])[:16] // Use first 16 chars of hash

	// Build name with hash suffix: zen-lock-inject-<namespace>-<podName>-<hash>
	// Hash suffix is critical for uniqueness, so we preserve it even when truncating
	prefix := fmt.Sprintf("zen-lock-inject-%s-%s-", namespace, podName)
	secretName := prefix + hashStr

	// Ensure name is valid (max 253 chars, lowercase alphanumeric + dash)
	// If truncation is needed, preserve hash suffix and truncate prefix
	if len(secretName) > 253 {
		maxPrefixLen := 253 - len(hashStr) - 1 // -1 for dash
		if maxPrefixLen < len("zen-lock-inject-") {
			// Extreme case: use minimal prefix + full hash
			secretName = fmt.Sprintf("zl-%s", hashStr)
		} else {
			// Truncate prefix, preserve hash
			truncatedPrefix := prefix[:maxPrefixLen]
			secretName = truncatedPrefix + hashStr
		}
	}
	return secretName
}

// PodHandler handles mutating admission webhook requests for Pods
type PodHandler struct {
	Client     client.Client
	decoder    admission.Decoder
	crypto     crypto.Encryptor
	privateKey string
	cache      *ZenLockCache
}

// NewPodHandler creates a new PodHandler
func NewPodHandler(client client.Client, scheme *runtime.Scheme) (*PodHandler, error) {
	decoder := admission.NewDecoder(scheme)

	// Load private key from environment (cached in handler)
	privateKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	if privateKey == "" {
		return nil, fmt.Errorf("ZEN_LOCK_PRIVATE_KEY environment variable is not set")
	}

	// Initialize crypto
	encryptor := crypto.NewAgeEncryptor()

	// Initialize cache with 5 minute TTL (configurable via env in future)
	cacheTTL := 5 * time.Minute
	if ttlStr := os.Getenv("ZEN_LOCK_CACHE_TTL"); ttlStr != "" {
		if parsedTTL, err := time.ParseDuration(ttlStr); err == nil {
			cacheTTL = parsedTTL
		}
	}
	cache := NewZenLockCache(cacheTTL)
	// Register cache for invalidation
	RegisterCache(cache)

	return &PodHandler{
		Client:     client,
		decoder:    decoder,
		crypto:     encryptor,
		privateKey: privateKey,
		cache:      cache,
	}, nil
}

// Handle processes admission requests
func (h *PodHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	startTime := time.Now()

	// Decode Pod
	pod := &corev1.Pod{}
	if err := h.decoder.Decode(req, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if injection is requested
	injectName := pod.GetAnnotations()[annotationInject]
	if injectName == "" {
		return admission.Allowed("no zen-lock injection requested")
	}

	// Validate inject annotation
	if err := ValidateInjectAnnotation(injectName); err != nil {
		duration := time.Since(startTime).Seconds()
		metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
		metrics.RecordValidationFailure(req.Namespace, "invalid_inject_annotation")
		return admission.Denied(fmt.Sprintf("invalid inject annotation: %v", err))
	}

	// Get mount path from annotation or use default
	mountPath := pod.GetAnnotations()[annotationMountPath]
	if mountPath == "" {
		mountPath = defaultMountPath
	} else {
		// Validate mount path if provided
		if err := ValidateMountPath(mountPath); err != nil {
			duration := time.Since(startTime).Seconds()
			metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
			metrics.RecordValidationFailure(req.Namespace, "invalid_mount_path")
			return admission.Denied(fmt.Sprintf("invalid mount path: %v", err))
		}
	}

	// Fetch ZenLock CRD (with caching)
	zenlockKey := types.NamespacedName{
		Name:      injectName,
		Namespace: req.Namespace,
	}

	// Try cache first
	zenlock, cacheHit := h.cache.Get(zenlockKey)
	if cacheHit {
		metrics.RecordCacheHit(req.Namespace, injectName)
	} else {
		metrics.RecordCacheMiss(req.Namespace, injectName)
		// Fetch from API server
		zenlock = &securityv1alpha1.ZenLock{}
		if err := h.Client.Get(ctx, zenlockKey, zenlock); err != nil {
			duration := time.Since(startTime).Seconds()
			metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
			// Sanitize error to prevent information leakage
			sanitizedErr := SanitizeError(err, "fetch ZenLock")
			return admission.Errored(http.StatusInternalServerError, sanitizedErr)
		}
		// Cache the result
		h.cache.Set(zenlockKey, zenlock)
	}

	// Validate AllowedSubjects if specified
	if len(zenlock.Spec.AllowedSubjects) > 0 {
		if err := h.validateAllowedSubjects(ctx, pod, zenlock.Spec.AllowedSubjects); err != nil {
			duration := time.Since(startTime).Seconds()
			metrics.RecordWebhookInjection(req.Namespace, injectName, "denied", duration)
			return admission.Denied(fmt.Sprintf("Pod ServiceAccount not allowed to use ZenLock %q: %v", injectName, err))
		}
	}

	// Decrypt data
	decryptStart := time.Now()
	decryptedMap, err := h.crypto.DecryptMap(zenlock.Spec.EncryptedData, h.privateKey)
	decryptDuration := time.Since(decryptStart).Seconds()
	if err != nil {
		duration := time.Since(startTime).Seconds()
		metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
		metrics.RecordDecryption(req.Namespace, injectName, "error", decryptDuration)
		// Invalidate cache on decryption failure (might be stale)
		h.cache.Invalidate(zenlockKey)
		// Sanitize error to prevent information leakage
		sanitizedErr := SanitizeError(err, "decrypt ZenLock")
		return admission.Errored(http.StatusInternalServerError, sanitizedErr)
	}

	// Record successful decryption
	metrics.RecordDecryption(req.Namespace, injectName, "success", decryptDuration)

	// Convert decrypted map to Kubernetes Secret format (base64-encoded strings)
	secretData := make(map[string][]byte)
	for k, v := range decryptedMap {
		secretData[k] = v
	}

	// Generate stable secret name from namespace and pod name (available at admission time)
	secretName := GenerateSecretName(req.Namespace, pod.Name)

	// Skip Secret creation/updates in dry-run mode (no side effects)
	if req.DryRun != nil && *req.DryRun {
		// In dry-run mode, we still mutate the Pod but skip Secret operations
		// This allows accurate mutation simulation without side effects
		mutatedPod := pod.DeepCopy()
		if err := h.mutatePod(mutatedPod, secretName, mountPath); err != nil {
			duration := time.Since(startTime).Seconds()
			metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
			sanitizedErr := SanitizeError(err, "mutate pod (dry-run)")
			return admission.Errored(http.StatusInternalServerError, sanitizedErr)
		}
		mutatedPodBytes, err := json.Marshal(mutatedPod)
		if err != nil {
			duration := time.Since(startTime).Seconds()
			metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
			sanitizedErr := SanitizeError(err, "marshal mutated pod (dry-run)")
			return admission.Errored(http.StatusInternalServerError, sanitizedErr)
		}
		duration := time.Since(startTime).Seconds()
		metrics.RecordWebhookInjection(req.Namespace, injectName, "success", duration)
		return admission.PatchResponseFromRaw(req.Object.Raw, mutatedPodBytes)
	}

	// Create ephemeral Secret with labels (OwnerReference will be set by controller later)
	// Pod UID is not available at admission time, so we use labels for tracking
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: req.Namespace,
			Labels: map[string]string{
				common.LabelPodName:      pod.Name,
				common.LabelPodNamespace: req.Namespace,
				common.LabelZenLockName:  injectName,
			},
		},
		Data: secretData,
	}

	// Create the secret with retry logic for transient errors
	retryConfig := common.DefaultRetryConfig()
	retryConfig.MaxAttempts = 3
	retryConfig.InitialDelay = 50 * time.Millisecond
	retryConfig.MaxDelay = 1 * time.Second

	createErr := common.Retry(ctx, retryConfig, func() error {
		return h.Client.Create(ctx, secret)
	})
	if createErr != nil {
		// If secret already exists, validate it's not stale
		if k8serrors.IsAlreadyExists(createErr) {
			// Fetch existing secret to validate it matches current ZenLock data (with retry)
			existingSecret := &corev1.Secret{}
			if err := common.Retry(ctx, retryConfig, func() error {
				return h.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: req.Namespace}, existingSecret)
			}); err != nil {
				duration := time.Since(startTime).Seconds()
				metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
				sanitizedErr := SanitizeError(err, "fetch existing secret")
				return admission.Errored(http.StatusInternalServerError, sanitizedErr)
			}

			// Ensure labels map is initialized (defense against nil labels from collision Secrets)
			if existingSecret.Labels == nil {
				existingSecret.Labels = make(map[string]string)
			}

			// Check if existing secret matches current ZenLock (by comparing ZenLock name label)
			existingZenLockName, hasZenLockLabel := existingSecret.Labels[common.LabelZenLockName]
			if !hasZenLockLabel || existingZenLockName != injectName {
				// Secret exists but is for a different ZenLock - this is stale, update it
				// Skip update in dry-run mode
				if req.DryRun == nil || !*req.DryRun {
					existingSecret.Data = secretData
					existingSecret.Labels[common.LabelZenLockName] = injectName
					existingSecret.Labels[common.LabelPodName] = pod.Name
					existingSecret.Labels[common.LabelPodNamespace] = req.Namespace
					
					// Retry update with exponential backoff for conflict/transient errors
					updateErr := common.Retry(ctx, retryConfig, func() error {
						return h.Client.Update(ctx, existingSecret)
					})
					if updateErr != nil {
						duration := time.Since(startTime).Seconds()
						metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
						sanitizedErr := SanitizeError(updateErr, "update stale secret")
						return admission.Errored(http.StatusInternalServerError, sanitizedErr)
					}
				}
			} else {
				// Secret exists and matches current ZenLock - verify data matches (defense in depth)
				// If ZenLock was updated, secret data should be refreshed
				dataMatches := true
				if len(existingSecret.Data) != len(secretData) {
					dataMatches = false
				} else {
					for k, v := range secretData {
						if existingVal, ok := existingSecret.Data[k]; !ok || !bytes.Equal(existingVal, v) {
							dataMatches = false
							break
						}
					}
				}
				if !dataMatches {
					// Data doesn't match - update secret with fresh data
					existingSecret.Data = secretData
					
					// Retry update with exponential backoff for conflict/transient errors
					updateErr := common.Retry(ctx, retryConfig, func() error {
						return h.Client.Update(ctx, existingSecret)
					})
					if updateErr != nil {
						duration := time.Since(startTime).Seconds()
						metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
						sanitizedErr := SanitizeError(updateErr, "refresh secret data")
						return admission.Errored(http.StatusInternalServerError, sanitizedErr)
					}
				}
			}
			// Secret already exists and is valid - continue with injection
		} else {
			// Other error creating secret (non-retryable or retries exhausted)
			duration := time.Since(startTime).Seconds()
			metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
			sanitizedErr := SanitizeError(createErr, "create ephemeral secret")
			return admission.Errored(http.StatusInternalServerError, sanitizedErr)
		}
	}

	// Mutate Pod object in-memory (correct pattern for PatchResponseFromRaw)
	mutatedPod := pod.DeepCopy()
	if err := h.mutatePod(mutatedPod, secretName, mountPath); err != nil {
		duration := time.Since(startTime).Seconds()
		metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
		sanitizedErr := SanitizeError(err, "mutate pod")
		return admission.Errored(http.StatusInternalServerError, sanitizedErr)
	}

	// Marshal mutated pod
	mutatedPodBytes, err := json.Marshal(mutatedPod)
	if err != nil {
		duration := time.Since(startTime).Seconds()
		metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
		sanitizedErr := SanitizeError(err, "marshal mutated pod")
		return admission.Errored(http.StatusInternalServerError, sanitizedErr)
	}

	// Record successful injection
	duration := time.Since(startTime).Seconds()
	metrics.RecordWebhookInjection(req.Namespace, injectName, "success", duration)

	// Use PatchResponseFromRaw with original and mutated bytes (correct pattern)
	return admission.PatchResponseFromRaw(req.Object.Raw, mutatedPodBytes)
}

// mutatePod mutates the Pod object in-memory to add volume and volume mounts
func (h *PodHandler) mutatePod(pod *corev1.Pod, secretName, mountPath string) error {
	// Check if volume already exists
	volumeExists := false
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == defaultVolumeName {
			volumeExists = true
			break
		}
	}

	// Add volume to pod spec if it doesn't exist
	if !volumeExists {
		volume := corev1.Volume{
			Name: defaultVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}
		pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
	}

	// Add volume mount to all containers
	for i := range pod.Spec.Containers {
		// Check if mount already exists
		mountExists := false
		for _, mount := range pod.Spec.Containers[i].VolumeMounts {
			if mount.Name == defaultVolumeName {
				mountExists = true
				break
			}
		}
		if !mountExists {
			volumeMount := corev1.VolumeMount{
				Name:      defaultVolumeName,
				MountPath: mountPath,
				ReadOnly:  true,
			}
			pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts, volumeMount)
		}
	}

	// Add volume mount to all init containers
	for i := range pod.Spec.InitContainers {
		// Check if mount already exists
		mountExists := false
		for _, mount := range pod.Spec.InitContainers[i].VolumeMounts {
			if mount.Name == defaultVolumeName {
				mountExists = true
				break
			}
		}
		if !mountExists {
			volumeMount := corev1.VolumeMount{
				Name:      defaultVolumeName,
				MountPath: mountPath,
				ReadOnly:  true,
			}
			pod.Spec.InitContainers[i].VolumeMounts = append(pod.Spec.InitContainers[i].VolumeMounts, volumeMount)
		}
	}

	return nil
}

// validateAllowedSubjects checks if the Pod's ServiceAccount is allowed to use the ZenLock
func (h *PodHandler) validateAllowedSubjects(ctx context.Context, pod *corev1.Pod, allowedSubjects []securityv1alpha1.SubjectReference) error {
	podServiceAccount := pod.Spec.ServiceAccountName
	if podServiceAccount == "" {
		podServiceAccount = "default"
	}

	podNamespace := pod.Namespace
	if podNamespace == "" {
		podNamespace = "default"
	}

	for _, subject := range allowedSubjects {
		// Only ServiceAccount is supported (User and Group require additional resolution)
		if subject.Kind != "ServiceAccount" {
			continue
		}

		subjectNamespace := subject.Namespace
		if subjectNamespace == "" {
			subjectNamespace = podNamespace
		}

		if subject.Name == podServiceAccount && subjectNamespace == podNamespace {
			return nil // Allowed
		}
	}

	return fmt.Errorf("ServiceAccount %q in namespace %q is not in the allowed subjects list", podServiceAccount, podNamespace)
}
