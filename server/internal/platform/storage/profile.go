package storage

import (
	"errors"
	"strings"
)

// ProviderProfile identifies the target product behind the S3 protocol.
// It is metadata for configuration and evidence; it does not certify a target.
type ProviderProfile string

const (
	ProviderProfileGenericS3  ProviderProfile = "generic-s3"
	ProviderProfileAWSS3      ProviderProfile = "aws-s3"
	ProviderProfileMinIO      ProviderProfile = "minio"
	ProviderProfileCephRGW    ProviderProfile = "ceph-rgw"
	ProviderProfileAlibabaOSS ProviderProfile = "alibaba-oss"
	ProviderProfileTencentCOS ProviderProfile = "tencent-cos"
	ProviderProfileHuaweiOBS  ProviderProfile = "huawei-obs"
	ProviderProfileLocal      ProviderProfile = "local"
)

var errUnsupportedProfile = errors.New("unsupported storage provider profile")

// ResolveProviderProfile maps product aliases to the explicit target profile.
// All S3 profiles use the same capability-negotiated protocol adapter.
func ResolveProviderProfile(value string) (ProviderProfile, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "local", "filesystem":
		return ProviderProfileLocal, nil
	case "s3", "aws", "aws-s3", "amazon-s3":
		return ProviderProfileAWSS3, nil
	case "s3-compatible", "s3_compatible", "generic-s3", "generic":
		return ProviderProfileGenericS3, nil
	case "minio", "minio-s3":
		return ProviderProfileMinIO, nil
	case "ceph", "ceph-rgw", "radosgw":
		return ProviderProfileCephRGW, nil
	case "oss", "aliyun-oss", "alibaba-oss":
		return ProviderProfileAlibabaOSS, nil
	case "cos", "tencent-cos":
		return ProviderProfileTencentCOS, nil
	case "obs", "huawei-obs":
		return ProviderProfileHuaweiOBS, nil
	default:
		return "", errUnsupportedProfile
	}
}

// IsS3Profile reports whether a profile uses the S3 protocol adapter.
func IsS3Profile(profile ProviderProfile) bool {
	return profile != ProviderProfileLocal
}
