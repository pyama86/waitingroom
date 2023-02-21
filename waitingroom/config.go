package waitingroom

type Config struct {
	LogLevel string
	Listener string

	ClientPollingIntervalSec int   `mapstructure:"client_polling_interval_sec",omitempty` // Clientがポーリングしてくる必要がある間隔
	AllowedAccessSec         int   `mapstructure:"allowed_access_sec",omitempty`          // アクセス許可後アクセスできる時間
	EntryDelaySec            int64 `mapstructure:"entry_delay_sec",omitempty`             // 初回エントリーをDelayさせる秒数
	QueueEnableSec           int   `mapstructure:"queue_enable_sec",omitempty`            // 待合室を有効にしておく時間
	AllowIntervalSec         int   `mapstructure:"allow_interval_sec",omitempty`          // アクセス許可判定周期
	AllowUnitNumber          int64 `mapstructure:"allow_unit_number",omitempty`           // アクセス許可する単位(AllowIntervalSecあたりAllowUnitNumber許可)
	CacheTTLSec              int   `mapstructure:"cache_ttl_sec",omitempty`               // ローカルメモリキャッシュTTL
}
