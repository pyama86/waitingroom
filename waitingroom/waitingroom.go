package waitingroom

import "github.com/google/uuid"

type WaitingInfo struct {
	SerialNumber   int64  // 通し番号
	EntryTimestamp int64  // キューに追加された時間
	ID             string // ユーザー固有ID
}

func (w *WaitingInfo) setID() error {
	if w.ID == "" {
		u, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		w.ID = u.String()
	}
	return nil
}
