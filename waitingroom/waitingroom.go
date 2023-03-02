package waitingroom

type WaitingInfo struct {
	SerialNumber int64  // 通し番号
	ID           string // ユーザー固有ID
}

func (w *WaitingInfo) delayKey() string {
	return w.ID + "_delay"
}
