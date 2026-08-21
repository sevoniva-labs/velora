//go:build tools

package tools

import (
	_ "github.com/anchore/syft/cmd/syft"
	_ "github.com/bufbuild/buf/cmd/buf"
	_ "github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2"
	_ "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
	_ "github.com/zricethezav/gitleaks/v8"
	_ "github.com/securego/gosec/v2/cmd/gosec"
	_ "golang.org/x/vuln/cmd/govulncheck"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
	_ "honnef.co/go/tools/cmd/staticcheck"
)
