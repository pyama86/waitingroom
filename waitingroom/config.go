package waitingroom

type Config struct {
	LogLevel string
	Listener string

	PermittedAccessSec  int    `mapstructure:"permitted_access_sec,omitempty" validate:"required"`                                                  // アクセス許可後アクセスできる時間
	EntryDelaySec       int64  `mapstructure:"entry_delay_sec,omitempty" validate:"required"`                                                       // 初回エントリーをDelayさせる秒数
	QueueEnableSec      int    `mapstructure:"queue_enable_sec,omitempty" validate:"required"`                                                      // 待合室を有効にしておく時間
	PermitIntervalSec   int    `mapstructure:"permit_interval_sec,omitempty" validate:"required,gtefield=CacheTTLSec,gtefield=NegativeCacheTTLSec"` // アクセス許可判定周期
	PermitUnitNumber    int64  `mapstructure:"permit_unit_number,omitempty" validate:"required"`                                                    // アクセス許可する単位(PermitIntervalSecあたりPermitUnitNumber許可)
	CacheTTLSec         int    `mapstructure:"cache_ttl_sec,omitempty" validate:"required"`                                                         // ローカルメモリキャッシュTTL
	NegativeCacheTTLSec int    `mapstructure:"negative_cache_ttl_sec,omitempty" validate:"required"`                                                // ローカルメモリネガティブキャッシュTTL
	PublicHost          string `mapstructure:"public_host,omitempty"`                                                                               // 公開URLのホスト
	SlackApiToken       string `mapstructure:"slack_api_token,omitempty"`                                                                           // Slack Api Token
	SlackChannel        string `mapstructure:"slack_channel,omitempty"`                                                                             // Slack Channel
}
