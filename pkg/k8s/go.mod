module xchat/pkg/k8s

go 1.26.0

require (
	github.com/stretchr/testify v1.12.1
	xchat/pkg/generator v0.0.0
)

require (
	github.com/sony/sonyflake/v2 v2.2.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
)

replace (
	xchat/pkg/generator => ../generator
)
