package waitingroom

type Config struct {
	LogLevel string
	Listener string

	ClientPollingIntervalSec int   // Clientがポーリングしてくる必要がある間隔
	AllowedAccessSec         int   // アクセス許可後アクセスできる時間
	EntryDelaySec            int64 // 初回エントリーをDelayさせる秒数
	QueueEnableSec           int   // 待合室を有効にしておく時間
	CacheTTLSec              int   // ローカルメモリキャッシュTTL
}
