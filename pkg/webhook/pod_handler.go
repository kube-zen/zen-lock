package webhook

import (
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

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.zen.io/v1alpha1"
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

	// Label keys for Secret tracking
	labelPodName      = "zen-lock.security.zen.io/pod-name"
	labelPodNamespace = "zen-lock.security.zen.io/pod-namespace"
	labelZenLockName  = "zen-lock.security.zen.io/zenlock-name"
)

// GenerateSecretName generates a stable secret name from namespace and pod name
// This function is exported for testing purposes
func GenerateSecretName(namespace, podName string) string {
	secretNameBase := fmt.Sprintf("zen-lock-inject-%s-%s", namespace, podName)
	hash := sha256.Sum256([]byte(secretNameBase))
	hashStr := hex.EncodeToString(hash[:])[:16] // Use first 16 chars of hash
	secretName := fmt.Sprintf("zen-lock-inject-%s-%s-%s", namespace, podName, hashStr)
	// Ensure name is valid (max 253 chars, lowercase alphanumeric + dash)
	if len(secretName) > 253 {
		secretName = secretName[:253]
	}
	return secretName
}

// PodHandler handles mutating admission webhook requests for Pods
type PodHandler struct {
	Client     client.Client
	decoder    admission.Decoder
	crypto     crypto.Encryptor
	privateKey string
}

// NewPodHandler creates a new PodHandler
func NewPodHandler(client client.Client, scheme *runtime.Scheme) (*PodHandler, error) {
	decoder := admission.NewDecoder(scheme)

	// Load private key from environment
	privateKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	if privateKey == "" {
		return nil, fmt.Errorf("ZEN_LOCK_PRIVATE_KEY environment variable is not set")
	}

	// Initialize crypto
	encryptor := crypto.NewAgeEncryptor()

	return &PodHandler{
		Client:     client,
		decoder:    decoder,
		crypto:     encryptor,
		privateKey: privateKey,
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

	// Get mount path from annotation or use default
	mountPath := pod.GetAnnotations()[annotationMountPath]
	if mountPath == "" {
		mountPath = defaultMountPath
	}

	// Fetch ZenLock CRD
	zenlock := &securityv1alpha1.ZenLock{}
	zenlockKey := types.NamespacedName{
		Name:      injectName,
		Namespace: req.Namespace,
	}

	if err := h.Client.Get(ctx, zenlockKey, zenlock); err != nil {
		duration := time.Since(startTime).Seconds()
		metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("failed to fetch ZenLock %q: %w", injectName, err))
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
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("failed to decrypt ZenLock %q: %w", injectName, err))
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

	// Create ephemeral Secret with labels (OwnerReference will be set by controller later)
	// Pod UID is not available at admission time, so we use labels for tracking
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: req.Namespace,
			Labels: map[string]string{
				labelPodName:      pod.Name,
				labelPodNamespace: req.Namespace,
				labelZenLockName:  injectName,
			},
		},
		Data: secretData,
	}

	// Create the secret
	if err := h.Client.Create(ctx, secret); err != nil {
		// If secret already exists (idempotency), that's okay - use existing secret
		if !k8serrors.IsAlreadyExists(err) {
			duration := time.Since(startTime).Seconds()
			metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
			return admission.Errored(http.StatusInternalServerError,
				fmt.Errorf("failed to create ephemeral secret: %w", err))
		}
		// Secret already exists - this is fine for idempotency
	}

	// Mutate Pod object in-memory (correct pattern for PatchResponseFromRaw)
	mutatedPod := pod.DeepCopy()
	if err := h.mutatePod(mutatedPod, secretName, mountPath); err != nil {
		duration := time.Since(startTime).Seconds()
		metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("failed to mutate pod: %w", err))
	}

	// Marshal mutated pod
	mutatedPodBytes, err := json.Marshal(mutatedPod)
	if err != nil {
		duration := time.Since(startTime).Seconds()
		metrics.RecordWebhookInjection(req.Namespace, injectName, "error", duration)
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("failed to marshal mutated pod: %w", err))
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
