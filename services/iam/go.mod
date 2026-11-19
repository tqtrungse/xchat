module xchat/services/iam

go 1.26.0

require (
	xchat/pkg/bytesconv v0.0.0
	xchat/pkg/collection v0.0.0
	xchat/pkg/config v0.0.0
	xchat/pkg/database v0.0.0
	xchat/pkg/generator v0.0.0
	xchat/pkg/jwt v0.0.0
	xchat/pkg/k8s v0.0.0
	xchat/pkg/log v0.0.0
	xchat/pkg/pool v0.0.0
	xchat/pkg/reject v0.0.0
	xchat/services/internal/shared v0.0.0
)

replace (
	xchat/pkg/bytesconv => ../../pkg/bytesconv
	xchat/pkg/collection => ../../pkg/collection
	xchat/pkg/config => ../../pkg/config
	xchat/pkg/cpu => ../../pkg/cpu
	xchat/pkg/database => ../../pkg/database
	xchat/pkg/generator => ../../pkg/generator
	xchat/pkg/jitter => ../../pkg/jitter
	xchat/pkg/jwt => ../../pkg/jwt
	xchat/pkg/k8s => ../../pkg/k8s
	xchat/pkg/log => ../../pkg/log
	xchat/pkg/pool => ../../pkg/pool
	xchat/pkg/reject => ../../pkg/reject
	xchat/services/internal/shared => ../internal/shared
)

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/gin-gonic/gin v1.12.0
	github.com/google/go-cmp v0.7.0
	github.com/mailru/easyjson v0.9.2
	github.com/spf13/viper v1.21.0
	github.com/stretchr/testify v1.12.1
	go.uber.org/mock v0.6.0
	go.uber.org/zap v1.28.0
	golang.org/x/crypto v0.59.0
	gorm.io/gorm v1.31.2
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/MicahParks/jwkset v0.11.5 // indirect
	github.com/MicahParks/keyfunc/v3 v3.9.2 // indirect
	github.com/bytedance/gopkg v0.1.4 // indirect
	github.com/bytedance/sonic v1.15.0 // indirect
	github.com/bytedance/sonic/loader v0.5.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.12 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.1 // indirect
	github.com/go-sql-driver/mysql v1.10.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3-0.20260624044523-35f78d5908f1 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.59.1 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/sony/sonyflake/v2 v2.2.0 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.1 // indirect
	go.mongodb.org/mongo-driver/v2 v2.9.2 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/arch v0.22.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gorm.io/driver/mysql v1.6.0 // indirect
	gorm.io/plugin/dbresolver v1.6.2 // indirect
	xchat/pkg/cpu v0.0.0 // indirect
	xchat/pkg/jitter v0.0.0 // indirect
)
