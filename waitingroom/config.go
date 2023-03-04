package waitingroom

type Config struct {
	LogLevel string
	Listener string

	PermittedAccessSec  int   `mapstructure:"permitted_access_sec,omitempty"`   // アクセス許可後アクセスできる時間
	EntryDelaySec       int64 `mapstructure:"entry_delay_sec,omitempty"`        // 初回エントリーをDelayさせる秒数
	QueueEnableSec      int   `mapstructure:"queue_enable_sec,omitempty"`       // 待合室を有効にしておく時間
	PermitIntervalSec   int   `mapstructure:"permit_interval_sec,omitempty"`    // アクセス許可判定周期
	PermitUnitNumber    int64 `mapstructure:"permit_unit_number,omitempty"`     // アクセス許可する単位(PermitIntervalSecあたりPermitUnitNumber許可)
	CacheTTLSec         int   `mapstructure:"cache_ttl_sec,omitempty"`          // ローカルメモリキャッシュTTL
	NegativeCacheTTLSec int   `mapstructure:"negative_cache_ttl_sec,omitempty"` // ローカルメモリネガティブキャッシュTTL
}
