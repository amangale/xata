package clusters

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	tests := map[string]struct {
		cfg     Config
		wantErr string
	}{
		"valid config": {
			cfg: Config{
				ClustersStorageClass:        "xatastor",
				ClustersVolumeSnapshotClass: "xatastor",
			},
		},
		"missing storage class": {
			cfg: Config{
				ClustersVolumeSnapshotClass: "xatastor",
			},
			wantErr: "storage class is required",
		},
		"missing volume snapshot class": {
			cfg: Config{
				ClustersStorageClass: "xatastor",
			},
			wantErr: "volume snapshot class is required",
		},
		"xvol child storage class not configured": {
			cfg: Config{
				ClustersStorageClass:        "xatastor",
				ClustersVolumeSnapshotClass: "xatastor",
			},
		},
		"xvol child storage class configured": {
			cfg: Config{
				ClustersStorageClass:        "xatastor",
				ClustersVolumeSnapshotClass: "xatastor",
				XVolChildStorageClass:       "xatastor-slot",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
