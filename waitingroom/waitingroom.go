package waitingroom

type WaitingInfo struct {
	SerialNumber   int64  // 通し番号
	EntryTimestamp int64  // キューに追加された時間
	ID             string // ユーザー固有ID
}
