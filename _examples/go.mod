module github.com/wmulabs/eywa/examples

go 1.26.4

require (
	github.com/gofiber/fiber/v2 v2.52.13
	github.com/wmulabs/eywa v1.24.0
	github.com/wmulabs/eywa/fiber v1.24.0
	github.com/wmulabs/eywa/mcp v1.24.0
	github.com/wmulabs/eywa/mongo v1.24.0
	github.com/wmulabs/eywa/providers/openai v1.24.0
	github.com/wmulabs/eywa/redis v1.24.0
)

require (
	github.com/andybalholm/brotli v1.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-redis/redis_rate/v10 v10.0.1 // indirect
	github.com/go-redsync/redsync/v4 v4.16.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/klauspost/compress v1.18.6 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/mattn/go-runewidth v0.0.23 // indirect
	github.com/montanaflynn/stats v0.9.0 // indirect
	github.com/nyaruka/phonenumbers v1.8.0 // indirect
	github.com/openai/openai-go v1.12.0 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/redis/go-redis/extra/rediscmd/v9 v9.20.1 // indirect
	github.com/redis/go-redis/extra/redisotel/v9 v9.20.1 // indirect
	github.com/redis/go-redis/v9 v9.20.1 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/tidwall/gjson v1.19.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/tinylib/msgp v1.6.4 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.71.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.mongodb.org/mongo-driver v1.17.9 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo v0.69.0 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/wmulabs/eywa => ../
	github.com/wmulabs/eywa/fiber => ../fiber
	github.com/wmulabs/eywa/mcp => ../mcp
	github.com/wmulabs/eywa/mongo => ../mongo
	github.com/wmulabs/eywa/providers/openai => ../providers/openai
	github.com/wmulabs/eywa/redis => ../redis
)
