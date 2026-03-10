package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// K8sLauncher wraps kubectl and kind for managing a local Kubernetes repro environment.
type K8sLauncher struct {
	projectDir  string
	clusterName string // default: "mm-repro"
	namespace   string // default: "mattermost-repro"
}

// NewK8sLauncher creates a K8sLauncher for the given repro project directory.
func NewK8sLauncher(projectDir string) (*K8sLauncher, error) {
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("invalid project directory: %w", err)
	}
	if _, err := os.Stat(filepath.Join(abs, "kubernetes")); err != nil {
		return nil, fmt.Errorf("kubernetes/ directory not found in %s — is this a kubernetes repro project?", abs)
	}
	return &K8sLauncher{
		projectDir:  abs,
		clusterName: "mm-repro",
		namespace:   "mattermost-repro",
	}, nil
}

// Up creates the kind cluster (if not exists) and applies all manifests.
func (l *K8sLauncher) Up() error {
	fmt.Println("Creating kind cluster 'mm-repro' (skipped if already exists)...")
	// kind create cluster is idempotent-ish; ignore error if cluster already exists
	kindCreate := exec.Command("kind", "create", "cluster", "--name", l.clusterName)
	kindCreate.Stdout = os.Stdout
	kindCreate.Stderr = os.Stderr
	_ = kindCreate.Run() // intentionally ignore error — cluster may already exist

	fmt.Println("Applying Kubernetes manifests...")
	return l.kubectl("apply", "-f", filepath.Join(l.projectDir, "kubernetes/"))
}

// Down deletes all manifests but keeps the kind cluster and its data.
func (l *K8sLauncher) Down() error {
	fmt.Println("Deleting Kubernetes manifests (cluster and volumes preserved)...")
	return l.kubectl("delete", "-f", filepath.Join(l.projectDir, "kubernetes/"), "--ignore-not-found")
}

// Reset deletes the entire kind cluster including all data.
func (l *K8sLauncher) Reset() error {
	fmt.Printf("Deleting kind cluster '%s' (all data will be lost)...\n", l.clusterName)
	return l.run("kind", "delete", "cluster", "--name", l.clusterName)
}

// Status shows pod status in the mattermost-repro namespace.
func (l *K8sLauncher) Status() error {
	return l.kubectl("get", "pods", "-n", l.namespace)
}

// Logs streams logs for the mattermost pods.
func (l *K8sLauncher) Logs(follow bool) error {
	args := []string{"logs", "-n", l.namespace, "-l", "app=mattermost"}
	if follow {
		args = append(args, "-f")
	}
	return l.kubectl(args...)
}

// CheckKubectl verifies kubectl is available.
func CheckKubectl() error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl not found in PATH — install it from https://kubernetes.io/docs/tasks/tools/")
	}
	return nil
}

// CheckKind verifies kind is available.
func CheckKind() error {
	if _, err := exec.LookPath("kind"); err != nil {
		return fmt.Errorf("kind not found in PATH — install it from https://kind.sigs.k8s.io/docs/user/quick-start/")
	}
	return nil
}

func (l *K8sLauncher) kubectl(args ...string) error {
	return l.run("kubectl", args...)
}

func (l *K8sLauncher) run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = l.projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
