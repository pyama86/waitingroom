package waitingroom

type WaitingInfo struct {
	SerialNumber         int64  // 通し番号
	ID                   string // ユーザー固有ID
	TakeSerialNumberTime int64  // シリアルナンバーを取得するUNIXTIME
}
