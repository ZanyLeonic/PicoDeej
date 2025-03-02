package deej

import (
	"time"

	"github.com/micmonay/keybd_event"
	"go.uber.org/zap"
)

func (sio *SerialIO) HandleMediaKeys(logger *zap.SugaredLogger, action string) {
	switch action {
	case "next":
		logger.Debug("Next track")
		sio.kbBonding.SetKeys(keybd_event.VK_NEXTSONG)
	case "play_pause":
		logger.Debug("Play/Pause")
		sio.kbBonding.SetKeys(keybd_event.VK_PLAYPAUSE)
	case "back":
		logger.Debug("Prev. track")
		sio.kbBonding.SetKeys(keybd_event.VK_PREVIOUSSONG)
	default:
		logger.Debug("Unimplemented keypress ", action)
	}

	sio.kbBonding.Press()
	time.Sleep(1 * time.Millisecond)
	sio.kbBonding.Release()
}