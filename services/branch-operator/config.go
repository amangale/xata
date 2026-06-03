package branchoperator

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	"xata/internal/envcfg"
)

type Config struct {
	ClustersNamespace string `env:"XATA_CLUSTERS_NAMESPACE" env-default:"xata-clusters" env-description:"namespace where the operator creates managed resources"`
	BackupsBucket     string `env:"XATA_BACKUPS_BUCKET" env-description:"bucket for storing the cluster backups"`
	BackupsEndpoint   string `env:"XATA_BACKUPS_ENDPOINT" env-description:"endpoint for reaching the backups bucket; set for a non-AWS S3-compatible store such as Cloudflare R2"`
	// When BackupsEndpoint is set (non-AWS S3-compatible store), backups
	// authenticate with static credentials from this Secret instead of an IAM
	// role. Defaults match the local-dev MinIO secret.
	BackupsCredentialsSecretName         string   `env:"XATA_BACKUPS_CREDENTIALS_SECRET_NAME" env-default:"minio-eu" env-description:"Secret (in the clusters namespace) holding static S3 credentials, used when XATA_BACKUPS_ENDPOINT is set"`
	BackupsCredentialsAccessKeyIDKey     string   `env:"XATA_BACKUPS_CREDENTIALS_ACCESS_KEY_ID_KEY" env-default:"rootUser" env-description:"key in the credentials Secret holding the access key ID"`
	BackupsCredentialsSecretAccessKeyKey string   `env:"XATA_BACKUPS_CREDENTIALS_SECRET_ACCESS_KEY_KEY" env-default:"rootPassword" env-description:"key in the credentials Secret holding the secret access key"`
	CloudProvider                        string   `env:"XATA_CLOUD_PROVIDER" env-default:"aws" env-description:"cloud provider for per-Branch identity and backup credentials: aws or gcp"`
	BarmanRegionSecretName               string   `env:"XATA_BARMAN_REGION_SECRET_NAME" env-default:"barman-dummy-secret" env-description:"chart-managed secret referenced as the barman AWS region"`
	BarmanRegionSecretKey                string   `env:"XATA_BARMAN_REGION_SECRET_KEY" env-default:"dummy" env-description:"key in the chart-managed barman AWS region secret"`
	TolerationsRaw                       []string `env:"XATA_CLUSTERS_TOLERATIONS" env-default:"xata.io/workload=dataplane:NoSchedule" env-separator:"," env-description:"tolerations for cluster pods in the format key=value:effect"`
	Tolerations                          []corev1.Toleration
	EnforceZone                          bool          `env:"XATA_ENFORCE_ZONE" env-default:"false" env-description:"enable zone-based pod anti-affinity for multi-instance clusters"`
	ImagePullSecrets                     []string      `env:"XATA_IMAGE_SECRETS" env-default:"ghcr-secret" env-description:"image pull secrets for private PostgreSQL images"`
	XatastorEnabled                      bool          `env:"XATA_XATASTOR_ENABLED" env-default:"true" env-description:"enable xatastor CSI integration for wakeup requests"`
	CSINodeNamespace                     string        `env:"XATA_CSI_NODE_NAMESPACE" env-default:"xatastor" env-description:"namespace where CSI node plugin pods run"`
	CSINodePort                          int           `env:"XATA_CSI_NODE_PORT" env-default:"50061" env-description:"port for the SlotController service on CSI node plugin pods"`
	WakeupRequestTTL                     time.Duration `env:"XATA_WAKEUP_REQUEST_TTL" env-default:"60s" env-description:"time to keep completed WakeupRequests before deletion"`
	WakeupMaxConcurrent                  int           `env:"XATA_WAKEUP_MAX_CONCURRENT" env-default:"16" env-description:"maximum concurrent wakeup reconciliations"`
}

func (cfg *Config) ParseTolerations() error {
	if len(cfg.TolerationsRaw) == 0 {
		return fmt.Errorf("tolerations are required but not set")
	}

	var tl envcfg.TolerationListField
	err := tl.SetValue(cfg.TolerationsRaw)
	if err != nil {
		return fmt.Errorf("failed to parse tolerations: %w", err)
	}
	cfg.Tolerations = tl.Value
	return nil
}
