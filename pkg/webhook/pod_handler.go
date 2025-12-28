package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/crypto"
)

const (
	// Annotation keys
	annotationInject   = "zen-lock/inject"
	annotationMountPath = "zen-lock/mount-path"
	
	// Default values
	defaultMountPath = "/zen-secrets"
	defaultVolumeName = "zen-secrets"
)

// PodHandler handles mutating admission webhook requests for Pods
type PodHandler struct {
	Client  client.Client
	decoder admission.Decoder
	crypto  crypto.Encryptor
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
		Client:  client,
		decoder: decoder,
		crypto:  encryptor,
	}, nil
}

// Handle processes admission requests
func (h *PodHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
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
		return admission.Errored(http.StatusInternalServerError, 
			fmt.Errorf("failed to fetch ZenLock %q: %w", injectName, err))
	}

	// Load private key from environment
	privateKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	if privateKey == "" {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("ZEN_LOCK_PRIVATE_KEY environment variable is not set"))
	}

	// Decrypt data
	decryptedMap, err := h.crypto.DecryptMap(zenlock.Spec.EncryptedData, privateKey)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("failed to decrypt ZenLock %q: %w", injectName, err))
	}

	// Convert decrypted map to Kubernetes Secret format (base64-encoded strings)
	secretData := make(map[string][]byte)
	for k, v := range decryptedMap {
		secretData[k] = v
	}

	// Generate unique secret name based on Pod UID
	secretName := fmt.Sprintf("zen-lock-inject-%s", string(pod.UID))

	// Create ephemeral Secret with OwnerReference to Pod
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: req.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       pod.Name,
					UID:        pod.UID,
					Controller: func() *bool { b := true; return &b }(),
				},
			},
		},
		Data: secretData,
	}

	// Create the secret
	if err := h.Client.Create(ctx, secret); err != nil {
		// If secret already exists (idempotency), that's okay
		if !strings.Contains(err.Error(), "already exists") {
			return admission.Errored(http.StatusInternalServerError,
				fmt.Errorf("failed to create ephemeral secret: %w", err))
		}
	}

	// Create JSON patch to add volume and volume mounts
	patch, err := h.createPatch(pod, secretName, mountPath)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("failed to create patch: %w", err))
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, patch)
}

// createPatch creates a JSON patch to add the secret volume and volume mounts
func (h *PodHandler) createPatch(pod *corev1.Pod, secretName, mountPath string) ([]byte, error) {
	patches := []map[string]interface{}{}

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
		// Ensure volumes array exists
		if len(pod.Spec.Volumes) == 0 {
			patches = append(patches, map[string]interface{}{
				"op":    "add",
				"path":  "/spec/volumes",
				"value": []interface{}{},
			})
		}
		volumePatch := map[string]interface{}{
			"op":    "add",
			"path":  "/spec/volumes/-",
			"value": map[string]interface{}{
				"name": defaultVolumeName,
				"secret": map[string]interface{}{
					"secretName": secretName,
				},
			},
		}
		patches = append(patches, volumePatch)
	}

	// Add volume mount to all containers
	for i := range pod.Spec.Containers {
		// Ensure volumeMounts array exists
		if len(pod.Spec.Containers[i].VolumeMounts) == 0 {
			patches = append(patches, map[string]interface{}{
				"op":    "add",
				"path":  fmt.Sprintf("/spec/containers/%d/volumeMounts", i),
				"value": []interface{}{},
			})
		}
		volumeMountPatch := map[string]interface{}{
			"op":   "add",
			"path": fmt.Sprintf("/spec/containers/%d/volumeMounts/-", i),
			"value": map[string]interface{}{
				"name":      defaultVolumeName,
				"mountPath": mountPath,
				"readOnly":  true,
			},
		}
		patches = append(patches, volumeMountPatch)
	}

	// Add volume mount to all init containers
	for i := range pod.Spec.InitContainers {
		// Ensure volumeMounts array exists
		if len(pod.Spec.InitContainers[i].VolumeMounts) == 0 {
			patches = append(patches, map[string]interface{}{
				"op":    "add",
				"path":  fmt.Sprintf("/spec/initContainers/%d/volumeMounts", i),
				"value": []interface{}{},
			})
		}
		volumeMountPatch := map[string]interface{}{
			"op":   "add",
			"path": fmt.Sprintf("/spec/initContainers/%d/volumeMounts/-", i),
			"value": map[string]interface{}{
				"name":      defaultVolumeName,
				"mountPath": mountPath,
				"readOnly":  true,
			},
		}
		patches = append(patches, volumeMountPatch)
	}

	return json.Marshal(patches)
}

