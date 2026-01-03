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

	"github.com/kube-zen/zen-sdk/pkg/retry"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/common"
	"github.com/kube-zen/zen-lock/pkg/config"
	"github.com/kube-zen/zen-lock/pkg/controller/metrics"
	"github.com/kube-zen/zen-lock/pkg/crypto"
)

// GenerateSecretName generates a stable secret name from namespace and pod name
// This function is exported for testing purposes
func GenerateSecretName(namespace, podName string) string {
	// Generate a stable name with hash suffix to ensure uniqueness and stay within Kubernetes limits
	base := fmt.Sprintf("zen-lock-inject-%s-%s", namespace, podName)
	
	// Kubernetes resource names must be <= 253 characters
	// If base is too long, truncate and add hash
	const maxLength = 253
	const hashLength = 8 // 8 hex chars = 4 bytes
	
	if len(base) <= maxLength-hashLength-1 {
		return base
	}
	
	// Truncate and add hash
	maxBaseLength := maxLength - hashLength - 1 // -1 for hyphen
	truncated := base[:maxBaseLength]
	
	// Generate hash of full name for uniqueness
	hash := sha256.Sum256([]byte(base))
	hashSuffix := hex.EncodeToString(hash[:4]) // Use first 4 bytes = 8 hex chars
	
	return fmt.Sprintf("%s-%s", truncated, hashSuffix)
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

	// Initialize cache with 5 minute TTL (configurable via ZEN_LOCK_CACHE_TTL env var)
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

// handleDryRun handles dry-run mode by mutating the pod without creating secrets
func (h *PodHandler) handleDryRun(ctx context.Context, pod *corev1.Pod, secretName, mountPath, injectName, namespace string, startTime time.Time, originalObject []byte) admission.Response {
	mutatedPod := pod.DeepCopy()
	if err := h.mutatePod(mutatedPod, secretName, mountPath); err != nil {
		duration := time.Since(startTime).Seconds()
		metrics.RecordWebhookInjection(namespace, injectName, "error", duration)
		sanitizedErr := SanitizeError(err, "mutate pod (dry-run)")
		return admission.Errored(http.StatusInternalServerError, sanitizedErr)
	}
	mutatedPodBytes, err := json.Marshal(mutatedPod)
	if err != nil {
		duration := time.Since(startTime).Seconds()
		metrics.RecordWebhookInjection(namespace, injectName, "error", duration)
		sanitizedErr := SanitizeError(err, "marshal mutated pod (dry-run)")
		return admission.Errored(http.StatusInternalServerError, sanitizedErr)
	}
	duration := time.Since(startTime).Seconds()
	metrics.RecordWebhookInjection(namespace, injectName, "success", duration)
	return admission.PatchResponseFromRaw(originalObject, mutatedPodBytes)
}

// ensureSecretExists ensures the secret exists and is up-to-date, handling conflicts and stale data
func (h *PodHandler) ensureSecretExists(ctx context.Context, secret *corev1.Secret, secretName, injectName, namespace, podName string, secretData map[string][]byte, startTime time.Time, retryConfig retry.Config, isDryRun bool) error {
	createErr := retry.Do(ctx, retryConfig, func() error {
		return h.Client.Create(ctx, secret)
	})
	if createErr == nil {
		return nil // Secret created successfully
	}

	// If secret already exists, validate it's not stale
	if !k8serrors.IsAlreadyExists(createErr) {
		return createErr // Other error creating secret
	}

	// Fetch existing secret to validate it matches current ZenLock data
	existingSecret := &corev1.Secret{}
	if err := retry.Do(ctx, retryConfig, func() error {
		return h.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, existingSecret)
	}); err != nil {
		return err
	}

	// Ensure labels map is initialized
	if existingSecret.Labels == nil {
		// Pre-allocate labels map with estimated size (Go 1.25 optimization)
		existingSecret.Labels = make(map[string]string, 4)
	}

	// Check if existing secret matches current ZenLock
	existingZenLockName, hasZenLockLabel := existingSecret.Labels[common.LabelZenLockName]
	if !hasZenLockLabel || existingZenLockName != injectName {
		// Secret exists but is for a different ZenLock - update it
		if !isDryRun {
			existingSecret.Data = secretData
			existingSecret.Labels[common.LabelZenLockName] = injectName
			existingSecret.Labels[common.LabelPodName] = podName
			existingSecret.Labels[common.LabelPodNamespace] = namespace
			if err := retry.Do(ctx, retryConfig, func() error {
				return h.Client.Update(ctx, existingSecret)
			}); err != nil {
				return err
			}
		}
		return nil
	}

	// Secret exists and matches current ZenLock - verify data matches
	if !h.secretDataMatches(existingSecret.Data, secretData) {
		// Data doesn't match - update secret with fresh data
		if !isDryRun {
			existingSecret.Data = secretData
			if err := retry.Do(ctx, retryConfig, func() error {
				return h.Client.Update(ctx, existingSecret)
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// secretDataMatches checks if two secret data maps are equal
func (h *PodHandler) secretDataMatches(existing, expected map[string][]byte) bool {
	if len(existing) != len(expected) {
		return false
	}
	for k, v := range expected {
		if existingVal, ok := existing[k]; !ok || !bytes.Equal(existingVal, v) {
			return false
		}
	}
	return true
}

// Handle processes admission requests
func (h *PodHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	// Add timeout to context (configurable via ZEN_LOCK_WEBHOOK_TIMEOUT env var)
	webhookTimeout := config.DefaultWebhookTimeout
	if timeoutStr := os.Getenv("ZEN_LOCK_WEBHOOK_TIMEOUT"); timeoutStr != "" {
		if parsedTimeout, err := time.ParseDuration(timeoutStr); err == nil {
			webhookTimeout = parsedTimeout
		}
	}
	ctx, cancel := context.WithTimeout(ctx, webhookTimeout)
	defer cancel()

	startTime := time.Now()

	// Decode Pod
	pod := &corev1.Pod{}
	if err := h.decoder.Decode(req, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// HA enforcement is handled upstream by zen-lead's Validating Admission Webhook
	// If this Pod creation request reached here, it means zen-lead has allowed it (it's the leader)
	// No need to check leader status here - zen-lead blocks non-leader Pod creation at the API level

	// Check if injection is requested
	injectName := pod.GetAnnotations()[config.AnnotationInject]
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
	mountPath := pod.GetAnnotations()[config.AnnotationMountPath]
	if mountPath == "" {
		mountPath = config.DefaultMountPath
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
	// Pre-allocate with known size for better performance (Go 1.25 optimization)
	secretData := make(map[string][]byte, len(decryptedMap))
	for k, v := range decryptedMap {
		secretData[k] = v
	}

	// Generate stable secret name from namespace and pod name (available at admission time)
	secretName := GenerateSecretName(req.Namespace, pod.Name)

	// Skip Secret creation/updates in dry-run mode (no side effects)
	isDryRun := req.DryRun != nil && *req.DryRun
	if isDryRun {
		return h.handleDryRun(ctx, pod, secretName, mountPath, injectName, req.Namespace, startTime, req.Object.Raw)
	}

	// Create ephemeral Secret with labels (OwnerReference will be set by controller later)
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

	// Ensure secret exists and is up-to-date
	retryConfig := retry.DefaultConfig()
	retryConfig.MaxAttempts = config.DefaultRetryMaxAttempts
	retryConfig.InitialDelay = config.DefaultWebhookRetryInitialDelay
	retryConfig.MaxDelay = config.DefaultWebhookRetryMaxDelay

	if err := h.ensureSecretExists(ctx, secret, secretName, injectName, req.Namespace, pod.Name, secretData, startTime, retryConfig, isDryRun); err != nil {
		duration := time.Since(startTime).Seconds()
		metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
		sanitizedErr := SanitizeError(err, "create ephemeral secret")
		return admission.Errored(http.StatusInternalServerError, sanitizedErr)
	}

	// Mutate Pod object and return response
	return h.createMutationResponse(pod, secretName, mountPath, injectName, req.Namespace, startTime, req.Object.Raw)
}

// createMutationResponse mutates the pod and creates the admission response
func (h *PodHandler) createMutationResponse(pod *corev1.Pod, secretName, mountPath, injectName, namespace string, startTime time.Time, originalObject []byte) admission.Response {
	mutatedPod := pod.DeepCopy()
	if err := h.mutatePod(mutatedPod, secretName, mountPath); err != nil {
		duration := time.Since(startTime).Seconds()
		metrics.RecordWebhookInjection(namespace, injectName, "error", duration)
		sanitizedErr := SanitizeError(err, "mutate pod")
		return admission.Errored(http.StatusInternalServerError, sanitizedErr)
	}

	mutatedPodBytes, err := json.Marshal(mutatedPod)
	if err != nil {
		duration := time.Since(startTime).Seconds()
		metrics.RecordWebhookInjection(namespace, injectName, "error", duration)
		sanitizedErr := SanitizeError(err, "marshal mutated pod")
		return admission.Errored(http.StatusInternalServerError, sanitizedErr)
	}

	duration := time.Since(startTime).Seconds()
	metrics.RecordWebhookInjection(namespace, injectName, "success", duration)
	return admission.PatchResponseFromRaw(originalObject, mutatedPodBytes)
}

// mutatePod mutates the Pod object in-memory to add volume and volume mounts
func (h *PodHandler) mutatePod(pod *corev1.Pod, secretName, mountPath string) error {
	// Check if volume already exists
	volumeExists := false
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == config.DefaultVolumeName {
			volumeExists = true
			break
		}
	}

	// Add volume to pod spec if it doesn't exist
	if !volumeExists {
		volume := corev1.Volume{
			Name: config.DefaultVolumeName,
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
			if mount.Name == config.DefaultVolumeName {
				mountExists = true
				break
			}
		}
		if !mountExists {
			volumeMount := corev1.VolumeMount{
				Name:      config.DefaultVolumeName,
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
			if mount.Name == config.DefaultVolumeName {
				mountExists = true
				break
			}
		}
		if !mountExists {
			volumeMount := corev1.VolumeMount{
				Name:      config.DefaultVolumeName,
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
