module xchat/services/internal/shared

go 1.26.0

require (
	filippo.io/edwards25519 v1.2.0
	github.com/stretchr/testify v1.12.1
	xchat/pkg/pool v0.0.0
)

replace xchat/pkg/pool => ../../../pkg/pool

require go.yaml.in/yaml/v3 v3.0.6 // indirect
