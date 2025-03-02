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
		sio.kbBonding.SetKeys(keybd_event.VK_MEDIA_NEXT_TRACK)
	case "play_pause":
		logger.Debug("Play/Pause")
		sio.kbBonding.SetKeys(keybd_event.VK_MEDIA_PLAY_PAUSE)
	case "back":
		logger.Debug("Prev. track")
		sio.kbBonding.SetKeys(keybd_event.VK_MEDIA_PREV_TRACK)
	default:
		logger.Debug("Unimplemented keypress ", action)
	}

	sio.kbBonding.Press()
	time.Sleep(1 * time.Millisecond)
	sio.kbBonding.Release()
}